package witgo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

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
)

// RuntimeOptions controls resource limits for a Component Model runtime.
type RuntimeOptions struct {
	Fuel             uint64
	FuelPerCall      uint64
	Timeout          time.Duration
	MemoryLimitBytes int64
	MaxResultBytes   uint64
	InstanceLimit    int64
	// BridgePath overrides the bundled library and WITGO_COMPONENT_LIBRARY.
	BridgePath string
	// BridgeSHA256 verifies BridgePath before loading. It is required when a
	// custom bridge is supplied in security-sensitive deployments.
	BridgeSHA256 string
	// DisableEmbeddedBridge prevents extraction or loading of the library
	// shipped with witgo. Set BridgePath to use an administrator-managed library.
	DisableEmbeddedBridge bool
}

// HostFunc implements one imported WIT function. Arguments and the result use
// ordinary Go values with the same shape as their WIT types.
type HostFunc func(args []any) (any, error)

// HostImport grants a component one host capability.
type HostImport struct {
	Interface string
	Function  string
	Call      HostFunc
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
	Kind           iwasm.Kind
	bridge         *componentBridge
	maxResultBytes uint64
	temporaryFile  string
}

// WitgoCtx is kept as a compatibility alias for Runtime.
type WitgoCtx = Runtime

func LoadRuntime(filename string) (*Runtime, error) {
	return LoadRuntimeWithOptions(filename, RuntimeOptions{})
}

func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeWithImports(filename, options, nil)
}

// LoadRuntimeWithImports loads a standard WebAssembly component and exposes
// only the explicitly listed host functions to it.
func LoadRuntimeWithImports(filename string, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return loadRuntime(filename, options, imports, nil)
}

// LoadRuntimeWithContract loads a component and rejects it when its imported or
// exported function names differ from the contract embedded in generated code.
func LoadRuntimeWithContract(filename string, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return loadRuntime(filename, options, imports, &contract)
}

func loadRuntime(filename string, options RuntimeOptions, imports []HostImport, contract *Contract) (*Runtime, error) {
	if err := validateRuntimeOptions(options); err != nil {
		return nil, err
	}
	path, err := componentPath(filename)
	if err != nil {
		return nil, err
	}
	bridge, err := startComponentBridge(path, options, imports, contract)
	if err != nil {
		return nil, err
	}
	return &Runtime{Kind: iwasm.KindComponent, bridge: bridge, maxResultBytes: options.MaxResultBytes}, nil
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
	return LoadRuntimeFromBytesWithOptions(data, RuntimeOptions{})
}

func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeFromBytesWithImports(data, options, nil)
}

func LoadRuntimeFromBytesWithImports(data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	return loadRuntimeFromBytes(data, options, imports, nil)
}

// LoadRuntimeFromBytesWithContract loads an in-memory component and verifies
// its manifest before instantiation.
func LoadRuntimeFromBytesWithContract(data []byte, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error) {
	return loadRuntimeFromBytes(data, options, imports, &contract)
}

func loadRuntimeFromBytes(data []byte, options RuntimeOptions, imports []HostImport, contract *Contract) (*Runtime, error) {
	name, err := writeTemporaryComponent(data)
	if err != nil {
		return nil, err
	}
	var runtime *Runtime
	if contract == nil {
		runtime, err = LoadRuntimeWithImports(name, options, imports)
	} else {
		runtime, err = loadRuntime(name, options, imports, contract)
	}
	if err != nil {
		_ = os.Remove(name)
		return nil, err
	}
	runtime.temporaryFile = name
	return runtime, nil
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

func NewEngine(filename string) (*WitgoCtx, error)      { return LoadRuntime(filename) }
func NewEngineFromBytes(data []byte) (*WitgoCtx, error) { return LoadRuntimeFromBytes(data) }

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var closeErr error
	if r.bridge != nil {
		closeErr = r.bridge.close()
	}
	if r.temporaryFile != "" {
		if err := os.Remove(r.temporaryFile); closeErr == nil && err != nil && !errors.Is(err, os.ErrNotExist) {
			closeErr = err
		}
		r.temporaryFile = ""
	}
	return closeErr
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
