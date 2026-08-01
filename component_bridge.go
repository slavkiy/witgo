package witgo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slavkiy/witgo/internal/bridgebin"
)

type componentBridge struct {
	native         *nativeBridge
	imports        map[string]HostFuncContext
	maxResultBytes uint64
	valueLimits    ValueLimits
	handleStates   map[uint64][]*handleState
	mu             sync.Mutex
	closed         bool
	closing        atomic.Bool
	system         runtimeSystemConfig
}

type bridgeMessage struct {
	Type            string            `json:"type"`
	Error           string            `json:"error,omitempty"`
	ProtocolVersion uint32            `json:"protocol_version,omitempty"`
	WitgoVersion    string            `json:"witgo_version,omitempty"`
	BridgeVersion   string            `json:"bridge_version,omitempty"`
	WasmtimeVersion string            `json:"wasmtime_version,omitempty"`
	Interface       string            `json:"interface,omitempty"`
	Function        string            `json:"function,omitempty"`
	Args            []any             `json:"args,omitempty"`
	Values          []any             `json:"values,omitempty"`
	Value           any               `json:"value,omitempty"`
	Features        []string          `json:"features,omitempty"`
	Imports         []string          `json:"imports,omitempty"`
	Exports         []string          `json:"exports,omitempty"`
	Signatures      map[string]string `json:"signatures,omitempty"`
	Consumed        []uint64          `json:"consumed,omitempty"`
	FuelRemaining   json.Number       `json:"fuel_remaining,omitempty"`
	FuelEnabled     bool              `json:"fuel_enabled,omitempty"`
}

type runtimeSystemConfig struct {
	enabled      bool
	pluginID     string
	maxCallDepth int
	memoryLimit  int64
	maxMessage   uint64
	policy       FuelRequestPolicy
	limits       FuelRequestLimits
	observer     RuntimeSecurityObserver
	initialFuel  uint64
	perCall      bool
}

var runtimeCallSequence atomic.Uint64

type bridgeImportSpec struct {
	Interface string   `json:"interface"`
	Functions []string `json:"functions"`
}

var runtimeSystemFunctions = []string{"call-info", "fuel-info", "is-cancelled", "limits", "request-additional-fuel"}

func runtimeSystemConfigFor(options RuntimeOptions) runtimeSystemConfig {
	maxMessage := options.MaxResultBytes
	if options.ValueLimits.MaxArgumentBytes > maxMessage {
		maxMessage = options.ValueLimits.MaxArgumentBytes
	}
	if options.ValueLimits.MaxResultBytes > maxMessage {
		maxMessage = options.ValueLimits.MaxResultBytes
	}
	configured := runtimeSystemConfig{
		enabled: options.EnableRuntimeAPI, pluginID: strings.TrimSpace(options.PluginID),
		maxCallDepth: 32, memoryLimit: options.MemoryLimitBytes, maxMessage: maxMessage,
		policy: options.FuelRequestPolicy, limits: options.FuelRequestLimits, observer: options.SecurityObserver,
		initialFuel: options.Fuel, perCall: options.FuelPerCall > 0,
	}
	if options.FuelPerCall > 0 {
		configured.initialFuel = options.FuelPerCall
	}
	if configured.pluginID == "" {
		configured.pluginID = "plugin"
	}
	if host := options.CompositionHost; host != nil {
		configured.maxCallDepth = host.options.MaxCallDepth
		configured.policy = host.options.FuelRequestPolicy
		configured.limits = host.options.FuelRequestLimits
		configured.observer = host.options.SecurityObserver
	}
	if configured.policy == nil {
		configured.policy = DenyFuelRequests{}
	}
	return configured
}

