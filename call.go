package witgo

import (
	"errors"
	"fmt"
)

var (
	ErrComponentCall = errors.New("calling component exports is not supported by wasmtime-go/v47")
)

func (wc *WitgoCtx) Call(name string, args ...interface{}) (interface{}, error) {
	if wc == nil {
		return nil, errors.New("witgo context is nil")
	}

	if wc.Module == nil {
		if wc.Component != nil {
			return nil, fmt.Errorf("%w: %q", ErrComponentCall, name)
		}

		return nil, errors.New("witgo context is not initialized")
	}

	return wc.Module.Call(name, args...)
}

func (r *Runtime) Call(name string, args ...interface{}) (interface{}, error) {
	if r == nil {
		return nil, errors.New("runtime is nil")
	}

	if r.Module == nil {
		if r.Component != nil {
			return nil, fmt.Errorf("%w: %q", ErrComponentCall, name)
		}

		return nil, errors.New("runtime is not initialized")
	}

	return r.Module.Call(name, args...)
}

func (mc *ModuleCtx) Call(name string, args ...interface{}) (interface{}, error) {
	if mc == nil || mc.Store == nil || mc.Instance == nil {
		return nil, errors.New("module context is not initialized")
	}

	fn := mc.Instance.GetFunc(mc.Store, name)
	if fn == nil {
		return nil, fmt.Errorf("wasm function %q not found", name)
	}

	result, err := fn.Call(mc.Store, args...)
	if err != nil {
		return nil, fmt.Errorf("call wasm function %q: %w", name, err)
	}

	return result, nil
}

func (mr *ModuleRuntime) Call(name string, args ...interface{}) (interface{}, error) {
	if mr == nil || mr.Store == nil || mr.Instance == nil {
		return nil, errors.New("module runtime is not initialized")
	}

	fn := mr.Instance.GetFunc(mr.Store, name)
	if fn == nil {
		return nil, fmt.Errorf("wasm function %q not found", name)
	}

	result, err := fn.Call(mr.Store, args...)
	if err != nil {
		return nil, fmt.Errorf("call wasm function %q: %w", name, err)
	}

	return result, nil
}
