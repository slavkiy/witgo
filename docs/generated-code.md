# Generated Go code

WIT world превращается в Go client с typed imports и exports.

```wit
interface host {
    process-string: func(value: string) -> string;
}

interface metadata {
    record info { name: string, version: string }
    get: func() -> info;
}

world plugin {
    import host;
    export metadata;
}
```

Основная форма generated API:

```go
type Host interface {
	ProcessString(value string) string
}

type Metadata interface {
	Get() (Info, error)
}

type Plugin struct {
	Metadata Metadata
}

func OpenPlugin(filename string, host Host) (*Plugin, error)
func OpenPluginWithOptions(filename string, options witgo.RuntimeOptions, host Host) (*Plugin, error)
func (p *Plugin) Close() error
```

Export вызывается удобно и соответствует владельцу из WIT:

```go
info, err := plugin.Metadata.Get()
```

Generated constructor создаёт `witgo.HostImport` adapters до instantiation.
Adapter проверяет число аргументов, поднимает каждое значение в Go type и
вызывает реализацию `Host`. Nil import отклоняется сразу.

Records передаются по Canonical ABI движком Wasmtime. Внутренний JSON-канал
между Go и отдельным bridge процессом является implementation detail и не
задаёт ABI плагина. В generated-коде больше нет `ReadMemory`, offset/length и
packed `i64`.

Ошибки export call возвращаются напрямую. Ошибка преобразования результата
оборачивается с полным WIT именем функции.

Generated package рассчитан на Go 1.18+. Файл рекомендуется хранить в Git,
чтобы изменения публичного API были видны в review.
