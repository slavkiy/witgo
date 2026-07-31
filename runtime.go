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
	ErrUnknownWasmKind = errors.New("unknown WebAssembly kind")
	ErrCoreModule      = errors.New("core WebAssembly modules are not supported; build a Component Model component")
	ErrFuelDisabled    = errors.New("WebAssembly fuel metering is disabled")
)

// RuntimeOptions controls resource limits for a Component Model runtime.
type RuntimeOptions struct {
	Fuel             uint64
	FuelPerCall      uint64
	Timeout          time.Duration
	MemoryLimitBytes int64
	MaxResultBytes   uint64
	InstanceLimit    int64
	// BridgePath overrides WITGO_COMPONENT_BRIDGE and PATH lookup.
	BridgePath string
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
	if err := validateRuntimeOptions(options); err != nil {
		return nil, err
	}
	path, err := ipath.NormalizePath(filename)
	if err != nil {
		return nil, fmt.Errorf("normalize component path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read component %q: %w", path, err)
	}
	kind := iwasm.KindUnknown
	if iwasm.IsWasm(data) {
		kind = iwasm.DetectKind(data)
	} else if bytes.HasPrefix(bytes.TrimSpace(data), []byte("(component")) {
		kind = iwasm.KindComponent
	} else {
		return nil, errors.New("data is not a valid WebAssembly component")
	}
	if kind == iwasm.KindCoreModule {
		return nil, ErrCoreModule
	}
	if kind != iwasm.KindComponent {
		return nil, fmt.Errorf("%w: %v", ErrUnknownWasmKind, kind)
	}
	bridge, err := startComponentBridge(path, options, imports)
	if err != nil {
		return nil, err
	}
	return &Runtime{Kind: kind, bridge: bridge, maxResultBytes: options.MaxResultBytes}, nil
}

func LoadRuntimeFromBytes(data []byte) (*Runtime, error) {
	return LoadRuntimeFromBytesWithOptions(data, RuntimeOptions{})
}

func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error) {
	return LoadRuntimeFromBytesWithImports(data, options, nil)
}

func LoadRuntimeFromBytesWithImports(data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error) {
	if !iwasm.IsWasm(data) && !bytes.HasPrefix(bytes.TrimSpace(data), []byte("(component")) {
		return nil, errors.New("data is not a valid WebAssembly component")
	}
	file, err := os.CreateTemp("", "witgo-component-*.wasm")
	if err != nil {
		return nil, fmt.Errorf("create temporary component: %w", err)
	}
	name := file.Name()
	if _, err = file.Write(data); err == nil {
		err = file.Close()
	} else {
		_ = file.Close()
	}
	if err != nil {
		_ = os.Remove(name)
		return nil, fmt.Errorf("write temporary component: %w", err)
	}
	runtime, err := LoadRuntimeWithImports(name, options, imports)
	if err != nil {
		_ = os.Remove(name)
		return nil, err
	}
	runtime.temporaryFile = name
	return runtime, nil
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
	return nil
}
