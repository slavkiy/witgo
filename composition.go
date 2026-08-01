package witgo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPluginNotRegistered      = errors.New("plugin provider is not registered")
	ErrPluginAlreadyRegistered  = errors.New("plugin provider is already registered")
	ErrPluginDependencyMismatch = errors.New("plugin dependency contract mismatch")
	ErrPluginCallCycle          = errors.New("plugin call cycle detected")
	ErrPluginCallDepthExceeded  = errors.New("plugin call depth exceeded")
	ErrPluginProviderClosed     = errors.New("plugin provider is closed")
	ErrCrossRuntimeHandle       = errors.New("handle cannot cross plugin runtime boundary")
)

// InterfaceDescriptor is the stable identity and structural contract of one
// generated WIT interface.
type InterfaceDescriptor struct {
	ID        string
	Functions map[string]string
}

// DescriptorUsesRuntimeHandles reports whether an interface contains values
// whose ownership is tied to one Component Model Store.
func DescriptorUsesRuntimeHandles(descriptor InterfaceDescriptor) bool {
	for _, signature := range descriptor.Functions {
		if strings.Contains(signature, "future<") || strings.Contains(signature, "stream<") ||
			strings.Contains(signature, "error-context") || strings.Contains(signature, "(own") ||
			strings.Contains(signature, ",own") || strings.Contains(signature, ":own") ||
			strings.Contains(signature, "(borrow") || strings.Contains(signature, ",borrow") ||
			strings.Contains(signature, ":borrow") {
			return true
		}
	}
	return false
}

// HostOptions controls transparent calls between registered providers.
type HostOptions struct {
	MaxCallDepth      int
	RejectCycles      bool
	CallTimeout       time.Duration
	Observer          CallObserver
	FuelRequestPolicy FuelRequestPolicy
	FuelRequestLimits FuelRequestLimits
	SecurityObserver  RuntimeSecurityObserver
}

// PluginCallFrame identifies one routed provider call.
type PluginCallFrame struct {
	Plugin    string
	Interface string
	Function  string
}

// PluginCallContext describes the current nested call chain.
type PluginCallContext struct {
	ID       string
	ParentID string
	Depth    int
	Path     []PluginCallFrame
	Deadline time.Time
}

// PluginCallEvent is delivered to an observer outside registry locks.
type PluginCallEvent struct {
	CallID       string
	ParentCallID string
	Depth        int
	Consumer     string
	Provider     string
	Interface    string
	Function     string
	Path         []PluginCallFrame
	Started      time.Time
	Duration     time.Duration
	Err          error
}

type CallObserver interface {
	OnCallStart(PluginCallEvent)
	OnCallFinish(PluginCallEvent)
}

// PluginDependencyError reports a structural mismatch before a consumer is
// instantiated. It supports errors.Is(err, ErrPluginDependencyMismatch).
type PluginDependencyError struct {
	Consumer  string
	Provider  string
	Interface string
	Report    ValidationReport
	CallPath  []PluginCallFrame
}

func (e *PluginDependencyError) Error() string {
	if e == nil {
		return ErrPluginDependencyMismatch.Error()
	}
	return fmt.Sprintf("%s: consumer=%q provider=%q interface=%q: %s", ErrPluginDependencyMismatch, e.Consumer, e.Provider, e.Interface, e.Report.Summary())
}

func (e *PluginDependencyError) Unwrap() error { return ErrPluginDependencyMismatch }

// PluginCallError preserves the routed call path and underlying runtime error.
type PluginCallError struct {
	Consumer string
	Provider string
	Frame    PluginCallFrame
	Path     []PluginCallFrame
	Cause    error
}

func (e *PluginCallError) Error() string {
	if e == nil {
		return "plugin call failed"
	}
	frames := make([]string, 0, len(e.Path))
	for _, frame := range e.Path {
		frames = append(frames, frame.Plugin+" "+frame.Interface+"#"+frame.Function)
	}
	if len(frames) == 0 {
		frames = append(frames, e.Frame.Plugin+" "+e.Frame.Interface+"#"+e.Frame.Function)
	}
	return "plugin call failed: " + strings.Join(frames, " -> ") + ": " + e.Cause.Error()
}

func (e *PluginCallError) Unwrap() error { return e.Cause }

// ProviderCall is the typed-neutral core callback. Generated registration
// helpers create it from a strongly typed generated interface.
type ProviderCall func(context.Context, string, []any) (any, error)

type RegisterOptions struct {
	Owned       bool
	Close       func() error
	Composition ComponentComposition
}

type RegisterOption func(*RegisterOptions)

