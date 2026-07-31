package witgo

import (
	"errors"
	"fmt"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
)

var ErrFuelExhausted = errors.New("WebAssembly fuel exhausted")

func (r *Runtime) FuelRemaining() (uint64, error) {
	if r == nil {
		return 0, errors.New("runtime is nil")
	}
	return storeFuel(r.store())
}

func (r *Runtime) SetFuel(fuel uint64) error {
	if r == nil {
		return errors.New("runtime is nil")
	}
	return setStoreFuel(r.store(), fuel)
}

func (r *Runtime) store() *wasmtime.Store {
	if r.Module != nil {
		return r.Module.Store
	}
	if r.Component != nil {
		return r.Component.Store
	}
	return nil
}

func storeFuel(store *wasmtime.Store) (uint64, error) {
	if store == nil {
		return 0, errors.New("runtime store is not initialized")
	}
	fuel, err := store.GetFuel()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFuelDisabled, err)
	}
	return fuel, nil
}

func setStoreFuel(store *wasmtime.Store, fuel uint64) error {
	if store == nil {
		return errors.New("runtime store is not initialized")
	}
	if err := store.SetFuel(fuel); err != nil {
		return fmt.Errorf("%w: %v", ErrFuelDisabled, err)
	}
	return nil
}

func callError(name string, err error) error {
	var trap *wasmtime.Trap
	if errors.As(err, &trap) {
		code := trap.Code()
		if code != nil && *code == wasmtime.OutOfFuel {
			return fmt.Errorf("call wasm function %q: %w: %v", name, ErrFuelExhausted, err)
		}
	}
	return fmt.Errorf("call wasm function %q: %w", name, err)
}
