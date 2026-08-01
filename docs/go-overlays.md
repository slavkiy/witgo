# Go type overlays

Go overlay меняет только публичное представление generated API. `.wit`, contract
signatures и Component Model wire values остаются стандартными.

```yaml
version: 1
types:
  example:users/users@1.0.0#timestamp:
    go_type: time.Time
    import: time
    codec: unix-seconds
errors:
  example:users/users@1.0.0#save:
    result_error: true
```

Canonical ID всегда имеет форму
`namespace:package/interface@version#member`. Если WIT package не имеет version,
часть `@version` отсутствует.

```go
err := witgo.Generate(witgo.Config{
	WIT:       "plugin.wit",
	GoOverlay: "plugin.witgo.yaml",
	Output:    "./contract",
})
```

Встроенные codecs:

- `unix-seconds`, `unix-milliseconds`, `unix-nanoseconds`: `s64` и `time.Time`;
- `duration-nanoseconds`: `s64` и `time.Duration`.

Generated код использует private wire structs и прямые typed lower/lift functions,
без reflection и registry на каждом вызове. `result_error` доступен только для
`result<T,E>` и возвращает `witgo.WITError[E]`; payload извлекается через
`errors.As`.

Сейчас mapping поддержан для aliases и рекурсивных полей records. Mapping внутри
`list`, `option`, `result`, `map`, tuple или variant отклоняется генератором с
явной ошибкой. Custom codec symbols и `uuid-string` пока не реализованы.