func prepareHostImports(imports []HostImport) (map[string]HostFuncContext, []string, []bridgeImportSpec, error) {
	registered := make(map[string]HostFuncContext, len(imports))
	registeredNames := make([]string, 0, len(imports))
	grouped := make(map[string][]string)
	for _, item := range imports {
		if item.Interface == "" || item.Function == "" || (item.Call == nil && item.CallContext == nil) {
			return nil, nil, nil, errors.New("host import interface, function, and Call or CallContext are required")
		}
		if strings.HasPrefix(item.Interface, "witgo:runtime/runtime@") {
			return nil, nil, nil, errors.New("witgo runtime system interfaces are reserved and cannot be supplied by application code")
		}
		key := item.Interface + "#" + item.Function
		if _, exists := registered[key]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate host import %q", key)
		}
		call := item.CallContext
		if call == nil {
			legacy := item.Call
			call = func(_ context.Context, args []any) (any, error) { return legacy(args) }
		}
		registered[key] = call
		registeredNames = append(registeredNames, key)
		grouped[item.Interface] = append(grouped[item.Interface], item.Function)
	}
	specs := make([]bridgeImportSpec, 0, len(grouped))
	for name, functions := range grouped {
		sort.Strings(functions)
		specs = append(specs, bridgeImportSpec{name, functions})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Interface < specs[j].Interface })
	return registered, registeredNames, specs, nil
}

func pingComponentBridge(ctx context.Context, component string, options RuntimeOptions, imports []HostImport) (*componentBridge, bridgeMessage, []string, error) {
	if err := contextError(ctx); err != nil {
		return nil, bridgeMessage{}, nil, err
	}
	registered, registeredNames, specs, err := prepareHostImports(imports)
	if err != nil {
		return nil, bridgeMessage{}, nil, err
	}
	path, err := resolveBridge(options)
	if err != nil {
		return nil, bridgeMessage{}, nil, err
	}
	native, err := openNativeBridge(path)
	if err != nil {
		return nil, bridgeMessage{}, nil, fmt.Errorf("load component bridge library %q: %w", path, err)
	}
	system := runtimeSystemConfigFor(options)
	if system.enabled {
		for key := range registered {
			if strings.HasPrefix(key, "witgo:runtime/runtime@") {
				_ = native.close()
				return nil, bridgeMessage{}, nil, errors.New("runtime system imports are reserved and cannot be overridden")
			}
		}
		functions := append([]string(nil), runtimeSystemFunctions...)
		specs = append(specs, bridgeImportSpec{Interface: RuntimeSystemInterfaceID, Functions: functions})
		for _, function := range functions {
			registeredNames = append(registeredNames, RuntimeSystemInterfaceID+"#"+function)
		}
		sort.Strings(registeredNames)
	}
	valueLimits := options.ValueLimits
	if valueLimits.MaxResultBytes == 0 {
		valueLimits.MaxResultBytes = options.MaxResultBytes
	}
	b := &componentBridge{native: native, imports: registered, maxResultBytes: valueLimits.MaxResultBytes, valueLimits: valueLimits, system: system}
	clientVersion := witgoVersion()
	init := map[string]any{
		"type": "init", "protocol_version": bridgeProtocolVersion,
		"witgo_version": clientVersion, "bridge_version": bridgeVersion,
		"required_features": append([]string(nil), bridgeRequiredFeatures...),
		"component":         component, "composition": cloneCompositionPlugs(options.CompositionPlugs), "imports": specs,
		"options": map[string]any{"fuel": options.Fuel, "fuel_per_call": options.FuelPerCall, "timeout_millis": options.Timeout.Milliseconds(), "memory_limit_bytes": options.MemoryLimitBytes, "instance_limit": options.InstanceLimit},
	}
	if err := b.write(init); err != nil {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, err
	}
	message, err := b.read()
	if err != nil {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, err
	}
	if err := contextError(ctx); err != nil {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, err
	}
	if message.Type == "error" || message.Type == "fatal" {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, messageError(message)
	}
	if err := validateBridgeHandshake(message, clientVersion); err != nil {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, err
	}
	if message.Type != "pong" {
		_ = b.abort()
		return nil, bridgeMessage{}, nil, fmt.Errorf("%w: expected contract pong, bridge returned %q", ErrBridgeProtocolMismatch, message.Type)
	}
	return b, message, registeredNames, nil
}

