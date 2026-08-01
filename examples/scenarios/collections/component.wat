(component
  (core module $m
    (memory (export "memory") 1)
    (global $heap (mut i32) (i32.const 1024))
    (func (export "realloc") (param i32 i32 i32 i32) (result i32)
      global.get $heap
      global.get $heap
      local.get 3
      i32.add
      global.set $heap)
    (func (export "variant-id") (param i32 i32 i32) (result i32)
      i32.const 24
      local.get 0
      i32.store
      i32.const 28
      local.get 1
      i32.store
      i32.const 32
      local.get 2
      i32.store
      i32.const 24)
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

  (type $choice (variant (case "none") (case "some" (option string))))
  (type $matrix (list (list u32)))

  (func $roundtrip-variant (param "value" $choice) (result $choice)
    (canon lift (core func $variant-id) (memory $memory) (realloc $realloc) string-encoding=utf8))
  (func $roundtrip-list (param "value" $matrix) (result $matrix)
    (canon lift (core func $list-id) (memory $memory) (realloc $realloc)))

  (instance $api
    (export "choice" (type $choice))
    (export "matrix" (type $matrix))
    (export "roundtrip-variant" (func $roundtrip-variant))
    (export "roundtrip-list" (func $roundtrip-list))
  )
  (export "examples:collections/api@1.0.0" (instance $api))
)
