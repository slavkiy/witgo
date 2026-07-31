package witgo

import (
	"errors"
	"fmt"
)

var ErrFuelExhausted = errors.New("WebAssembly fuel exhausted")
var ErrCallTimeout = errors.New("WebAssembly call timed out")

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
		return fmt.Sprintf("call component function %q: %v", e.Function, e.Limit)
	}
	return fmt.Sprintf("call component function %q: %v: %v", e.Function, e.Limit, e.Cause)
}
func (e *ExecutionLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
func (e *ExecutionLimitError) Is(target error) bool { return e != nil && target == e.Limit }

type FuelDisabledError struct{ Cause error }

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

func (r *Runtime) FuelRemaining() (uint64, error) {
	if r == nil || r.bridge == nil {
		return 0, errors.New("component runtime is not initialized")
	}
	fuel, err := r.bridge.fuel()
	if err != nil {
		return 0, &FuelDisabledError{Cause: err}
	}
	return fuel, nil
}

func (r *Runtime) SetFuel(fuel uint64) error {
	if r == nil || r.bridge == nil {
		return errors.New("component runtime is not initialized")
	}
	if err := r.bridge.setFuel(fuel); err != nil {
		return &FuelDisabledError{Cause: err}
	}
	return nil
}
