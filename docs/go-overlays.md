# Go type overlays

Overlay меняет публичное представление generated Go API, не изменяя `.wit`,
structural signatures или Component Model ABI. Без `GoOverlay` generation и
runtime behavior полностью совпадают с прежним режимом.

## Формат

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

Canonical ID имеет форму
`namespace:package/interface@version#member`. Для package без version часть
`@version` отсутствует. Короткие имена и формат через точку не принимаются.

```go
err := witgo.Generate(witgo.Config{
	WIT:       "plugin.wit",
	GoOverlay: "plugin.witgo.yaml",
	Output:    "./contract",
})
```

В CLI тот же файл задаётся через `witgen -go-overlay`.

## Public и wire types

Для WIT alias `type timestamp = s64` mapping на `time.Time` создаёт публичный
alias и прямые adapters. Если alias входит в record, generator создаёт private
wire struct:

```go
type Timestamp = time.Time

type User struct {
	CreatedAt time.Time `json:"created-at"`
}

type userWire struct {
	CreatedAt int64 `json:"created-at"`
}
```

Component client, Go host import и registered provider используют одни и те же
generated lower/lift functions. Значение `time.Time` никогда не отправляется в
bridge через его стандартный JSON marshal; bridge получает канонический `s64`.

## Codecs

Встроены:

- `unix-seconds`: WIT `s64` и `time.Time`;
- `unix-milliseconds`: WIT `s64` и `time.Time`;
- `unix-nanoseconds`: WIT `s64` и `time.Time`;
- `duration-nanoseconds`: WIT `s64` и `time.Duration`.

`TypeCodec[Wire, Go]` и `TypeCodecFuncs` доступны для typed codec tooling.
Generated built-in codecs являются прямыми функциями: reflection и generic
registry на каждом вызове не используются. Custom symbols и `uuid-string` пока
не подключаются из YAML.

## Result как Go error

`result_error: true` разрешён только для функции с `result<T,E>`:

- `result<_,E>` становится `error`;
- `result<T,E>` становится `(T, error)`.

Transport/runtime error возвращается как есть. WIT error возвращается как
`witgo.WITError[E]`, поэтому structured payload сохраняется:

```go
var witErr witgo.WITError[SaveError]
if errors.As(err, &witErr) {
	log.Print(witErr.Value)
}
```

Go provider может вернуть такой же, в том числе обёрнутый, `WITError[E]`; adapter
преобразует его обратно в WIT `err`. Обычный Go error остаётся transport error.

## Validation и детерминированность

До rendering проверяются schema version, canonical ID, наличие WIT alias,
синтаксис и безопасность Go type expression, import qualifier, codec name и
совместимость wire type. Pointer, channel, function, interface,
`context.Context`, `unsafe.Pointer` и произвольный Go object не допускаются.

Ошибка содержит путь overlay, canonical ID и конкретную причину. Imports и WIT
sources сортируются; одинаковые WIT и overlay дают byte-identical output.

## Текущая граница

Поддержаны mapped aliases и их рекурсивное использование в records. Mapping
внутри `list`, `option`, `result`, `map`, tuple или variant пока отклоняется до
rendering с явной ошибкой. Это ограничение overlay adapters, а не ограничение
обычного generated API: без overlay эти WIT containers поддерживаются полностью.
