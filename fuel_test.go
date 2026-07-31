package witgo

import (
	"errors"
	"testing"
	"time"

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
	} else {
		var trap *wasmtime.Trap
		if !errors.As(err, &trap) {
			t.Fatalf("Call(spin) error = %v, want original Wasmtime trap", err)
		}
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

	_, err = runtime.FuelRemaining()
	if !errors.Is(err, ErrFuelDisabled) {
		t.Fatalf("FuelRemaining error = %v, want ErrFuelDisabled", err)
	}
	var disabled *FuelDisabledError
	if !errors.As(err, &disabled) || disabled.Cause == nil {
		t.Fatalf("FuelRemaining error = %#v, want FuelDisabledError with cause", err)
	}
}

func TestRuntimeFuelPerCallResetsBudget(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`
		(module
			(func (export "answer") (result i32)
				i32.const 42
			)
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := LoadRuntimeFromBytesWithOptions(wasm, RuntimeOptions{FuelPerCall: 10})
	if err != nil {
		t.Fatal(err)
	}

	for call := 0; call < 2; call++ {
		result, err := runtime.Call("answer")
		if err != nil {
			t.Fatalf("call %d: %v", call+1, err)
		}
		if result != int32(42) {
			t.Fatalf("call %d result = %#v", call+1, result)
		}
	}
}

func TestRuntimeTimeoutStopsRunawayModule(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`(module (func (export "spin") (loop br 0)))`)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := LoadRuntimeFromBytesWithOptions(wasm, RuntimeOptions{Timeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Call("spin"); !errors.Is(err, ErrCallTimeout) {
		t.Fatalf("Call(spin) error = %v, want ErrCallTimeout", err)
	}
}

func TestRuntimeMemoryAndResultLimits(t *testing.T) {
	tooLarge, err := wasmtime.Wat2Wasm(`(module (memory 2))`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeFromBytesWithOptions(tooLarge, RuntimeOptions{MemoryLimitBytes: 64 * 1024}); err == nil {
		t.Fatal("loading two-page memory succeeded with a one-page limit")
	}

	wasm, err := wasmtime.Wat2Wasm(`(module (memory (export "memory") 1))`)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := LoadRuntimeFromBytesWithOptions(wasm, RuntimeOptions{MaxResultBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ReadMemory(0, 9); !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("ReadMemory error = %v, want ErrResultTooLarge", err)
	}
}

func TestRuntimeOptionsRejectAmbiguousFuel(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`(module)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadRuntimeFromBytesWithOptions(wasm, RuntimeOptions{Fuel: 1, FuelPerCall: 1})
	if err == nil {
		t.Fatal("Fuel and FuelPerCall were accepted together")
	}
}