func startComponentBridge(ctx context.Context, component string, options RuntimeOptions, imports []HostImport, contract *Contract) (*componentBridge, error) {
	b, message, registeredNames, err := pingComponentBridge(ctx, component, options, imports)
	if err != nil {
		return nil, err
	}
	if err := options.Capabilities.ValidateImports(message.Imports); err != nil {
		_ = b.abort()
		return nil, err
	}
	if contract != nil {
		effectiveContract := compositionContract(*contract, options.CompositionPlugs)
		if options.EnableRuntimeAPI {
			expectsSystem := false
			for _, name := range effectiveContract.Imports {
				if strings.HasPrefix(name, RuntimeSystemInterfaceID+"#") {
					expectsSystem = true
					break
				}
			}
			if !expectsSystem {
				filtered := registeredNames[:0]
				for _, name := range registeredNames {
					if !strings.HasPrefix(name, RuntimeSystemInterfaceID+"#") {
						filtered = append(filtered, name)
					}
				}
				registeredNames = filtered
			}
		}
		if err := compareFunctions("registered host adapters", effectiveContract.Imports, registeredNames); err != nil {
			_ = b.abort()
			return nil, err
		}
		if err := validateContract(effectiveContract, message.Imports, message.Exports); err != nil {
			_ = b.abort()
			return nil, err
		}
		if err := compareSignatures(effectiveContract.Signatures, message.Signatures); err != nil {
			_ = b.abort()
			return nil, err
		}
	}
	if err := b.write(map[string]any{"type": "start"}); err != nil {
		_ = b.abort()
		return nil, err
	}
	message, err = b.read()
	if err != nil {
		_ = b.abort()
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		_ = b.abort()
		return nil, err
	}
	if message.Type != "ready" {
		_ = b.abort()
		return nil, messageError(message)
	}
	return b, nil
}

func inspectComponentBridge(ctx context.Context, component string, options RuntimeOptions) (Contract, error) {
	b, message, _, err := pingComponentBridge(ctx, component, options, nil)
	if err != nil {
		return Contract{}, err
	}
	if err := b.abort(); err != nil {
		return Contract{}, fmt.Errorf("close component inspection bridge: %w", err)
	}
	imports, _ := normalizedFunctions(message.Imports)
	exports, _ := normalizedFunctions(message.Exports)
	return Contract{Imports: imports, Exports: exports, Signatures: cloneSignatures(message.Signatures)}, nil
}

func validateBridgeHandshake(message bridgeMessage, clientVersion string) error {
	if message.ProtocolVersion != bridgeProtocolVersion {
		return fmt.Errorf("%w: Go package requires %d, bridge reports %d (bridge=%q, wasmtime=%q)", ErrBridgeProtocolMismatch,
			bridgeProtocolVersion, message.ProtocolVersion, message.BridgeVersion, message.WasmtimeVersion)
	}
	if message.BridgeVersion != bridgeVersion {
		return fmt.Errorf("%w: Go package requires %q, bridge reports %q (protocol=%d, wasmtime=%q)", ErrBridgeVersionMismatch,
			bridgeVersion, message.BridgeVersion, message.ProtocolVersion, message.WasmtimeVersion)
	}
	if message.WitgoVersion != clientVersion {
		return fmt.Errorf("component bridge handshake was initialized for witgo %q, expected %q", message.WitgoVersion, clientVersion)
	}
	for _, required := range bridgeRequiredFeatures {
		if !containsString(message.Features, required) {
			return fmt.Errorf("%w: bridge does not support required feature %q", ErrBridgeProtocolMismatch, required)
		}
	}
	return nil
}

func validateContract(expected Contract, actualImports, actualExports []string) error {
	if err := compareFunctions("host imports", expected.Imports, actualImports); err != nil {
		return err
	}
	return compareFunctions("plugin exports", expected.Exports, actualExports)
}

func compareSignatures(expected, actual map[string]string) error {
	for name, want := range expected {
		if got, ok := actual[name]; !ok || got != want {
			return fmt.Errorf("%w: function %q signature differs: expected=%q actual=%q", ErrContractMismatch, name, want, got)
		}
	}
	return nil
}

func cloneSignatures(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for name, signature := range source {
		result[name] = signature
	}
	return result
}

func compareFunctions(side string, expected, actual []string) error {
	want, wantDuplicates := normalizedFunctions(expected)
	got, gotDuplicates := normalizedFunctions(actual)
	if len(wantDuplicates) > 0 || len(gotDuplicates) > 0 {
		return fmt.Errorf("%w: %s contain duplicate names (expected=%v, actual=%v)", ErrContractMismatch, side, wantDuplicates, gotDuplicates)
	}
	missing, unexpected := difference(want, got), difference(got, want)
	if len(missing) != 0 || len(unexpected) != 0 {
		return fmt.Errorf("%w: %s differ: missing=%v unexpected=%v", ErrContractMismatch, side, missing, unexpected)
	}
	return nil
}