func OwnedProvider(close func() error) RegisterOption {
	return func(options *RegisterOptions) { options.Owned, options.Close = true, close }
}

func ExternallyOwnedProvider() RegisterOption {
	return func(options *RegisterOptions) { options.Owned = false }
}

// ComponentProvider marks a provider as an export of a Component Model graph.
// Generated consumers use this metadata to compose the provider into their own
// Store instead of proxying Store-owned handles through Go.
func ComponentProvider(composition ComponentComposition) RegisterOption {
	return func(options *RegisterOptions) {
		options.Composition = ComponentComposition{
			Component:    composition.Component,
			Dependencies: cloneCompositionPlugs(composition.Dependencies),
		}
	}
}

// Host owns the provider registry and call policy. It never holds its registry
// mutex while user code or a component runtime is executing.
type Host struct {
	options HostOptions
	mu      sync.RWMutex
	byID    map[string]map[string]*ProviderHandle
	closed  bool
	nextID  uint64
}

func NewHost(options ...HostOptions) (*Host, error) {
	if err := RequireHostBuild(); err != nil {
		return nil, err
	}
	if len(options) > 1 {
		return nil, errors.New("at most one HostOptions value is allowed")
	}
	configured := HostOptions{MaxCallDepth: 32, RejectCycles: true}
	if len(options) == 1 {
		configured = options[0]
		if configured.MaxCallDepth == 0 {
			configured.MaxCallDepth = 32
		}
	}
	if configured.MaxCallDepth < 0 || configured.CallTimeout < 0 {
		return nil, errors.New("HostOptions limits cannot be negative")
	}
	return &Host{options: configured, byID: make(map[string]map[string]*ProviderHandle)}, nil
}

// ProviderHandle is a stable, concurrency-safe reference. Unregister prevents
// new leases and waits for calls that already acquired one.
type ProviderHandle struct {
	host        *Host
	name        string
	descriptor  InterfaceDescriptor
	call        ProviderCall
	owned       bool
	close       func() error
	composition ComponentComposition

	mu       sync.Mutex
	cond     *sync.Cond
	active   int
	closed   bool
	draining bool
}

func (p *ProviderHandle) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}
func (p *ProviderHandle) Descriptor() InterfaceDescriptor {
	if p == nil {
		return InterfaceDescriptor{}
	}
	return cloneDescriptor(p.descriptor)
}

// CompositionPlug returns the exact edge needed to instantiate this provider
// in the consumer's Store. The full interface ID prevents short-name clashes.
func (p *ProviderHandle) CompositionPlug() (CompositionPlug, bool) {
	if p == nil || p.composition.Component == "" || p.isClosed() {
		return CompositionPlug{}, false
	}
	return CompositionPlug{
		Interface:    p.descriptor.ID,
		Component:    p.composition.Component,
		Dependencies: cloneCompositionPlugs(p.composition.Dependencies),
	}, true
}

func (h *Host) RegisterProvider(name string, descriptor InterfaceDescriptor, call ProviderCall, options ...RegisterOption) (*ProviderHandle, error) {
	if strings.TrimSpace(name) == "" || descriptor.ID == "" || call == nil {
		return nil, errors.New("provider name, interface descriptor, and callback are required")
	}
	if err := validateDescriptor(descriptor); err != nil {
		return nil, err
	}
	configured := RegisterOptions{}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	handle := &ProviderHandle{host: h, name: name, descriptor: cloneDescriptor(descriptor), call: call, owned: configured.Owned, close: configured.Close, composition: configured.Composition}
	handle.cond = sync.NewCond(&handle.mu)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrPluginProviderClosed
	}
	providers := h.byID[descriptor.ID]
	if providers == nil {
		providers = make(map[string]*ProviderHandle)
		h.byID[descriptor.ID] = providers
	}
	if _, exists := providers[name]; exists {
		return nil, fmt.Errorf("%w: %s/%s", ErrPluginAlreadyRegistered, descriptor.ID, name)
	}
	providers[name] = handle
	return handle, nil
}

func (h *Host) ResolveProvider(name string, expected InterfaceDescriptor) (*ProviderHandle, error) {
	h.mu.RLock()
	provider := h.byID[expected.ID][name]
	h.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("%w: %s/%s", ErrPluginNotRegistered, expected.ID, name)
	}
	if report, err := compareDescriptors(expected, provider.descriptor); err != nil {
		return nil, &PluginDependencyError{Provider: name, Interface: expected.ID, Report: report}
	}
	if provider.isClosed() {
		return nil, ErrPluginProviderClosed
	}
	return provider, nil
}

