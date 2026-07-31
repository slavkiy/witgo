# Использование generated-кода

Этот документ описывает API, который создаёт `witgo.Generate`. Generated package
должен быть единственной точкой контакта приложения с плагином: сервер работает
с Go-интерфейсами и моделями и не вызывает Wasm runtime напрямую.

## Общая схема

```text
WIT contract
     │
     ▼
witgo.Generate
     │
     ▼
generated Go package
     │
     ▼
OpenWorld(...) ──► plugin.wasm
```

Generated package открывает только настоящий Wasm-плагин.

## Package metadata

Для объявления:

```wit
package examples:contract@1.0.0;
```

генерируются:

```go
const (
	WITPackageNamespace = "examples"
	WITPackageName      = "contract"
	WITPackageVersion   = "1.0.0"
	WITPackageID        = "examples:contract@1.0.0"
)
```

Эти значения можно использовать в логах, диагностике и проверке совместимости
плагинов.

Если `Config.Package` не задан, имя Go package берётся из WIT package name.

## Records

WIT:

```wit
record plugin-metadata {
    name: string,
    version: string,
}
```

Go:

```go
type PluginMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
```

Generated record является обычной Go-моделью. Его можно передавать между
пакетами, сериализовать и использовать в прикладном коде.

## Интерфейсы exports

WIT:

```wit
interface plugin-info {
    metadata: func() -> plugin-metadata;
}

world plugin {
    export plugin-info;
}
```

Go:

```go
type PluginInfo interface {
	Metadata() (PluginMetadata, error)
}

type Plugin struct {
	// private generated fields
}

func (p *Plugin) Metadata() (PluginMetadata, error)
```

К каждому exported-вызову добавляется `error`, потому что загрузка плагина,
вызов runtime и декодирование результата могут завершиться ошибкой.

## Загрузка Wasm

Для загрузки файла используется `Open<World>`:

```go
plugin, err := contract.OpenPlugin("./plugins/plugin.wasm")
if err != nil {
	return err
}

metadata, err := plugin.PluginInfo.Metadata()
```

Каждый экспортированный WIT-интерфейс становится полем world-клиента. Поэтому
методы сохраняют принадлежность из контракта: `metadata.get` вызывается как
`plugin.Metadata.Get()`, а не как `plugin.Get()`. Функции, объявленные напрямую
внутри `world`, остаются методами самого world-клиента.

Чтобы ограничить выполнение плагина, generated package также создаёт
`Open<World>WithOptions`:

```go
plugin, err := contract.OpenPluginWithOptions(
	"./plugins/plugin.wasm",
	witgo.RuntimeOptions{Fuel: 1_000_000},
)
```

Fuel расходуется всеми вызовами world совместно. Исчерпание определяется через
`errors.Is(err, witgo.ErrFuelExhausted)`.

Прикладному коду достаточно пути к `.wasm` и методов generated world.

## Host imports

WIT import описывает функции, которые plugin ожидает от сервера:

```wit
interface host {
    current-user: func() -> user;
}

world plugin {
    import host;
    export plugin-api;
}
```

Generated package создаёт интерфейс:

```go
type Host interface {
	CurrentUser() User
}
```

Сервер реализует его:

```go
type host struct {
	current User
}

func (h host) CurrentUser() contract.User {
	return contract.User{
		Name: h.current.Name,
	}
}
```

Реализация передаётся напрямую в конструктор:

```go
plugin, err := contract.OpenPlugin("plugin.wasm", host{current: user})
```

Generated world хранит imports в приватных полях. Поддержка передачи imports в
настоящий Wasm component сейчас ограничена API `wasmtime-go/v47`.

## Методы моделей

Если exported-функция первым аргументом принимает record:

```wit
save-task: func(task: task) -> bool;
```

generated-код связывает модель с world и добавляет удобный метод:

```go
task, err := plugin.FindTask(10)
task.Done = true

saved, err := task.Save()
```

Внутри `Task` хранится приватный generated-интерфейс. Он присваивается модели,
когда модель возвращается методом world.

Модель, созданная вручную, ещё не привязана к plugin:

```go
task := contract.Task{ID: 10}
_, err := task.Save() // model is not attached to a plugin
```

