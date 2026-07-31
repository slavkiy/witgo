<p align="center"><img src="assets/art.png" alt="art" width="300"></p>

`witgo` генерирует типизированный Go API из WIT-контракта и позволяет Go-host
загружать WebAssembly Component плагины, вызывать их exports и предоставлять им
host-функции.

> Статус: beta. Основной сценарий `string` + числа + records + lists + options
> работает и покрыт end-to-end тестом. Перед production-релизом проверяйте свой
> конкретный WIT-контракт тестами.

## Требования

- Go 1.18 или новее;
- плагин в формате WebAssembly Component (`.wasm`), а не core Wasm module;
- tagged module содержит native shared library для Windows, Linux и macOS на
  amd64/arm64; отдельная установка и download при запуске не нужны.

## Установка

```powershell
go get github.com/slavkiy/witgo
```

## 1. Создайте WIT-контракт

Например, `wit/plugin.wit`:

```wit
package example:plugins@1.0.0;

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

`import host` - функции, которые предоставляет приложение.

`export metadata` - функции, которые реализует плагин.

## 2. Сгенерируйте Go package

Создайте `generate.go`:

```go
//go:build ignore

package main

import (
	"log"

	"github.com/slavkiy/witgo"
)

func main() {
	err := witgo.Generate(witgo.Config{
		WIT:     "./wit",
		Output:  "./internal/contract",
		Package: "contract",
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Запустите:

```powershell
go run generate.go
```

Будет создан `internal/contract/bindings.gen.go` с типами `Info`, `Host`,
`Plugin` и конструкторами `OpenPlugin`/`OpenPluginWithOptions`.

## 3. Реализуйте функции host

```go
type pluginHost struct{}

func (pluginHost) ProcessString(value string) string {
	return "HOST:" + value
}
```

Сигнатуру проверяет компилятор Go: реализация должна соответствовать generated
interface `contract.Host`.

## 4. Откройте плагин

```go
package main

import (
	"fmt"
	"log"

	contract "example.com/myapp/internal/contract"
)

func main() {
	report, err := contract.ValidatePlugin("./plugins/plugin.component.wasm")
	if err != nil {
		log.Fatal(err)
	}
	if !report.Compatible {
		log.Fatalf("incompatible plugin: %+v", report)
	}

	plugin, err := contract.OpenPlugin("./plugins/plugin.component.wasm", contract.PluginImports{
		Host: pluginHost{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()

	info, err := plugin.Metadata.Get()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info.Name)
	fmt.Println(info.Version)
	fmt.Println(info.Author)
}
```

Вызов делается через interface, которому принадлежит функция:

```go
info, err := plugin.Metadata.Get()
```

## 5. Ограничьте плагин

```go
plugin, err := contract.OpenPluginWithOptions(
	"./plugins/plugin.component.wasm",
	witgo.RuntimeOptions{
		FuelPerCall:      1_000_000,
		Timeout:          2 * time.Second,
		MemoryLimitBytes: 64 << 20,
		MaxResultBytes:   1 << 20,
		InstanceLimit:    8,
	},
	contract.PluginImports{Host: pluginHost{}},
)
```

Если подробный отчёт не нужен, используйте короткую проверку:

```go
if err := contract.CheckPlugin("./plugins/plugin.component.wasm"); err != nil {
	log.Fatal(err)
}
```

Ошибка от `CheckPlugin` поддерживает `errors.Is(err,
witgo.ErrContractMismatch)` и `errors.As` в `*witgo.ContractValidationError`.
Подробное руководство: [валидация компонентов](docs/validation.md).

| Поле | Назначение |
| --- | --- |
| `FuelPerCall` | Новый вычислительный бюджет для каждого вызова |
| `Fuel` | Общий бюджет на всё время жизни плагина |
| `Timeout` | Максимальное время выполнения Wasm-вызова |
| `MemoryLimitBytes` | Лимит каждой linear memory |
| `MaxResultBytes` | Максимальный объём передаваемого результата |
| `InstanceLimit` | Максимальное количество Wasm instances |

`Fuel` и `FuelPerCall` нельзя задавать одновременно.

```go
info, err := plugin.Metadata.Get()
switch {
case errors.Is(err, witgo.ErrFuelExhausted):
	log.Println("plugin exhausted its fuel")
case errors.Is(err, witgo.ErrCallTimeout):
	log.Println("plugin timed out")
case err != nil:
	log.Println("plugin failed:", err)
default:
	fmt.Println(info.Name)
}
```

## Host capabilities

Плагин получает только imports из WIT world, реализации которых переданы в
`OpenPlugin`. Не добавляйте filesystem или network imports, если плагину они не
нужны.

Timeout прерывает выполнение Wasm, но не зависшую реализацию Go host-функции.
Host-функции должны самостоятельно соблюдать timeout и лимиты.

## Поддерживаемые значения

Runtime проверен для `bool`, чисел, `char`, `string`, records, nested lists,
type-safe options/results, tuples, enums, flags и variants. Resources, futures,
streams и `error-context` передаются как
привязанные к Runtime `witgo.Handle`: их можно вернуть в Component и явно
закрыть через `Handle.Close`. Чтение payload future/stream из Go пока не входит
в dynamic handle API.

CI собирает Go и Rust bridge на Linux, macOS и Windows для amd64 и arm64. E2E
проверяет version handshake, variants, nested lists и resource-контракты.

Полная матрица parser/generator/runtime, эксплуатационные гарантии и известные
ограничения собраны в [документе возможностей](docs/capabilities.md).

## Рабочий пример

```powershell
go run ./examples/generate
go run ./examples/server
```

Ожидаемый вывод:

```text
Plugin metadata
Name: HOST:image-resizer
Version: 1.4.0
Author: Example Team
Description: Resizes uploaded images and creates previews.
```

- [WIT-контракт](examples/generate/wit/plugin.wit)
- [Generated package](examples/generate/out/bindings.gen.go)
- [Component plugin](examples/plugin/plugin.wat)
- [Go host](examples/server/main.go)

## How it works

`witgo` loads a version-matched Wasmtime DLL/shared library in the Go process.
There is no child executable, stdin/stdout IPC, separate installation or
runtime download. The current platform library is already embedded in the Go
module and is materialized into a content-addressed cache after SHA-256
verification. Go calls its stable C ABI without CGO.

Every release contains Linux, macOS and Windows libraries for amd64 and arm64,
raw and packaged SHA-256 checksums, an SPDX SBOM, and GitHub build-provenance
attestations. Cache installation and concurrent calls are handled explicitly.
The complete loading order, in-process protocol,
supported WIT types and lifecycle guarantees are documented in
[Runtime architecture](docs/architecture.md); reproducible build and artifact
verification commands are in [Releasing](docs/releasing.md).
Release changes are listed in [CHANGELOG.md](CHANGELOG.md).

## Проверка

```powershell
go test ./...
```

Описание API: [docs/public-api.md](docs/public-api.md).
