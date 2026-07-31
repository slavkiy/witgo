package witgo

import (
	"errors"
	"fmt"
	"time"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
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

	return callModule(mc.Store, mc.Instance, mc.limits, name, args...)
}

func (mr *ModuleRuntime) Call(name string, args ...interface{}) (interface{}, error) {
	if mr == nil || mr.Store == nil || mr.Instance == nil {
		return nil, errors.New("module runtime is not initialized")
	}

	return callModule(mr.Store, mr.Instance, mr.limits, name, args...)
}

func callModule(store *wasmtime.Store, instance *wasmtime.Instance, limits *callLimits, name string, args ...interface{}) (interface{}, error) {
	if limits != nil {
		limits.mu.Lock()
		defer limits.mu.Unlock()
		if limits.fuelPerCall > 0 {
			if err := store.SetFuel(limits.fuelPerCall); err != nil {
				return nil, fmt.Errorf("reset fuel for wasm function %q: %w", name, err)
			}
		}
	}

	fn := instance.GetFunc(store, name)
	if fn == nil {
		return nil, fmt.Errorf("wasm function %q not found", name)
	}

	stopTimeout := startCallTimeout(store, limits)
	result, err := fn.Call(store, args...)
	stopTimeout()
	if err != nil {
		return nil, callError(name, err, limits != nil && limits.timeout > 0)
	}

	return result, nil
}

func startCallTimeout(store *wasmtime.Store, limits *callLimits) func() {
	if limits == nil || limits.timeout <= 0 {
		return func() {}
	}
	store.SetEpochDeadline(1)
	done := make(chan struct{})
	timer := time.AfterFunc(limits.timeout, func() {
		limits.engine.IncrementEpoch()
		close(done)
	})
	return func() {
		if !timer.Stop() {
			<-done
		}
	}
}
