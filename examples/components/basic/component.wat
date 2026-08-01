(component
  (type $host-type (instance
    (type $process-type (func (param "value" string) (result string)))
    (export "process-string" (func (type $process-type)))
  ))
  (import "examples:contract/host@1.0.0" (instance $host (type $host-type)))
  (alias export $host "process-string" (func $process))

  (core module $memory-module
    (memory (export "memory") 1)
    (global $heap (mut i32) (i32.const 2048))
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
    (data (i32.const 128) "image-resizer")
    (data (i32.const 160) "1.4.0")
    (data (i32.const 176) "Example Team")
    (data (i32.const 208) "Resizes uploaded images and creates previews.")
    (func (export "metadata") (result i32)
      ;; Ask the Go host to process the plugin name. The lowered host function
      ;; writes the returned string's pointer/length pair at offset 64.
      i32.const 128
      i32.const 13
      i32.const 64
      call $host-process

      i32.const 0
      i32.const 64
      i32.load
      i32.store
      i32.const 4
      i32.const 68
      i32.load
      i32.store

      i32.const 8
      i32.const 160
      i32.store
      i32.const 12
      i32.const 5
      i32.store

      i32.const 16
      i32.const 176
      i32.store
      i32.const 20
      i32.const 12
      i32.store

      i32.const 24
      i32.const 208
      i32.store
      i32.const 28
      i32.const 45
      i32.store
      i32.const 0)
  )
  (core instance $host-core
    (export "memory" (memory $memory))
    (export "process-string" (func $lowered-process)))
  (core instance $plugin-core (instantiate $plugin-module (with "host" (instance $host-core))))
  (alias core export $plugin-core "metadata" (core func $metadata-core))

  (type $metadata-record (record
    (field "name" string)
    (field "version" string)
    (field "author" string)
    (field "description" string)))
  (type $metadata-type (func (result $metadata-record)))
  (export "plugin-metadata" (type $metadata-record))
  (func $metadata (type $metadata-type) (canon lift (core func $metadata-core)
    (memory $memory) (realloc $realloc) string-encoding=utf8))

  (instance $plugin-info
    (export "plugin-metadata" (type $metadata-record))
    (export "metadata" (func $metadata)))
  (export "examples:contract/plugin-info@1.0.0" (instance $plugin-info))
)
