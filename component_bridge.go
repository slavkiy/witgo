package witgo

import (
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

	"github.com/slavkiy/witgo/internal/bridgebin"
)

type componentBridge struct {
	native         *nativeBridge
	imports        map[string]HostFunc
	maxResultBytes uint64
	mu             sync.Mutex
	closed         bool
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
}

type bridgeImportSpec struct {
	Interface string   `json:"interface"`
	Functions []string `json:"functions"`
}

func prepareHostImports(imports []HostImport) (map[string]HostFunc, []string, []bridgeImportSpec, error) {
	registered := make(map[string]HostFunc, len(imports))
	registeredNames := make([]string, 0, len(imports))
	grouped := make(map[string][]string)
	for _, item := range imports {
		if item.Interface == "" || item.Function == "" || item.Call == nil {
			return nil, nil, nil, errors.New("host import interface, function, and Call are required")
		}
		key := item.Interface + "#" + item.Function
		if _, exists := registered[key]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate host import %q", key)
		}
		registered[key] = item.Call
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

func pingComponentBridge(component string, options RuntimeOptions, imports []HostImport) (*componentBridge, bridgeMessage, []string, error) {
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
	b := &componentBridge{native: native, imports: registered, maxResultBytes: options.MaxResultBytes}
	clientVersion := witgoVersion()
	init := map[string]any{
		"type": "init", "protocol_version": bridgeProtocolVersion,
		"witgo_version": clientVersion, "component": component, "imports": specs,
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

func startComponentBridge(component string, options RuntimeOptions, imports []HostImport, contract *Contract) (*componentBridge, error) {
	b, message, registeredNames, err := pingComponentBridge(component, options, imports)
	if err != nil {
		return nil, err
	}
	if contract != nil {
		if err := compareFunctions("registered host adapters", contract.Imports, registeredNames); err != nil {
			_ = b.abort()
			return nil, err
		}
		if err := validateContract(*contract, message.Imports, message.Exports); err != nil {
			_ = b.abort()
			return nil, err
		}
		if err := compareSignatures(contract.Signatures, message.Signatures); err != nil {
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
	if message.Type != "ready" {
		_ = b.abort()
		return nil, messageError(message)
	}
	return b, nil
}

func inspectComponentBridge(component string, options RuntimeOptions) (Contract, error) {
	b, message, _, err := pingComponentBridge(component, options, nil)
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

func (b *componentBridge) call(name string, args []any) ([]any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrRuntimeClosed
	}
	if args == nil {
		args = []any{}
	}
	if err := b.write(map[string]any{"type": "call", "name": name, "args": args}); err != nil {
		return nil, err
	}
	for {
		message, err := b.read()
		if err != nil {
			return nil, err
		}
		switch message.Type {
		case "host_call":
			fn := b.imports[message.Interface+"#"+message.Function]
			if fn == nil {
				_ = b.write(map[string]any{"type": "host_result", "error": "host import is not registered"})
				continue
			}
			result, callErr := fn(message.Args)
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
			return message.Values, nil
		case "error", "fatal":
			return nil, messageError(message)
		default:
			return nil, fmt.Errorf("unexpected bridge message %q", message.Type)
		}
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
	return errors.New(message.Error)
}
func parseUint(value string) (uint64, error) {
	var result uint64
	_, err := fmt.Sscan(value, &result)
	return result, err
}
