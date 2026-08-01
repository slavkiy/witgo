package witgo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	ipath "github.com/slavkiy/witgo/internal/path"
	iwasm "github.com/slavkiy/witgo/internal/wasm"
)

var (
	ErrUnknownWasmKind        = errors.New("unknown WebAssembly kind")
	ErrCoreModule             = errors.New("core WebAssembly modules are not supported; build a Component Model component")
	ErrFuelDisabled           = errors.New("WebAssembly fuel metering is disabled")
	ErrRuntimeClosed          = errors.New("component runtime is closed")
	ErrBridgeProtocolMismatch = errors.New("component bridge protocol mismatch")
	ErrBridgeVersionMismatch  = errors.New("component bridge version mismatch")
	ErrContractMismatch       = errors.New("component function contract mismatch")
	ErrCapabilityDenied       = errors.New("component capability policy denied required host import")
	ErrHandleClosed           = errors.New("component handle is closed or unknown")
	ErrResultTooLarge         = errors.New("WebAssembly result exceeds configured limit")
	ErrNestedPluginNotFound   = errors.New("nested plugin provider was not found")
	ErrNestedPluginAmbiguous  = errors.New("multiple nested plugins provide the same import")
	ErrNestedPluginCycle      = errors.New("nested plugin dependency cycle")
	ErrNestedPluginPathDenied = errors.New("nested plugin dependency path is outside the host policy")
	ErrNestedPluginBudget     = errors.New("plugin box resource budget is too small")
)

// RuntimeOptions controls resource limits for a Component Model runtime.
type RuntimeOptions struct {
	Fuel             uint64
	FuelPerCall      uint64
	Timeout          time.Duration
	MemoryLimitBytes int64
	MaxResultBytes   uint64
	InstanceLimit    int64
	// EnableRuntimeAPI binds the versioned witgo:runtime/runtime@1.0.0 vendor
	// capability. Read-only queries are safe; additional fuel remains default-deny.
	EnableRuntimeAPI bool
	// PluginID is the sanitized logical identity reported by RuntimeSystem.
	// It must not contain a path or secret. The zero value is "plugin".
	PluginID string
	// Fuel request settings are used when no CompositionHost is attached.
	FuelRequestPolicy FuelRequestPolicy
	FuelRequestLimits FuelRequestLimits
	SecurityObserver  RuntimeSecurityObserver
	ValueLimits       ValueLimits
	// BridgePath overrides the bundled library and WITGO_COMPONENT_LIBRARY.
	BridgePath string
	// BridgeSHA256 verifies BridgePath before loading. It is required when a
	// custom bridge is supplied in security-sensitive deployments.
	BridgeSHA256 string
	// DisableEmbeddedBridge prevents extraction or loading of the library
	// shipped with witgo. Set BridgePath to use an administrator-managed library.
	DisableEmbeddedBridge bool
	// Capabilities restricts which host import functions a component may require.
	// Zero value allows everything.
	Capabilities CapabilityPolicy
	// NestedPlugins controls automatic resolution of missing imports through
	// exports of sibling Component Model plugins. The zero value enables it.
	NestedPlugins NestedPluginOptions
	// PluginHost centralizes nested-plugin discovery policy. Every top-level
	// load still creates an independent box of runtime instances.
	PluginHost *PluginHost
	// CompositionHost enables transparent registered-provider routing and call
	// chain protection for this runtime.
	CompositionHost *Host
	// CompositionPlugs are exact provider edges encoded into one Component
	// Model component and instantiated in the same Store. Generated bindings
	// populate this field; ordinary applications should use their typed helpers.
	CompositionPlugs []CompositionPlug
}

// HostFunc implements one imported WIT function. Arguments and the result use
// ordinary Go values with the same shape as their WIT types.
type HostFunc func(args []any) (any, error)

// HostFuncContext implements an imported WIT function with cancellation and
// deadline propagation. New code should prefer it over HostFunc.
type HostFuncContext func(ctx context.Context, args []any) (any, error)

// HostImport grants a component one host capability.
type HostImport struct {
	Interface   string
	Function    string
	Call        HostFunc
	CallContext HostFuncContext
}

