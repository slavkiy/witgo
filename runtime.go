package witgo

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	ipath "github.com/slavkiy/witgo/internal/path"
	iwasm "github.com/slavkiy/witgo/internal/wasm"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
)

var (
	ErrUnknownWasmKind = errors.New("unknown WebAssembly kind")
	// ErrFuelDisabled is returned by fuel methods for an unlimited runtime.
	ErrFuelDisabled = errors.New("WebAssembly fuel metering is disabled")
)

// RuntimeOptions controls resource limits for a WebAssembly runtime.
type RuntimeOptions struct {
	// Fuel is the initial cumulative instruction budget. Prefer FuelPerCall
	// when every invocation should receive an independent budget.
	Fuel uint64
	// FuelPerCall resets the instruction budget before every exported call.
	FuelPerCall uint64
	// Timeout interrupts WebAssembly execution after this duration. It cannot
	// interrupt a blocking Go host function.
	Timeout time.Duration
	// MemoryLimitBytes limits the size of each linear memory in the store.
	MemoryLimitBytes int64
	// MaxResultBytes limits data copied from exported Wasm memory.
	MaxResultBytes uint64
	// InstanceLimit limits the number of Wasm instances in the store.
	InstanceLimit int64
}

type callLimits struct {
	engine         *wasmtime.Engine
	fuelPerCall    uint64
	timeout        time.Duration
	maxResultBytes uint64
	mu             sync.Mutex
}

type WitgoCtx struct {
	Kind      iwasm.Kind
	Module    *ModuleCtx
	Component *ComponentCtx
}

type Runtime struct {
	Kind      iwasm.Kind
	Module    *ModuleRuntime
	Component *ComponentRuntime
}

type ModuleCtx struct {
	Store    *wasmtime.Store
	Module   *wasmtime.Module
	Instance *wasmtime.Instance
	limits   *callLimits
}

type ModuleRuntime struct {
	Store    *wasmtime.Store
	Module   *wasmtime.Module
	Instance *wasmtime.Instance
	limits   *callLimits
}

type ComponentCtx struct {
	Store     *wasmtime.Store
	Component *wasmtime.Component
	Instance  *wasmtime.ComponentInstance
	Linker    *wasmtime.ComponentLinker
}

type ComponentRuntime struct {
	Store     *wasmtime.Store
	Component *wasmtime.Component
	Instance  *wasmtime.ComponentInstance
	Linker    *wasmtime.ComponentLinker
}

func LoadRuntime(filename string) (*Runtime, error) {
	return LoadRuntimeWithOptions(filename, RuntimeOptions{})
}

// LoadRuntimeWithOptions loads a WebAssembly runtime with resource limits.
func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error) {
	normalizedPath, err := ipath.NormalizePath(filename)
	if err != nil {
		return nil, fmt.Errorf("normalize wasm path: %w", err)
	}

	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", normalizedPath, err)
	}

	return LoadRuntimeFromBytesWithOptions(data, options)
}

func LoadRuntimeFromBytes(data []byte) (*Runtime, error) {
	return LoadRuntimeFromBytesWithOptions(data, RuntimeOptions{})
}

