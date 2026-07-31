package witgo

import (
	"errors"
	"fmt"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
)

// ErrFuelExhausted identifies a call trapped after consuming its fuel budget.
var ErrFuelExhausted = errors.New("WebAssembly fuel exhausted")

// ErrCallTimeout identifies WebAssembly interrupted by RuntimeOptions.Timeout.
var ErrCallTimeout = errors.New("WebAssembly call timed out")

// ExecutionLimitError preserves the original Wasmtime trap while identifying
// the witgo limit that stopped a call.
type ExecutionLimitError struct {
	Function string
	Limit    error
	Cause    error
}

func (e *ExecutionLimitError) Error() string {
	if e == nil {
		return "WebAssembly execution limit exceeded"
	}
	if e.Cause == nil {
		return fmt.Sprintf("call wasm function %q: %v", e.Function, e.Limit)
	}
	return fmt.Sprintf("call wasm function %q: %v: %v", e.Function, e.Limit, e.Cause)
}

func (e *ExecutionLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *ExecutionLimitError) Is(target error) bool {
	return e != nil && target == e.Limit
}

// FuelDisabledError preserves the Wasmtime error while remaining comparable
// with ErrFuelDisabled through errors.Is.
type FuelDisabledError struct {
	Cause error
}

func (e *FuelDisabledError) Error() string {
	if e == nil || e.Cause == nil {
		return ErrFuelDisabled.Error()
	}
	return ErrFuelDisabled.Error() + ": " + e.Cause.Error()
}

func (e *FuelDisabledError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *FuelDisabledError) Is(target error) bool { return target == ErrFuelDisabled }

// FuelRemaining returns the instruction budget remaining in the runtime.
func (r *Runtime) FuelRemaining() (uint64, error) {
	if r == nil {
		return 0, errors.New("runtime is nil")
	}
	return storeFuel(r.store())
}

// SetFuel replaces the runtime's remaining instruction budget.
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
		return 0, &FuelDisabledError{Cause: err}
	}
	return fuel, nil
}

func setStoreFuel(store *wasmtime.Store, fuel uint64) error {
	if store == nil {
		return errors.New("runtime store is not initialized")
	}
	if err := store.SetFuel(fuel); err != nil {
		return &FuelDisabledError{Cause: err}
	}
	return nil
}

func callError(name string, err error, timeoutEnabled bool) error {
	var trap *wasmtime.Trap
	if errors.As(err, &trap) {
		code := trap.Code()
		if code != nil && *code == wasmtime.OutOfFuel {
			return &ExecutionLimitError{Function: name, Limit: ErrFuelExhausted, Cause: err}
		}
		if timeoutEnabled && code != nil && *code == wasmtime.Interrupt {
			return &ExecutionLimitError{Function: name, Limit: ErrCallTimeout, Cause: err}
		}
	}
	return fmt.Errorf("call wasm function %q: %w", name, err)
}