func normalizedFunctions(values []string) ([]string, []string) {
	copy := append([]string(nil), values...)
	sort.Strings(copy)
	unique := copy[:0]
	var duplicates []string
	for _, value := range copy {
		if len(unique) > 0 && unique[len(unique)-1] == value {
			if len(duplicates) == 0 || duplicates[len(duplicates)-1] != value {
				duplicates = append(duplicates, value)
			}
			continue
		}
		unique = append(unique, value)
	}
	return unique, duplicates
}

func difference(left, right []string) []string {
	set := make(map[string]struct{}, len(right))
	for _, value := range right {
		set[value] = struct{}{}
	}
	var result []string
	for _, value := range left {
		if _, ok := set[value]; !ok {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func resolveBridge(options RuntimeOptions) (string, error) {
	var diagnostics []string
	explicit := options.BridgePath
	if explicit == "" {
		explicit = os.Getenv("WITGO_COMPONENT_LIBRARY")
	}
	if explicit != "" {
		if err := verifyBridgeFile(explicit, options.BridgeSHA256); err != nil {
			return "", err
		}
		return explicit, nil
	}
	if options.BridgeSHA256 != "" {
		return "", errors.New("RuntimeOptions.BridgeSHA256 requires BridgePath or WITGO_COMPONENT_LIBRARY")
	}
	if !options.DisableEmbeddedBridge && os.Getenv("WITGO_DISABLE_EMBEDDED_BRIDGE") != "1" {
		if path, err := bridgebin.Library(); err == nil {
			return path, nil
		} else if !errors.Is(err, bridgebin.ErrUnavailable) {
			diagnostics = append(diagnostics, "embedded bridge: "+err.Error())
		}
	}
	name := "libwitgo_bridge.so"
	if runtime.GOOS == "windows" {
		name = "witgo_bridge.dll"
	}
	if runtime.GOOS == "darwin" {
		name = "libwitgo_bridge.dylib"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	candidates := []string{filepath.Join("bridge", "target", "release", name), filepath.Join("bridge", "target", "debug", name)}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	detail := ""
	if len(diagnostics) > 0 {
		detail = "; resolution errors: " + strings.Join(diagnostics, "; ")
	}
	return "", fmt.Errorf("component bridge library not found for %s/%s; set BridgePath or build ./bridge%s", runtime.GOOS, runtime.GOARCH, detail)
}

func verifyBridgeFile(path, expected string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect component bridge %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("component bridge %q is a directory", path)
	}
	if expected == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open component bridge %q: %w", path, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash component bridge %q: %w", path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close component bridge %q: %w", path, closeErr)
	}
	want, _ := hex.DecodeString(expected)
	if !bytesEqual(hash.Sum(nil), want) {
		return fmt.Errorf("component bridge %q failed SHA-256 verification: got %x, want %s", path, hash.Sum(nil), strings.ToLower(expected))
	}
	return nil
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for i := range left {
		different |= left[i] ^ right[i]
	}
	return different == 0
}

func (b *componentBridge) call(ctx context.Context, name string, args []any) ([]any, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	b.mu.Lock()
	var securityEvents []FuelRequestEvent
	defer func() {
		b.mu.Unlock()
		for _, event := range securityEvents {
			b.observeFuelRequest(event)
		}
	}()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if b.closed {
		return nil, ErrRuntimeClosed
	}
	if args == nil {
		args = []any{}
	}
	if err := validateArguments(args, b.valueLimits); err != nil {
		return nil, err
	}
	callState := b.newRuntimeFuelCallState(ctx, name)
	if err := b.write(map[string]any{"type": "call", "name": name, "args": args}); err != nil {
		return nil, err
	}
	for {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		message, err := b.read()
		if err != nil {
			return nil, err
		}
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		switch message.Type {
		case "host_call":
			if message.Interface == RuntimeSystemInterfaceID {
				response, event := b.handleRuntimeSystemCall(ctx, message, &callState)
				if event != nil {
					securityEvents = append(securityEvents, *event)
				}
				if err := b.write(response); err != nil {
					return nil, err
				}
				continue
			}
			fn := b.imports[message.Interface+"#"+message.Function]
			if fn == nil {
				_ = b.write(map[string]any{"type": "host_result", "error": "host import is not registered"})
				continue
			}
			bound, ok := b.bindHandles(message.Args).([]any)
			if !ok {
				_ = b.write(map[string]any{"type": "host_result", "error": "host arguments are not an array"})
				continue
			}
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			result, callErr := fn(ctx, bound)
			if callErr != nil {
				_ = b.write(map[string]any{"type": "host_result", "error": callErr.Error()})
				continue
			}
			values := []any{}
			if result != nil {
				values = append(values, result)
			}
			if err := b.write(map[string]any{"type": "host_result", "values": values}); err != nil {
				return nil, err
			}
		case "result":
			b.markHandlesConsumed(message.Consumed)
			bound, ok := b.bindHandles(message.Values).([]any)
			if !ok {
				return nil, errors.New("component bridge result values are not an array")
			}
			if err := validateResults(bound, b.valueLimits); err != nil {
				return nil, err
			}
			return bound, nil
		case "error", "fatal":
			return nil, messageError(message)
		default:
			return nil, fmt.Errorf("unexpected bridge message %q", message.Type)
		}
	}
}

func (b *componentBridge) newRuntimeFuelCallState(ctx context.Context, function string) runtimeFuelCallState {
	parent, ok := PluginCallContextFromContext(ctx)
	callID := parent.ID
	parentID := parent.ParentID
	depth := parent.Depth
	path := parent.Path
	if !ok || callID == "" {
		callID = fmt.Sprintf("witgo-runtime-%d", runtimeCallSequence.Add(1))
		parentID = ""
		depth = 1
		path = []PluginCallFrame{{Plugin: b.system.pluginID, Function: function}}
	}
	info := RuntimeCallInfo{CallID: callID, Depth: uint32(depth), PluginID: b.system.pluginID}
	if parentID != "" {
		info.ParentCallID = Some(parentID)
	} else {
		info.ParentCallID = None[string]()
	}
	if deadline, exists := ctx.Deadline(); exists {
		info.DeadlineUnixNanos = Some(deadline.UnixNano())
	}
	return runtimeFuelCallState{info: info, path: append([]PluginCallFrame(nil), path...), initial: b.system.initialFuel}
}

func (b *componentBridge) handleRuntimeSystemCall(ctx context.Context, message bridgeMessage, state *runtimeFuelCallState) (map[string]any, *FuelRequestEvent) {
	result := func(value any) map[string]any { return map[string]any{"type": "host_result", "values": []any{value}} }
	if !b.system.enabled {
		return map[string]any{"type": "host_result", "error": "runtime system capability is unavailable"}, nil
	}
	deadline := state.info.DeadlineUnixNanos
	currentFuel, fuelAvailable := uint64(0), false
	if message.FuelEnabled && message.FuelRemaining != "" {
		if parsed, err := parseUint(message.FuelRemaining.String()); err == nil {
			currentFuel, fuelAvailable = parsed, true
		}
	}
	switch message.Function {
	case "call-info":
		return result(state.info), nil
	case "fuel-info":
		info := RuntimeFuelInfo{Enabled: fuelAvailable, PerCall: b.system.perCall}
		if fuelAvailable {
			info.Remaining = Some(currentFuel)
			if state.initial <= ^uint64(0)-state.totalGranted {
				initial := state.initial + state.totalGranted
				info.Initial = Some(initial)
				if currentFuel <= initial {
					info.Consumed = Some(initial - currentFuel)
				}
			}
		}
		return result(info), nil
	case "limits":
		maxDepth := uint32(0)
		if b.system.maxCallDepth > 0 {
			if uint64(b.system.maxCallDepth) > uint64(^uint32(0)) {
				maxDepth = ^uint32(0)
			} else {
				maxDepth = uint32(b.system.maxCallDepth)
			}
		}
		remainingDepth := uint32(0)
		if maxDepth > state.info.Depth {
			remainingDepth = maxDepth - state.info.Depth
		}
		limits := RuntimeLimits{MaxCallDepth: maxDepth, RemainingCallDepth: remainingDepth, DeadlineUnixNanos: deadline}
		if b.system.memoryLimit > 0 {
			limits.MemoryLimitBytes = Some(uint64(b.system.memoryLimit))
		}
		if b.system.maxMessage > 0 {
			limits.MaxMessageBytes = Some(b.system.maxMessage)
		}
		return result(limits), nil
	case "is-cancelled":
		return result(contextError(ctx) != nil), nil
	case "request-additional-fuel":
		return b.handleFuelRequest(ctx, message, state, currentFuel, fuelAvailable)
	default:
		return map[string]any{"type": "host_result", "error": "unknown runtime system function"}, nil
	}
}

func (b *componentBridge) handleFuelRequest(ctx context.Context, message bridgeMessage, state *runtimeFuelCallState, current uint64, available bool) (map[string]any, *FuelRequestEvent) {
	amount, reason := uint64(0), ""
	if len(message.Args) == 2 {
		amount, _ = interfaceUint64(message.Args[0])
		reason, _ = message.Args[1].(string)
	}
	request := FuelRequest{CallID: state.info.CallID, PluginID: state.info.PluginID, Requested: amount, Reason: reason, CurrentFuel: current, InitialFuel: state.initial, TotalGranted: state.totalGranted, RequestCount: state.requestCount, CallDepth: int(state.info.Depth)}
	if parent, ok := state.info.ParentCallID.Get(); ok {
		request.ParentCallID = parent
	}
	if deadline, ok := state.info.DeadlineUnixNanos.Get(); ok {
		request.Deadline = time.Unix(0, deadline)
	}
	if len(state.path) > 0 {
		frame := state.path[len(state.path)-1]
		request.ProviderID, request.Interface, request.Function = frame.Plugin, frame.Interface, frame.Function
	}
	state.requestCount++
	if b.closing.Load() {
		event := &FuelRequestEvent{Time: time.Now(), CallID: request.CallID, PluginID: request.PluginID, CallPath: append([]PluginCallFrame(nil), state.path...), Requested: amount, GuestReason: sanitizeGuestReason(reason), RemainingBefore: current, DenialReason: string(FuelDeniedRuntimeClosing)}
		return map[string]any{"type": "host_result", "values": []any{map[string]any{"err": map[string]any{"case": string(FuelDeniedRuntimeClosing)}}}}, event
	}
	decision, denial, err := decideFuelRequest(ctx, b.system.policy, b.system.limits, request)
	if !available && err == nil {
		denial, err = FuelDeniedDisabled, ErrFuelRequestDisabled
	}
	event := &FuelRequestEvent{Time: time.Now(), CallID: request.CallID, PluginID: request.PluginID, CallPath: append([]PluginCallFrame(nil), state.path...), Requested: amount, GuestReason: sanitizeGuestReason(reason), RemainingBefore: current}
	if err != nil {
		event.DenialReason = string(denial)
		variant := map[string]any{"case": string(denial)}
		if denial == FuelDeniedRequestTooLarge {
			variant["value"] = b.system.limits.MaxGrantPerRequest
		}
		return map[string]any{"type": "host_result", "values": []any{map[string]any{"err": variant}}}, event
	}
	state.totalGranted += decision.Grant
	event.Granted = decision.Grant
	event.RemainingAfter = current + decision.Grant
	grant := FuelGrant{Requested: amount, Granted: decision.Grant, Remaining: event.RemainingAfter}
	return map[string]any{"type": "host_result", "fuel_grant": decision.Grant, "values": []any{map[string]any{"ok": grant}}}, event
}

func sanitizeGuestReason(reason string) string {
	reason = strings.ReplaceAll(reason, "\r", "\\r")
	return strings.ReplaceAll(reason, "\n", "\\n")
}

func (b *componentBridge) observeFuelRequest(event FuelRequestEvent) {
	if b.system.observer == nil {
		return
	}
	func() { defer func() { _ = recover() }(); b.system.observer.OnFuelRequest(event) }()
}

func (b *componentBridge) releaseHandle(id uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrRuntimeClosed
	}
	if err := b.write(map[string]any{"type": "handle_drop", "handle": id}); err != nil {
		return err
	}
	message, err := b.read()
	if err != nil {
		return err
	}
	if message.Type != "result" {
		return messageError(message)
	}
	delete(b.handleStates, id)
	return nil
}

func (b *componentBridge) bindHandles(value any) any {
	switch value := value.(type) {
	case []any:
		result := make([]any, len(value))
		for index := range value {
			result[index] = b.bindHandles(value[index])
		}
		return result
	case map[string]any:
		if rawID, ok := value["$witgo_handle"]; ok {
			id, err := interfaceUint64(rawID)
			kind, kindOK := value["kind"].(string)
			if err == nil && kindOK {
				owned, _ := value["owned"].(bool)
				return b.newBoundHandle(handleWire{ID: id, Kind: HandleKind(kind), Owned: owned})
			}
		}
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[key] = b.bindHandles(item)
		}
		return result
	default:
		return value
	}
}

