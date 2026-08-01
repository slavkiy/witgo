(component
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
      i32.const 11
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
  (export "examples:handles/api@1.0.0" (instance $api))
)
