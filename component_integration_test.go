package witgo

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/slavkiy/witgo/internal/bridgebin"
)

const passthroughComponent = `(component
  (type $host-type (instance
    (type $process-type (func (param "value" string) (result string)))
    (export "process-string" (func (type $process-type)))
  ))
  (import "test:plugin/host@1.0.0" (instance $host (type $host-type)))
  (alias export $host "process-string" (func $process))

  (core module $memory-module
    (memory (export "memory") 1)
    (global $heap (mut i32) (i32.const 1024))
    (func (export "realloc") (param i32 i32 i32 i32) (result i32)
      (local $result i32)
      global.get $heap
      local.tee $result
      local.get 3
      i32.add
      global.set $heap
      local.get $result)
  )
  (core instance $memory-instance (instantiate $memory-module))
  (alias core export $memory-instance "memory" (core memory $memory))
  (alias core export $memory-instance "realloc" (core func $realloc))
  (core func $lowered-process (canon lower (func $process)
    (memory $memory) (realloc $realloc) string-encoding=utf8))

  (core module $plugin-module
    (import "host" "memory" (memory 1))
    (import "host" "process-string" (func $host-process (param i32 i32 i32)))
    (func (export "run") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.const 0
      call $host-process
      i32.const 0)
  )
  (core instance $host-core
    (export "memory" (memory $memory))
    (export "process-string" (func $lowered-process)))
  (core instance $plugin-core (instantiate $plugin-module (with "host" (instance $host-core))))
  (alias core export $plugin-core "run" (core func $run-core))
  (type $run-type (func (param "value" string) (result string)))
  (func $run (type $run-type) (canon lift (core func $run-core)
    (memory $memory) (realloc $realloc) string-encoding=utf8))
  (instance $api
    (export "run" (func $run))
  )
  (export "test:plugin/api@1.0.0" (instance $api))
)`

const valueTypesComponent = `(component
  (core module $m
    (func (export "id-enum") (param i32) (result i32) local.get 0)
    (func (export "id-flags") (param i32) (result i32) local.get 0)
    (func (export "id-char") (param i32) (result i32) local.get 0)
  )
  (core instance $i (instantiate $m))
  (alias core export $i "id-enum" (core func $id-enum))
  (alias core export $i "id-flags" (core func $id-flags))
  (alias core export $i "id-char" (core func $id-char))

  (type $color (enum "red" "green" "blue"))
  (type $permissions (flags "read" "write" "admin"))
  (func $enum (param "value" $color) (result $color)
    (canon lift (core func $id-enum)))
  (func $flags (param "value" $permissions) (result $permissions)
    (canon lift (core func $id-flags)))
  (func $char (param "value" char) (result char)
    (canon lift (core func $id-char)))
  (instance $api
    (export "color" (type $color))
    (export "permissions" (type $permissions))
    (export "roundtrip-enum" (func $enum))
    (export "roundtrip-flags" (func $flags))
    (export "roundtrip-char" (func $char))
  )
  (export "test:types/api@1.0.0" (instance $api))
)`

const compositeTypesComponent = `(component
  (core module $m
    (memory (export "memory") 1)
    (global $heap (mut i32) (i32.const 1024))
    (func (export "realloc") (param i32 i32 i32 i32) (result i32)
      global.get $heap
      global.get $heap
      local.get 3
      i32.add
      global.set $heap)
    (func (export "variant-id") (param i32) (result i32)
      local.get 0)
    (func (export "list-id") (param i32 i32) (result i32)
      i32.const 8
      local.get 0
      i32.store
      i32.const 12
      local.get 1
      i32.store
      i32.const 8)
  )
  (core instance $i (instantiate $m))
  (alias core export $i "memory" (core memory $memory))
  (alias core export $i "realloc" (core func $realloc))
  (alias core export $i "variant-id" (core func $variant-id))
  (alias core export $i "list-id" (core func $list-id))

  (type $choice (variant (case "none") (case "some")))
  (type $matrix (list (list u32)))
  (func $roundtrip-variant (param "value" $choice) (result $choice)
    (canon lift (core func $variant-id)))
  (func $roundtrip-list (param "value" $matrix) (result $matrix)
    (canon lift (core func $list-id) (memory $memory) (realloc $realloc)))
  (instance $api
    (export "choice" (type $choice))
    (export "matrix" (type $matrix))
    (export "roundtrip-variant" (func $roundtrip-variant))
    (export "roundtrip-list" (func $roundtrip-list))
  )
  (export "test:composite/api@1.0.0" (instance $api))
)`

const resourceContractComponent = `(component
  (import "item" (type $item (sub resource)))
  (type $use-item (func (param "item" (borrow $item))))
  (import "use-item" (func (type $use-item)))
)`

