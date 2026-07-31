# witgo

`witgo` генерирует типизированный Go API из WIT и запускает плагины как
стандартные WebAssembly Components. Граница plugin ↔ host использует Component
Model и Canonical ABI; старый JSON/packed-`i64` ABI удалён.

Минимальная версия — Go 1.18. В Go-проекте нет CGO и зависимости от
`wasmtime-go`: официальный Wasmtime работает в отдельном Rust bridge. Готовый
bridge выбирается из `go:embed` по `GOOS/GOARCH` и извлекается в кэш при первом
запуске.

## Нормальный сценарий

Контракт:

```wit
package test:metadata@1.0.0;

interface metadata {
    record info {
        name: string,
        version: string,
        description: string,
        author: string,
        license: string,
    }

    get: func() -> info;
}

interface host {
    process-string: func(value: string) -> string;
}

world plugin {
    import host;
    export metadata;
}
```

После генерации host реализует обычный Go interface:

```go
type host struct{}

func (host) ProcessString(value string) string {
	return "HOST:" + value
}

plugin, err := contract.OpenPlugin("plugin.component.wasm", host{})
if err != nil {
	log.Fatal(err)
}
defer plugin.Close()

info, err := plugin.Metadata.Get()
```

Generated-код сам:

- регистрирует `test:metadata/host@1.0.0#process-string` до instantiation;
- преобразует аргументы host call в типизированные Go-значения;
- вызывает export как `plugin.Metadata.Get()`, то есть метод принадлежит модели
  WIT interface, а не свален в корневой `Plugin`;
- поднимает WIT record прямо в Go struct без чтения linear memory приложением.

Компонент не получает filesystem, network или другие возможности автоматически.
Он видит только imports, перечисленные его `world` и переданные generated
конструктору.

## Генерация

Из Go:

```go
err := witgo.Generate(witgo.Config{
	WIT:    "./wit",
	Output: "./internal/contract",
})
```

Или примером из репозитория:

```powershell
go run ./examples/generate
go run ./examples/server
```

Ожидаемый результат:

```text
Plugin metadata
Name: HOST:image-resizer
Version: 1.4.0
Author: Example Team
Description: Resizes uploaded images and creates previews.
```

[Пример Component WAT](examples/plugin/plugin.wat) действительно вызывает
`host.process-string`; это не эмуляция вызова на стороне Go.

## Лимиты runtime

```go
plugin, err := contract.OpenPluginWithOptions(
	"plugin.component.wasm",
	witgo.RuntimeOptions{
		FuelPerCall:      1_000_000,
		Timeout:          2 * time.Second,
		MemoryLimitBytes: 64 << 20,
		MaxResultBytes:   1 << 20,
		InstanceLimit:    8,
	},
	host{},
)
```

- `FuelPerCall` выдаёт новый budget каждому export call.
- `Fuel` задаёт один общий budget на весь runtime. Вместе с `FuelPerCall` его
  задавать нельзя.
- `Timeout` использует epoch interruption Wasmtime.
- `MemoryLimitBytes` ограничивает каждую linear memory.
- `InstanceLimit` ограничивает instances в Store.
- `MaxResultBytes` ограничивает сообщения между Go и bridge.

Fuel и timeout распознаются через `errors.Is(err, witgo.ErrFuelExhausted)` и
`errors.Is(err, witgo.ErrCallTimeout)`.

Это не абсолютная sandbox-модель. Epoch interruption не может прервать зависший
Go host callback; host-память и рекурсия host calls требуют ограничений самого
приложения. Filesystem/network безопасны только пока host явно не выдаёт такие
capabilities.

## Embedded bridge

Runtime ищет bridge в таком порядке:

1. `RuntimeOptions.BridgePath`;
2. `WITGO_COMPONENT_BRIDGE`;
3. embedded binary для текущих `GOOS/GOARCH`;
4. `witgo-component-host` в `PATH`;
5. локальный `bridge/target/{release,debug}` для разработки.

Поддерживаемый release matrix:

- Windows amd64/arm64;
- Linux amd64/arm64;
- macOS amd64/arm64.

Workflow [.github/workflows/bridge-binaries.yml](.github/workflows/bridge-binaries.yml)
собирает шесть native binaries, сжимает их и создаёт source bundle, в котором
они уже доступны `go:embed`. Локальная сборка bridge:

```powershell
cd bridge
cargo build --locked --release
```

## Низкоуровневый API

```go
runtime, err := witgo.LoadRuntimeWithImports(
	"plugin.component.wasm",
	witgo.RuntimeOptions{},
	[]witgo.HostImport{{
		Interface: "test:metadata/host@1.0.0",
		Function:  "process-string",
		Call: func(args []any) (any, error) {
			return strings.ToUpper(args[0].(string)), nil
		},
	}},
)
defer runtime.Close()

value, err := runtime.Call("test:metadata/metadata@1.0.0#get")
```

Для прикладного кода предпочтительнее generated API. Core Wasm modules теперь
отклоняются с `ErrCoreModule`: плагин должен быть Component binary. Component
WAT принимается только как удобный формат разработки и тестов.

Сейчас динамический bridge поддерживает числа, `bool`, `string`, records,
lists и options. Handles/resources, futures, streams и error-context пока
возвращают явную ошибку. Генератор парсит больше WIT-конструкций, но их runtime
mapping нужно расширять отдельно и тестировать end-to-end.

## Проверка

```powershell
go test ./...
cd bridge
cargo check --locked
```