// Contract describes the functions expected by generated bindings. Names are
// either "interface#function" or a direct world function name. Signatures use
// a deterministic structural representation of Component Model value types.
type Contract struct {
	Imports    []string
	Exports    []string
	Signatures map[string]string
}

type Runtime struct {
	Kind             iwasm.Kind
	bridge           *componentBridge
	maxResultBytes   uint64
	temporaryFile    string
	nested           []*Runtime
	nestedPaths      []string
	componentFile    string
	pluginHost       *PluginHost
	effectiveOptions RuntimeOptions
	compositionHost  *Host
}

func LoadRuntime(filename string) (*Runtime, error) {
	return LoadRuntimeContext(context.Background(), filename)
}

func LoadRuntimeContext(ctx context.Context, filename string) (*Runtime, error) {
	return LoadRuntimeWithOptionsContext(ctx, filename, RuntimeOptions{})
}

func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeWithOptionsContext(context.Background(), filename, options)
}

func LoadRuntimeWithOptionsContext(ctx context.Context, filename string, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeWithImportsContext(ctx, filename, options, nil)
}

// LoadRuntimeWithImports loads a standard WebAssembly component and exposes
// only the explicitly listed host functions to it.
func LoadRuntimeWithImports(filename string, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return LoadRuntimeWithImportsContext(context.Background(), filename, options, imports)
}

func LoadRuntimeWithImportsContext(ctx context.Context, filename string, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return loadRuntime(ctx, filename, options, imports, nil)
}

// LoadRuntimeWithContract loads a component and rejects it when its imported or
// exported function names differ from the contract embedded in generated code.
func LoadRuntimeWithContract(filename string, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return LoadRuntimeWithContractContext(context.Background(), filename, options, imports, contract)
}

func LoadRuntimeWithContractContext(ctx context.Context, filename string, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return loadRuntime(ctx, filename, options, imports, &contract)
}

func loadRuntime(ctx context.Context, filename string, options RuntimeOptions, imports []HostImport, contract *Contract) (*Runtime, error) {
	if err := RequireHostBuild(); err != nil {
		return nil, err
	}
	host := options.PluginHost
	if host == nil {
		host = newPluginHost(options.NestedPlugins)
	} else {
		options.NestedPlugins = host.options
	}
	runtime, err := loadRuntimeNested(ctx, filename, options, imports, contract, newNestedLoadState(host))
	if err != nil {
		return nil, err
	}
	if err := host.register(runtime); err != nil {
		_ = runtime.Close()
		return nil, err
	}
	runtime.pluginHost = host
	return runtime, nil
}

func loadRuntimeNested(ctx context.Context, filename string, options RuntimeOptions, imports []HostImport, contract *Contract, state *nestedLoadState) (*Runtime, error) {
	if state != nil && state.host != nil {
		options.PluginHost = state.host
		options.NestedPlugins = state.host.options
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateRuntimeOptions(options); err != nil {
		return nil, err
	}
	path, err := componentPath(filename)
	if err != nil {
		return nil, err
	}
	if err := state.enter(path); err != nil {
		return nil, err
	}
	defer state.leave(path)
	options.CompositionPlugs, err = normalizeComposition(path, options.CompositionPlugs)
	if err != nil {
		return nil, err
	}
	resolvedImports, children, childPaths, effectiveOptions, err := resolveNestedImports(ctx, path, options, imports, state)
	if err != nil {
		closeRuntimes(children)
		return nil, err
	}
	bridge, err := startComponentBridge(ctx, path, effectiveOptions, resolvedImports, contract)
	if err != nil {
		closeRuntimes(children)
		return nil, err
	}
	return &Runtime{Kind: iwasm.KindComponent, bridge: bridge, maxResultBytes: effectiveOptions.MaxResultBytes, nested: children, nestedPaths: childPaths, componentFile: path, effectiveOptions: effectiveOptions, compositionHost: effectiveOptions.CompositionHost}, nil
}

func componentPath(filename string) (string, error) {
	path, err := ipath.NormalizePath(filename)
	if err != nil {
		return "", fmt.Errorf("normalize component path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read component %q: %w", path, err)
	}
	kind := iwasm.KindUnknown
	if iwasm.IsWasm(data) {
		kind = iwasm.DetectKind(data)
	} else if bytes.HasPrefix(bytes.TrimSpace(data), []byte("(component")) {
		kind = iwasm.KindComponent
	} else {
		return "", errors.New("data is not a valid WebAssembly component")
	}
	if kind == iwasm.KindCoreModule {
		return "", ErrCoreModule
	}
	if kind != iwasm.KindComponent {
		return "", fmt.Errorf("%w: %v", ErrUnknownWasmKind, kind)
	}
	return path, nil
}

func LoadRuntimeFromBytes(data []byte) (*Runtime, error) {
	return LoadRuntimeFromBytesContext(context.Background(), data)
}

func LoadRuntimeFromBytesContext(ctx context.Context, data []byte) (*Runtime, error) {
	return LoadRuntimeFromBytesWithOptionsContext(ctx, data, RuntimeOptions{})
}

func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeFromBytesWithOptionsContext(context.Background(), data, options)
}

