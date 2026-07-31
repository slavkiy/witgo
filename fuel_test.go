package witgo

import (
	"errors"
	"testing"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
)

func TestRuntimeFuelStopsRunawayModule(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`
		(module
			(func (export "spin")
				(loop
					br 0
				)
			)
			(func (export "answer") (result i32)
				i32.const 42
			)
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	runtime, err := LoadRuntimeFromBytesWithOptions(wasm, RuntimeOptions{Fuel: 1_000})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.Call("spin"); !errors.Is(err, ErrFuelExhausted) {
		t.Fatalf("Call(spin) error = %v, want ErrFuelExhausted", err)
	}

	remaining, err := runtime.FuelRemaining()
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("remaining fuel = %d, want 0", remaining)
	}

	if err := runtime.SetFuel(100); err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Call("answer")
	if err != nil {
		t.Fatal(err)
	}
	if result != int32(42) {
		t.Fatalf("answer = %#v, want int32(42)", result)
	}
}

func TestRuntimeFuelDisabledByDefault(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`(module)`)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := LoadRuntimeFromBytes(wasm)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.FuelRemaining(); !errors.Is(err, ErrFuelDisabled) {
		t.Fatalf("FuelRemaining error = %v, want ErrFuelDisabled", err)
	}
}
