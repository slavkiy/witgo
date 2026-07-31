# witgo

`witgo` генерирует типизированный Go API из WIT-контракта и позволяет Go-host
загружать WebAssembly Component плагины, вызывать их exports и предоставлять им
host-функции.

> Статус: beta. Основной сценарий `string` + числа + records + lists + options
> работает и покрыт end-to-end тестом. Перед production-релизом проверяйте свой
> конкретный WIT-контракт тестами.

## Требования

- Go 1.18 или новее;
- плагин в формате WebAssembly Component (`.wasm`), а не core Wasm module;
- Windows amd64 работает из текущего репозитория без установки отдельного
  runtime. Остальные платформенные файлы формируются release-сборкой.

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
	plugin, err := contract.OpenPlugin("./plugins/plugin.component.wasm", pluginHost{})
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
	pluginHost{},
)
```

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

Runtime проверен для `bool`, чисел, `string`, records, lists и options.
Resources/handles, futures, streams и `error-context` пока не поддерживаются.

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

## Проверка

```powershell
go test ./...
```

Описание API: [docs/public-api.md](docs/public-api.md).
