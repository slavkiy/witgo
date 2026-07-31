package witgo

import (
	"errors"
	"fmt"
	"os"

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
//
// Fuel is the initial instruction budget shared by all calls made through the
// runtime. A zero value disables fuel metering for backwards compatibility.
type RuntimeOptions struct {
	Fuel uint64
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
}

type ModuleRuntime struct {
	Store    *wasmtime.Store
	Module   *wasmtime.Module
	Instance *wasmtime.Instance
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
	if options.Fuel == 0 {
		engine := wasmtime.NewEngine()
		return engine, wasmtime.NewStore(engine), nil
	}

	config := wasmtime.NewConfig()
	config.SetConsumeFuel(true)
	engine := wasmtime.NewEngineWithConfig(config)
	store := wasmtime.NewStore(engine)
	if err := store.SetFuel(options.Fuel); err != nil {
		return nil, nil, fmt.Errorf("set initial WebAssembly fuel: %w", err)
	}
	return engine, store, nil
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