func (h *Host) AutoResolveProvider(expected InterfaceDescriptor) (*ProviderHandle, error) {
	h.mu.RLock()
	providers := h.byID[expected.ID]
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	h.mu.RUnlock()
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: no provider registered for %s", ErrPluginNotRegistered, expected.ID)
	}
	if len(names) > 1 {
		return nil, fmt.Errorf("%w: multiple providers registered for %s: %s; select one explicitly", ErrPluginAlreadyRegistered, expected.ID, strings.Join(names, ", "))
	}
	return h.ResolveProvider(names[0], expected)
}

func (h *Host) UnregisterProvider(interfaceID, name string) error {
	h.mu.Lock()
	provider := h.byID[interfaceID][name]
	if provider != nil {
		delete(h.byID[interfaceID], name)
		if len(h.byID[interfaceID]) == 0 {
			delete(h.byID, interfaceID)
		}
	}
	h.mu.Unlock()
	if provider == nil {
		return fmt.Errorf("%w: %s/%s", ErrPluginNotRegistered, interfaceID, name)
	}
	return provider.drainAndClose()
}

func (h *Host) ReplaceProvider(name string, descriptor InterfaceDescriptor, call ProviderCall, options ...RegisterOption) (*ProviderHandle, error) {
	if strings.TrimSpace(name) == "" || call == nil {
		return nil, errors.New("provider name and callback are required")
	}
	if err := validateDescriptor(descriptor); err != nil {
		return nil, err
	}
	configured := RegisterOptions{}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	next := &ProviderHandle{host: h, name: name, descriptor: cloneDescriptor(descriptor), call: call, owned: configured.Owned, close: configured.Close, composition: configured.Composition}
	next.cond = sync.NewCond(&next.mu)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, ErrPluginProviderClosed
	}
	providers := h.byID[descriptor.ID]
	if providers == nil || providers[name] == nil {
		h.mu.Unlock()
		return nil, fmt.Errorf("%w: %s/%s", ErrPluginNotRegistered, descriptor.ID, name)
	}
	previous := providers[name]
	if report, err := compareDescriptors(previous.descriptor, descriptor); err != nil {
		h.mu.Unlock()
		return nil, &PluginDependencyError{Provider: name, Interface: descriptor.ID, Report: report}
	}
	providers[name] = next
	h.mu.Unlock()
	return next, previous.drainAndClose()
}

func (h *Host) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	var providers []*ProviderHandle
	for _, byName := range h.byID {
		for _, provider := range byName {
			providers = append(providers, provider)
		}
	}
	h.byID = make(map[string]map[string]*ProviderHandle)
	h.mu.Unlock()
	var first error
	for _, provider := range providers {
		if err := provider.drainAndClose(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (p *ProviderHandle) CallContext(ctx context.Context, consumer, function string, args ...any) (any, error) {
	if p == nil {
		return nil, ErrPluginNotRegistered
	}
	if containsRuntimeHandle(args) {
		return nil, ErrCrossRuntimeHandle
	}
	if err := p.acquire(); err != nil {
		return nil, err
	}
	defer p.release()
	ctx, cancel, event, err := p.host.beginCall(ctx, consumer, p.name, p.descriptor.ID, function)
	if err != nil {
		return nil, err
	}
	defer cancel()
	p.host.observeStart(event)
	started := event.Started
	result, callErr := p.call(ctx, function, args)
	if callErr == nil && containsRuntimeHandle(result) {
		callErr = ErrCrossRuntimeHandle
	}
	event.Duration, event.Err = time.Since(started), callErr
	p.host.observeFinish(event)
	if callErr != nil {
		return nil, &PluginCallError{Consumer: consumer, Provider: p.name, Frame: event.Path[len(event.Path)-1], Path: event.Path, Cause: callErr}
	}
	return result, nil
}

func (p *ProviderHandle) acquire() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.draining {
		return ErrPluginProviderClosed
	}
	p.active++
	return nil
}
func (p *ProviderHandle) release() {
	p.mu.Lock()
	p.active--
	if p.active == 0 {
		p.cond.Broadcast()
	}
	p.mu.Unlock()
}
func (p *ProviderHandle) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed || p.draining
}
func (p *ProviderHandle) drainAndClose() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.draining = true
	for p.active > 0 {
		p.cond.Wait()
	}
	p.closed = true
	p.mu.Unlock()
	if p.owned && p.close != nil {
		return p.close()
	}
	return nil
}

type pluginCallContextKey struct{}
type pluginRuntimeStackKey struct{}