func LoadRuntimeFromBytesWithOptionsContext(ctx context.Context, data []byte, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeFromBytesWithImportsContext(ctx, data, options, nil)
}

func LoadRuntimeFromBytesWithImports(data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return LoadRuntimeFromBytesWithImportsContext(context.Background(), data, options, imports)
}

func LoadRuntimeFromBytesWithImportsContext(ctx context.Context, data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return loadRuntimeFromBytes(ctx, data, options, imports, nil)
}

// LoadRuntimeFromBytesWithContract loads an in-memory component and verifies
// its manifest before instantiation.
func LoadRuntimeFromBytesWithContract(data []byte, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return LoadRuntimeFromBytesWithContractContext(context.Background(), data, options, imports, contract)
}

func LoadRuntimeFromBytesWithContractContext(ctx context.Context, data []byte, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return loadRuntimeFromBytes(ctx, data, options, imports, &contract)
}

func loadRuntimeFromBytes(ctx context.Context, data []byte, options RuntimeOptions, imports []HostImport, contract *Contract) (*Runtime, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	name, err := writeTemporaryComponent(data)
	if err != nil {
		return nil, err
	}
	var runtime *Runtime
	if contract == nil {
		runtime, err = LoadRuntimeWithImportsContext(ctx, name, options, imports)
	} else {
		runtime, err = loadRuntime(ctx, name, options, imports, contract)
	}
	if err != nil {
		_ = os.Remove(name)
		return nil, err
	}
	runtime.temporaryFile = name
	return runtime, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func writeTemporaryComponent(data []byte) (string, error) {
	if !iwasm.IsWasm(data) && !bytes.HasPrefix(bytes.TrimSpace(data), []byte("(component")) {
		return "", errors.New("data is not a valid WebAssembly component")
	}
	file, err := os.CreateTemp("", "witgo-component-*.wasm")
	if err != nil {
		return "", fmt.Errorf("create temporary component: %w", err)
	}
	name := file.Name()
	if _, err = file.Write(data); err == nil {
		err = file.Close()
	} else {
		_ = file.Close()
	}
	if err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("write temporary component: %w", err)
	}
	return name, nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var closeErr error
	if r.bridge != nil {
		closeErr = r.bridge.close()
	}
	for index := len(r.nested) - 1; index >= 0; index-- {
		if err := r.nested[index].Close(); closeErr == nil && err != nil {
			closeErr = err
		}
	}
	r.nested = nil
	r.nestedPaths = nil
	if r.pluginHost != nil {
		r.pluginHost.unregister(r)
		r.pluginHost = nil
	}
	if r.temporaryFile != "" {
		if err := os.Remove(r.temporaryFile); closeErr == nil && err != nil && !errors.Is(err, os.ErrNotExist) {
			closeErr = err
		}
		r.temporaryFile = ""
	}
	return closeErr
}

// ComponentPath returns the normalized component filename used by this runtime.
func (r *Runtime) ComponentPath() string {
	if r == nil {
		return ""
	}
	return r.componentFile
}

