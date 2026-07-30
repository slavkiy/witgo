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
)

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
	normalizedPath, err := ipath.NormalizePath(filename)
	if err != nil {
		return nil, fmt.Errorf("normalize wasm path: %w", err)
	}

	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", normalizedPath, err)
	}

	return LoadRuntimeFromBytes(data)
}

func LoadRuntimeFromBytes(data []byte) (*Runtime, error) {
	if !iwasm.IsWasm(data) {
		return nil, errors.New("data is not a valid WebAssembly binary")
	}

	switch kind := iwasm.DetectKind(data); kind {
	case iwasm.KindCoreModule:
		moduleRuntime, err := newModuleRuntime(data)
		if err != nil {
			return nil, err
		}

		return &Runtime{
			Kind:   kind,
			Module: moduleRuntime,
		}, nil

	case iwasm.KindComponent:
		componentRuntime, err := newComponentRuntime(data)
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

func newModuleRuntime(data []byte) (*ModuleRuntime, error) {
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)

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

func newComponentRuntime(data []byte) (*ComponentRuntime, error) {
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)

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