const liveResourceComponent = `(component
  (core module $drop-module
    (func (export "drop") (param i32)))
  (core instance $drop-instance (instantiate $drop-module))
  (alias core export $drop-instance "drop" (core func $drop))
  (type $item (resource (rep i32) (dtor (func $drop))))
  (core func $item-new (canon resource.new $item))
  (core instance $intrinsics
    (export "new" (func $item-new)))

  (core module $implementation
    (import "" "new" (func $new (param i32) (result i32)))
    (func (export "make") (result i32)
      i32.const 7
      call $new)
    (func (export "value") (param i32) (result i32)
      local.get 0)
    (func (export "consume") (param i32)))
  (core instance $implementation-instance
    (instantiate $implementation (with "" (instance $intrinsics))))
  (alias core export $implementation-instance "make" (core func $make))
  (alias core export $implementation-instance "value" (core func $value))
  (alias core export $implementation-instance "consume" (core func $consume))

  (type $make-type (func (result (own $item))))
  (func $make-lifted (type $make-type) (canon lift (core func $make)))
  (type $value-type (func (param "self" (borrow $item)) (result u32)))
  (func $value-lifted (type $value-type) (canon lift (core func $value)))
  (type $consume-type (func (param "self" (own $item))))
  (func $consume-lifted (type $consume-type) (canon lift (core func $consume)))
  (instance $api
    (export "item" (type $item))
    (export "make" (func $make-lifted))
    (export "value" (func $value-lifted))
    (export "consume" (func $consume-lifted)))
  (export "test:handles/api@1.0.0" (instance $api))
)`

func TestComponentCallsGoHostImport(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("embedded bridge is not present for this platform")
	}
	runtime, err := LoadRuntimeFromBytesWithImports([]byte(passthroughComponent), RuntimeOptions{}, []HostImport{{
		Interface: "test:plugin/host@1.0.0",
		Function:  "process-string",
		Call:      func(args []any) (any, error) { return strings.ToUpper(args[0].(string)), nil },
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	result, err := runtime.Call("test:plugin/api@1.0.0#run", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "HELLO" {
		t.Fatalf("result = %#v, want HELLO", result)
	}
}

func TestComponentValueTypes(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("component bridge is not available")
	}
	runtime, err := LoadRuntimeFromBytes([]byte(valueTypesComponent))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	tests := []struct {
		name string
		arg  any
	}{
		{"test:types/api@1.0.0#roundtrip-enum", "green"},
		{"test:types/api@1.0.0#roundtrip-flags", []any{"read", "admin"}},
		{"test:types/api@1.0.0#roundtrip-char", "Ж"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runtime.Call(test.name, test.arg)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.arg) {
				t.Fatalf("got %#v, want %#v", got, test.arg)
			}
		})
	}
	t.Run("concurrent-calls", func(t *testing.T) {
		const count = 24
		var group sync.WaitGroup
		errors := make(chan error, count)
		for i := 0; i < count; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				got, err := runtime.Call("test:types/api@1.0.0#roundtrip-char", "Я")
				if err == nil && got != "Я" {
					err = fmt.Errorf("got %#v", got)
				}
				errors <- err
			}()
		}
		group.Wait()
		for i := 0; i < count; i++ {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestComponentVariantsAndComplexLists(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("component bridge is not available")
	}
	runtime, err := LoadRuntimeFromBytes([]byte(compositeTypesComponent))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	variant := map[string]any{"case": "some", "value": map[string]any{"none": true}}
	gotVariant, err := runtime.Call("test:composite/api@1.0.0#roundtrip-variant", variant)
	if err != nil {
		t.Fatal(err)
	}
	variantObject, ok := gotVariant.(map[string]any)
	if !ok || variantObject["case"] != "some" {
		t.Fatalf("variant result = %#v", gotVariant)
	}

	matrix := []any{[]any{uint32(1), uint32(2)}, []any{}, []any{uint32(3)}}
	gotMatrix, err := runtime.Call("test:composite/api@1.0.0#roundtrip-list", matrix)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(gotMatrix) != "[[1 2] [] [3]]" {
		t.Fatalf("complex list result = %#v", gotMatrix)
	}
}

func TestComponentResourceContractInspection(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("component bridge is not available")
	}
	contract, err := InspectComponentBytes([]byte(resourceContractComponent))
	if err != nil {
		t.Fatal(err)
	}
	if !contract.Requires("use-item") {
		t.Fatalf("resource contract imports = %v", contract.Imports)
	}
	if signature, ok := contract.Signature("use-item"); !ok || signature != "(borrow)->()" {
		t.Fatalf("resource signature = %q, %v", signature, ok)
	}
}

func TestComponentLiveResourceHandle(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("component bridge is not available")
	}
	runtime, err := LoadRuntimeFromBytes([]byte(liveResourceComponent))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	value, err := runtime.Call("test:handles/api@1.0.0#make")
	if err != nil {
		t.Fatal(err)
	}
	handle, ok := value.(Handle)
	if !ok || handle.Kind() != HandleResource || !handle.Owned() {
		t.Fatalf("resource result = %#v", value)
	}
	result, err := runtime.Call("test:handles/api@1.0.0#value", handle)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(result) != "7" {
		t.Fatalf("resource value = %#v", result)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if !handle.IsClosed() {
		t.Fatal("closed resource handle is reported open")
	}

	value, err = runtime.Call("test:handles/api@1.0.0#make")
	if err != nil {
		t.Fatal(err)
	}
	consumed := value.(Handle)
	if _, err := runtime.Call("test:handles/api@1.0.0#consume", consumed); err != nil {
		t.Fatal(err)
	}
	if !consumed.IsClosed() {
		t.Fatal("transferred own resource handle is reported open")
	}
}

func TestComponentVersionHandshake(t *testing.T) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		t.Skip("component bridge is not available")
	}
	contract, err := InspectComponentBytes([]byte("(component)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.FunctionNames()) != 0 {
		t.Fatalf("empty component functions = %v", contract.FunctionNames())
	}
}