// LoadRuntimeFromBytesWithOptions loads a WebAssembly runtime from memory with
// resource limits.
func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error) {
	if err := validateRuntimeOptions(options); err != nil {
		return nil, err
	}
	if !iwasm.IsWasm(data) {
		return nil, errors.New("data is not a valid WebAssembly binary")
	}

	switch kind := iwasm.DetectKind(data); kind {
	case iwasm.KindCoreModule:
		moduleRuntime, err := newModuleRuntime(data, options)
		if err != nil {
			return nil, err
		}

		return &Runtime{
			Kind:   kind,
			Module: moduleRuntime,
		}, nil

	case iwasm.KindComponent:
		componentRuntime, err := newComponentRuntime(data, options)
		if err != nil {
			return nil, err
		}

		return &Runtime{
			Kind:      kind,
			Component: componentRuntime,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %v", ErrUnknownWasmKind, kind)
	}
}

func NewEngine(filename string) (*WitgoCtx, error) {
	runtime, err := LoadRuntime(filename)
	if err != nil {
		return nil, err
	}

	return runtime.legacyContext(), nil
}

func NewEngineFromBytes(data []byte) (*WitgoCtx, error) {
	runtime, err := LoadRuntimeFromBytes(data)
	if err != nil {
		return nil, err
	}

	return runtime.legacyContext(), nil
}

func newModuleRuntime(data []byte, options RuntimeOptions) (*ModuleRuntime, error) {
	engine, store, err := newStore(options)
	if err != nil {
		return nil, err
	}

	module, err := wasmtime.NewModule(engine, data)
	if err != nil {
		return nil, fmt.Errorf("compile core wasm module: %w", err)
	}

	instance, err := wasmtime.NewInstance(
		store,
		module,
		[]wasmtime.AsExtern{},
	)
	if err != nil {
		return nil, fmt.Errorf("instantiate core wasm module: %w", err)
	}

	return &ModuleRuntime{
		Store:    store,
		Module:   module,
		Instance: instance,
		limits: &callLimits{
			engine:         engine,
			fuelPerCall:    options.FuelPerCall,
			timeout:        options.Timeout,
			maxResultBytes: options.MaxResultBytes,
		},
	}, nil
}

func newComponentRuntime(data []byte, options RuntimeOptions) (*ComponentRuntime, error) {
	engine, store, err := newStore(options)
	if err != nil {
		return nil, err
	}

	component, err := wasmtime.NewComponent(engine, data)
	if err != nil {
		return nil, fmt.Errorf("compile wasm component: %w", err)
	}

	linker := wasmtime.NewComponentLinker(engine)

	instance, err := linker.Instantiate(store, component)
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm component: %w", err)
	}

	return &ComponentRuntime{
		Store:     store,
		Component: component,
		Instance:  instance,
		Linker:    linker,
	}, nil
}

func newStore(options RuntimeOptions) (*wasmtime.Engine, *wasmtime.Store, error) {
	config := wasmtime.NewConfig()
	fuelEnabled := options.Fuel > 0 || options.FuelPerCall > 0
	if fuelEnabled {
		config.SetConsumeFuel(true)
	}
	if options.Timeout > 0 {
		config.SetEpochInterruption(true)
	}
	engine := wasmtime.NewEngineWithConfig(config)
	store := wasmtime.NewStore(engine)
	initialFuel := options.Fuel
	if options.FuelPerCall > 0 {
		initialFuel = options.FuelPerCall
	}
	if fuelEnabled {
		if err := store.SetFuel(initialFuel); err != nil {
			return nil, nil, fmt.Errorf("set initial WebAssembly fuel: %w", err)
		}
	}
	if options.MemoryLimitBytes > 0 || options.InstanceLimit > 0 {
		store.Limiter(limitOrDefault(options.MemoryLimitBytes), -1, limitOrDefault(options.InstanceLimit), -1, -1)
	}
	return engine, store, nil
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

func limitOrDefault(value int64) int64 {
	if value == 0 {
		return -1
	}
	return value
}

func (r *Runtime) legacyContext() *WitgoCtx {
	if r == nil {
		return nil
	}

	return &WitgoCtx{
		Kind:      r.Kind,
		Module:    r.Module.legacyRuntime(),
		Component: r.Component.legacyRuntime(),
	}
}

func (r *ModuleRuntime) legacyRuntime() *ModuleCtx {
	if r == nil {
		return nil
	}

	return &ModuleCtx{
		Store:    r.Store,
		Module:   r.Module,
		Instance: r.Instance,
		limits:   r.limits,
	}
}

func (r *ComponentRuntime) legacyRuntime() *ComponentCtx {
	if r == nil {
		return nil
	}

	return &ComponentCtx{
		Store:     r.Store,
		Component: r.Component,
		Instance:  r.Instance,
		Linker:    r.Linker,
	}
}