func (b *componentBridge) newBoundHandle(wire handleWire) Handle {
	handle := newHandle(b, wire)
	if b.handleStates == nil {
		b.handleStates = make(map[uint64][]*handleState)
	}
	b.handleStates[wire.ID] = append(b.handleStates[wire.ID], handle.state)
	return handle
}

func (b *componentBridge) markHandlesConsumed(ids []uint64) {
	for _, id := range ids {
		for _, state := range b.handleStates[id] {
			state.mu.Lock()
			state.closed = true
			state.mu.Unlock()
		}
		delete(b.handleStates, id)
	}
}

func interfaceUint64(value any) (uint64, error) {
	switch value := value.(type) {
	case json.Number:
		return parseUint(value.String())
	case float64:
		if value < 0 || value != float64(uint64(value)) {
			return 0, fmt.Errorf("invalid handle identifier %v", value)
		}
		return uint64(value), nil
	case uint64:
		return value, nil
	default:
		return 0, fmt.Errorf("invalid handle identifier %T", value)
	}
}

func (b *componentBridge) fuel() (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.write(map[string]any{"type": "fuel"}); err != nil {
		return 0, err
	}
	message, err := b.read()
	if err != nil {
		return 0, err
	}
	if message.Type != "result" {
		return 0, messageError(message)
	}
	switch value := message.Value.(type) {
	case json.Number:
		return parseUint(value.String())
	case float64:
		return uint64(value), nil
	default:
		return 0, fmt.Errorf("invalid fuel value %T", value)
	}
}

