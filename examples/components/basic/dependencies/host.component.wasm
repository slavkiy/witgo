(component
  (core module $host-module
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
    (func (export "process-string") (param i32 i32) (result i32)
      local.get 0)
  )
  (core instance $host-instance (instantiate $host-module))
  (alias core export $host-instance "memory" (core memory $memory))
  (alias core export $host-instance "realloc" (core func $realloc))
  (alias core export $host-instance "process-string" (core func $process-string))
  (type $process-type (func (param "value" string) (result string)))
  (func $process (type $process-type) (canon lift (core func $process-string)
    (memory $memory) (realloc $realloc) string-encoding=utf8))
  (instance $host
    (export "process-string" (func $process)))
  (export "examples:contract/host@1.0.0" (instance $host))
)