func PluginCallContextFromContext(ctx context.Context) (PluginCallContext, bool) {
	state, ok := ctx.Value(pluginCallContextKey{}).(PluginCallContext)
	state.Path = append([]PluginCallFrame(nil), state.Path...)
	return state, ok
}

func (h *Host) beginCall(ctx context.Context, consumer, provider, interfaceID, function string) (context.Context, context.CancelFunc, PluginCallEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parent, _ := PluginCallContextFromContext(ctx)
	frame := PluginCallFrame{Plugin: provider, Interface: interfaceID, Function: function}
	if h.options.RejectCycles {
		for _, active := range parent.Path {
			if active.Plugin == provider && active.Interface == interfaceID {
				return ctx, func() {}, PluginCallEvent{}, fmt.Errorf("%w: %s/%s", ErrPluginCallCycle, interfaceID, provider)
			}
		}
	}
	depth := len(parent.Path) + 1
	if h.options.MaxCallDepth > 0 && depth > h.options.MaxCallDepth {
		return ctx, func() {}, PluginCallEvent{}, ErrPluginCallDepthExceeded
	}
	callID := fmt.Sprintf("witgo-%d", atomic.AddUint64(&h.nextID, 1))
	state := PluginCallContext{ID: callID, ParentID: parent.ID, Depth: depth, Path: append(parent.Path, frame)}
	var cancel context.CancelFunc = func() {}
	if h.options.CallTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, h.options.CallTimeout)
	}
	if deadline, ok := ctx.Deadline(); ok {
		state.Deadline = deadline
	}
	ctx = context.WithValue(ctx, pluginCallContextKey{}, state)
	event := PluginCallEvent{CallID: callID, ParentCallID: parent.ID, Depth: depth, Consumer: consumer, Provider: provider, Interface: interfaceID, Function: function, Path: append([]PluginCallFrame(nil), state.Path...), Started: time.Now()}
	return ctx, cancel, event, nil
}

func (h *Host) enterRuntimeCall(ctx context.Context, runtimeID, function string) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	stack, _ := ctx.Value(pluginRuntimeStackKey{}).([]string)
	if h.options.RejectCycles {
		for _, active := range stack {
			if active == runtimeID {
				return ctx, func() {}, fmt.Errorf("%w: runtime=%q function=%q", ErrPluginCallCycle, runtimeID, function)
			}
		}
	}
	if h.options.MaxCallDepth > 0 && len(stack)+1 > h.options.MaxCallDepth {
		return ctx, func() {}, ErrPluginCallDepthExceeded
	}
	copyStack := append(append([]string(nil), stack...), runtimeID)
	var cancel context.CancelFunc = func() {}
	if h.options.CallTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, h.options.CallTimeout)
	}
	return context.WithValue(ctx, pluginRuntimeStackKey{}, copyStack), cancel, nil
}

func (h *Host) observeStart(event PluginCallEvent) {
	if h.options.Observer != nil {
		func() { defer func() { _ = recover() }(); h.options.Observer.OnCallStart(event) }()
	}
}
func (h *Host) observeFinish(event PluginCallEvent) {
	if h.options.Observer != nil {
		func() { defer func() { _ = recover() }(); h.options.Observer.OnCallFinish(event) }()
	}
}

func validateDescriptor(descriptor InterfaceDescriptor) error {
	if descriptor.ID == "" {
		return errors.New("interface descriptor ID is empty")
	}
	for function, signature := range descriptor.Functions {
		if function == "" || signature == "" {
			return fmt.Errorf("interface descriptor %q contains an empty function or signature", descriptor.ID)
		}
	}
	return nil
}

func cloneDescriptor(descriptor InterfaceDescriptor) InterfaceDescriptor {
	result := InterfaceDescriptor{ID: descriptor.ID, Functions: make(map[string]string, len(descriptor.Functions))}
	for name, signature := range descriptor.Functions {
		result.Functions[name] = signature
	}
	return result
}

func descriptorContract(descriptor InterfaceDescriptor) Contract {
	exports := make([]string, 0, len(descriptor.Functions))
	signatures := make(map[string]string, len(descriptor.Functions))
	for name, signature := range descriptor.Functions {
		full := descriptor.ID + "#" + name
		exports = append(exports, full)
		signatures[full] = signature
	}
	return Contract{Exports: exports, Signatures: signatures}
}

func compareDescriptors(expected, actual InterfaceDescriptor) (ValidationReport, error) {
	report, err := CompareContracts(descriptorContract(expected), descriptorContract(actual))
	if err != nil {
		return report, err
	}
	return report, report.Err()
}