// EffectiveOptions returns the resource share assigned to this runtime inside
// its box. Router pointers are omitted from the returned copy.
func (r *Runtime) EffectiveOptions() RuntimeOptions {
	if r == nil {
		return RuntimeOptions{}
	}
	options := r.effectiveOptions
	options.PluginHost = nil
	options.CompositionHost = nil
	options.CompositionPlugs = cloneCompositionPlugs(options.CompositionPlugs)
	return options
}

// Composition returns the exact same-Store graph used to instantiate this
// runtime. It is used by generated export clients when they become providers.
func (r *Runtime) Composition() ComponentComposition {
	if r == nil {
		return ComponentComposition{}
	}
	return ComponentComposition{Component: r.componentFile, Dependencies: cloneCompositionPlugs(r.effectiveOptions.CompositionPlugs)}
}

func cloneCompositionPlugs(values []CompositionPlug) []CompositionPlug {
	result := make([]CompositionPlug, len(values))
	for index, value := range values {
		result[index] = CompositionPlug{Interface: value.Interface, Component: value.Component, Dependencies: cloneCompositionPlugs(value.Dependencies)}
	}
	return result
}

// NestedPluginPaths returns the direct child components selected automatically
// for this runtime. The returned slice is safe to modify.
func (r *Runtime) NestedPluginPaths() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.nestedPaths...)
}

// AllPluginPaths returns the root and all transitive nested components in this box.
func (r *Runtime) AllPluginPaths() []string {
	if r == nil {
		return nil
	}
	seen := make(map[string]bool)
	var paths []string
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	var visitPlugs func([]CompositionPlug)
	visitPlugs = func(plugs []CompositionPlug) {
		for _, plug := range plugs {
			add(plug.Component)
			visitPlugs(plug.Dependencies)
		}
	}
	var visit func(*Runtime)
	visit = func(current *Runtime) {
		if current == nil || seen[current.componentFile] {
			return
		}
		add(current.componentFile)
		visitPlugs(current.effectiveOptions.CompositionPlugs)
		for _, child := range current.nested {
			visit(child)
		}
	}
	visit(r)
	sort.Strings(paths)
	return paths
}

// IsClosed reports whether the runtime has been closed. A nil or uninitialized
// runtime is considered closed.
func (r *Runtime) IsClosed() bool {
	return r == nil || r.bridge == nil || r.bridge.isClosed()
}

func validateRuntimeOptions(options RuntimeOptions) error {
	if options.Fuel > 0 && options.FuelPerCall > 0 {
		return errors.New("RuntimeOptions.Fuel and FuelPerCall cannot be used together")
	}
	if options.Timeout < 0 {
		return errors.New("RuntimeOptions.Timeout cannot be negative")
	}
	if options.MemoryLimitBytes < 0 {
		return errors.New("RuntimeOptions.MemoryLimitBytes cannot be negative")
	}
	if options.InstanceLimit < 0 {
		return errors.New("RuntimeOptions.InstanceLimit cannot be negative")
	}
	if err := validatePluginID(options.PluginID); err != nil {
		return err
	}
	if options.FuelRequestLimits.MinRemainingTime < 0 || options.FuelRequestLimits.PolicyTimeout < 0 || options.FuelRequestLimits.MaxReasonBytes < 0 {
		return errors.New("RuntimeOptions fuel request limits cannot be negative")
	}
	if options.NestedPlugins.MaxCandidates < 0 {
		return errors.New("RuntimeOptions.NestedPlugins.MaxCandidates cannot be negative")
	}
	if options.BridgeSHA256 != "" {
		if len(options.BridgeSHA256) != 64 {
			return errors.New("RuntimeOptions.BridgeSHA256 must be a 64-character SHA-256 digest")
		}
		for _, char := range options.BridgeSHA256 {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return errors.New("RuntimeOptions.BridgeSHA256 must be hexadecimal")
			}
		}
	}
	return nil
}

func validatePluginID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if len(id) > 128 {
		return errors.New("RuntimeOptions.PluginID exceeds 128 bytes")
	}
	if strings.ContainsAny(id, `/\\`) {
		return errors.New("RuntimeOptions.PluginID must be a logical name, not a path")
	}
	for _, char := range id {
		if unicode.IsControl(char) {
			return errors.New("RuntimeOptions.PluginID cannot contain control characters")
		}
	}
	return nil
}
