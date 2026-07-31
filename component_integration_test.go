package witgo

import (
	"errors"
	"strings"
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

func TestComponentCallsGoHostImport(t *testing.T) {
	if _, err := bridgebin.Executable(); errors.Is(err, bridgebin.ErrUnavailable) {
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