Для ручной модели используйте метод world:

```go
saved, err := plugin.SaveTask(task)
```

## Enums

WIT:

```wit
enum status {
    pending,
    running,
    complete,
}
```

Go:

```go
type Status uint32

const (
	StatusPending  Status = 0
	StatusRunning  Status = 1
	StatusComplete Status = 2
)
```

## Flags

WIT:

```wit
flags permissions {
    read,
    write,
}
```

Go:

```go
type Permissions uint64

const (
	PermissionsRead  Permissions = 1 << 0
	PermissionsWrite Permissions = 1 << 1
)
```

Флаги объединяются обычным побитовым `OR`:

```go
permissions := contract.PermissionsRead | contract.PermissionsWrite
```

## Option, list и tuple

Основные преобразования типов:

| WIT | Go |
| --- | --- |
| `string` | `string` |
| `bool` | `bool` |
| `u8` ... `u64` | `uint8` ... `uint64` |
| `s8` ... `s64` | `int8` ... `int64` |
| `f32`, `f64` | `float32`, `float64` |
| `option<T>` | `*T` |
| `list<T>` | `[]T` |
| `map<K, V>` | `map[K]V` |
| `tuple<A, B>` | anonymous struct with `V0`, `V1` |

Пример optional:

```go
email := "user@example.com"
user := contract.User{Email: &email}
```

## Несколько WIT-файлов

`Config.WIT` может указывать на каталог:

```go
err := witgo.Generate(witgo.Config{
	WIT:    "./wit",
	Output: "./internal/contract",
})
```

Генератор рекурсивно читает `.wit`, сортирует файлы и объединяет их IR. Package
declaration и версия во всех файлах должны совпадать.

Тип можно вынести в отдельный interface:

```wit
interface types {
    record user {
        name: string,
    }
}

interface users {
    use types.{user};
    find: func() -> user;
}
```

`use types.{user};` делает общий WIT-тип доступным внутри interface `users`.

## Обработка ошибок

Ошибку нужно проверять при открытии world и при каждом exported-вызове:

```go
plugin, err := contract.OpenPlugin(filename)
if err != nil {
	return fmt.Errorf("open plugin: %w", err)
}

metadata, err := plugin.PluginInfo.Metadata()
if err != nil {
	return fmt.Errorf("read plugin metadata: %w", err)
}
```

Типичные ошибки:

- файл Wasm не найден;
- бинарник не является WebAssembly;
- нужный export отсутствует;
- тип результата не совпадает с контрактом;
- record JSON повреждён;
- метод вызван у модели, не привязанной к plugin.

## Тестирование приложения

Для integration-теста положите небольшой `.wasm` в `testdata` и открывайте его
через тот же публичный API:

```go
func TestMetadata(t *testing.T) {
	plugin, err := contract.OpenPlugin("testdata/plugin.wasm")
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := plugin.PluginInfo.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != "test-plugin" {
		t.Fatalf("name = %q", metadata.Name)
	}
}
```

Так тест проверяет тот же путь, который используется в production.

## Регенерация

Generated-файл имеет заголовок:

```go
// Code generated by witgo. DO NOT EDIT.
```

После изменения WIT повторно запустите генератор:

```sh
go run ./examples/generate
go test ./...
```

Для локализованных ошибок можно запускать optional CLI:

```sh
cd cmd/witgen
go run . \
  -wit ../../examples/generate/wit \
  -out ../../examples/generate/out \
  -package contract \
  -lang ru
```

CLI выводит локальные английские и русские сообщения без сетевого перевода.
`-auto-translate` сохранён как устаревший no-op для совместимости.

Рекомендуется хранить generated-файл в Git, чтобы изменения публичного API были
видны в code review.

## Текущие ограничения Wasm

Рабочий пример использует core Wasm ABI:

- экспорт имеет полное WIT-имя;
- record возвращается как JSON в памяти;
- `i64` содержит offset в младших 32 битах и длину в старших 32 битах.

Generated package скрывает это соглашение от приложения. Стандартный WIT
Component Model будет доступен после появления нужных component APIs в
используемой версии `wasmtime-go`.
