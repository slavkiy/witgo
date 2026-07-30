(module
  (memory (export "memory") 1)

  ;; JSON encoded plugin-metadata record at offset 0, length 128.
  (data (i32.const 0)
    "{\"name\":\"image-resizer\",\"version\":\"1.4.0\",\"author\":\"Example Team\",\"description\":\"Resizes uploaded images and creates previews.\"}"
  )

  ;; Packed string result: low 32 bits are pointer, high 32 bits are length.
  (func (export "examples:contract/plugin-info@1.0.0#metadata") (result i64)
    i64.const 549755813888
  )
)