func (b *componentBridge) setFuel(fuel uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.write(map[string]any{"type": "set_fuel", "fuel": fuel}); err != nil {
		return err
	}
	message, err := b.read()
	if err != nil {
		return err
	}
	if message.Type != "result" {
		return messageError(message)
	}
	return nil
}

func (b *componentBridge) write(value any) error {
	if b.closed {
		return errors.New("component bridge is closed")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if b.maxResultBytes > 0 && uint64(len(data)) > b.maxResultBytes {
		return fmt.Errorf("%w: %d bytes, limit is %d", ErrResultTooLarge, len(data), b.maxResultBytes)
	}
	return b.native.write(data)
}

func (b *componentBridge) read() (bridgeMessage, error) {
	line, err := b.native.read()
	if err != nil {
		return bridgeMessage{}, fmt.Errorf("read component bridge library: %w", err)
	}
	if b.maxResultBytes > 0 && uint64(len(line)) > b.maxResultBytes {
		return bridgeMessage{}, fmt.Errorf("%w: %d bytes, limit is %d", ErrResultTooLarge, len(line), b.maxResultBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(string(line)))
	decoder.UseNumber()
	var message bridgeMessage
	if err := decoder.Decode(&message); err != nil {
		return message, fmt.Errorf("decode component bridge response: %w", err)
	}
	return message, nil
}

func (b *componentBridge) close() error {
	b.closing.Store(true)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	return b.native.close()
}
func (b *componentBridge) abort() error {
	b.closed = true
	return b.native.close()
}
func (b *componentBridge) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
func messageError(message bridgeMessage) error {
	if message.Error == "" {
		return fmt.Errorf("component bridge returned %q", message.Type)
	}
	if strings.Contains(message.Error, "handle") && strings.Contains(message.Error, "closed or unknown") {
		return fmt.Errorf("%w: %s", ErrHandleClosed, message.Error)
	}
	return errors.New(message.Error)
}
func parseUint(value string) (uint64, error) {
	var result uint64
	_, err := fmt.Sscan(value, &result)
	return result, err
}
