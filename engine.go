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
	ErrComponentCall   = errors.New(
		"calling component exports is not supported by wasmtime-go/v47",
	)
)

type WitgoCtx struct {
	Kind iwasm.Kind

	Module    *ModuleCtx
	Component *ComponentCtx
}

type ModuleCtx struct {
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

func NewEngine(filename string) (*WitgoCtx, error) {
	normalizedPath, err := ipath.NormalizePath(filename)
	if err != nil {
		return nil, fmt.Errorf("normalize wasm path: %w", err)
	}

	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", normalizedPath, err)
	}

	return NewEngineFromBytes(data)
}

func NewEngineFromBytes(data []byte) (*WitgoCtx, error) {
	if !iwasm.IsWasm(data) {
		return nil, errors.New("data is not a valid WebAssembly binary")
	}

	switch kind := iwasm.DetectKind(data); kind {
	case iwasm.KindCoreModule:
		moduleCtx, err := newModuleCtx(data)
		if err != nil {
			return nil, err
		}

		return &WitgoCtx{
			Kind:   kind,
			Module: moduleCtx,
		}, nil

	case iwasm.KindComponent:
		componentCtx, err := newComponentCtx(data)
		if err != nil {
			return nil, err
		}

		return &WitgoCtx{
			Kind:      kind,
			Component: componentCtx,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %v", ErrUnknownWasmKind, kind)
	}
}

func newModuleCtx(data []byte) (*ModuleCtx, error) {
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

	return &ModuleCtx{
		Store:    store,
		Module:   module,
		Instance: instance,
	}, nil
}

func newComponentCtx(data []byte) (*ComponentCtx, error) {
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

	return &ComponentCtx{
		Store:     store,
		Component: component,
		Instance:  instance,
		Linker:    linker,
	}, nil
}
