<p align="center"><img src="assets/art.png" alt="witgo" width="300"></p>

`witgo` генерирует типизированную Go-библиотеку из WIT-контракта. Сервер
работает с обычными Go-моделями и интерфейсами, а загрузка Wasm, имена экспортов
и чтение памяти остаются внутри generated-кода.

Минимальная версия Go: **1.24**.

## Установка

```sh
go get github.com/slavkiy/witgo
```

## Быстрый старт

Контракт `wit/types.wit`:

```wit
package examples:contract@1.0.0;

interface types {
    record plugin-metadata {
        name: string,
        version: string,
        author: string,
        description: string,
    }
}
```

Контракт `wit/plugin.wit`:

```wit
package examples:contract@1.0.0;

interface plugin-info {
    use types.{plugin-metadata};

    metadata: func() -> plugin-metadata;
}

world plugin {
    export plugin-info;
}
```

Запуск генератора:

```go
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

```sh
go run ./cmd/generate
```

В `internal/contract/bindings.gen.go` появится готовая библиотека:

```go
type PluginMetadata struct {
	Name        string
	Version     string
	Author      string
	Description string
}

type PluginInfo interface {
	Metadata() (PluginMetadata, error)
}

func OpenPlugin(filename string) (*Plugin, error)

func (p *Plugin) Metadata() (PluginMetadata, error)
```

Generated-файл не нужно редактировать вручную.

Подробное описание всех generated-типов, constructors, imports, методов моделей
и тестирования находится в
[документации generated-кода](docs/generated-code.md).

### Диагностика генерации

Если нужен красивый локализованный вывод ошибок, используйте отдельный
generation-only CLI:

```sh
cd cmd/witgen
go run . \
  -wit ../../examples/generate/wit \
  -out ../../examples/generate/out \
  -package contract \
  -lang ru
```

CLI использует [digreyt](https://github.com/slavkiy/digreyt) только при запуске
генерации. `digreyt` не вшивается в `witgo`, generated-файл или Wasm runtime.
Автоперевод применяется только к ошибкам; для полностью офлайн-запуска
передайте `-auto-translate=false`.

CLI вынесен в отдельный Go-модуль, потому что текущая версия `digreyt` требует
Go 1.25.6. Основная библиотека и generated-код продолжают работать на Go 1.24.

## Использование Wasm-плагина

Сервер импортирует только generated package:

```go
package main

import (
	"fmt"
	"log"

	contract "example.com/project/internal/contract"
)

func main() {
	plugin, err := contract.OpenPlugin("./plugins/plugin.wasm")
	if err != nil {
		log.Fatal(err)
	}

	metadata, err := plugin.Metadata()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Name:", metadata.Name)
	fmt.Println("Version:", metadata.Version)
	fmt.Println("Author:", metadata.Author)
	fmt.Println("Description:", metadata.Description)
}
```

Пользовательский код не вызывает `wasmtime`, не ищет экспортированные функции
и не читает память Wasm самостоятельно.

## Что генерируется

| WIT | Generated Go |
| --- | --- |
| `package examples:contract@1.0.0` | package metadata constants |
| `record plugin-metadata` | `PluginMetadata` struct |
| импортируемый interface | Go interface, который реализует host |
| экспортируемый interface | Go interface и методы world |
| `world plugin` | `Plugin`, `OpenPlugin` |
| `metadata: func()` | `Metadata() (..., error)` |

Package declaration сохраняется в generated-коде:

```go
const (
	WITPackageNamespace = "examples"
	WITPackageName      = "contract"
	WITPackageVersion   = "1.0.0"
	WITPackageID        = "examples:contract@1.0.0"
)
```

Runtime-имя экспортированной функции строится из полного WIT ID:

```text
examples:contract/plugin-info@1.0.0#metadata
```

Если exported-функция принимает модель первым аргументом, generated-код может
добавить к модели удобный метод. Например:

```wit
save-user: func(user: user) -> bool;
```

создаёт:

```go
saved, err := user.Save()
```

Связь модели с plugin хранится в приватном поле и не видна пользователю.

## Config

```go
type Config struct {
	WIT      string
	Output   string
	Package  string
	Filename string
}
```

| Поле | Описание |
| --- | --- |
| `WIT` | Путь к `.wit`-файлу или каталогу. Каталог обходится рекурсивно |
| `Output` | Каталог для generated-кода. По умолчанию текущий каталог |
| `Package` | Имя Go package. Если пустое, берётся из WIT package |
| `Filename` | Имя файла. По умолчанию `bindings.gen.go` |

Все найденные WIT-файлы должны относиться к одному package и иметь одинаковую
версию. Запись generated-файла выполняется атомарно после `go/format`.

Для повторного использования конфигурации:

```go
generator, err := witgo.NewGenerator(config)
if err != nil {
	return err
}
return generator.Generate()
```

## Host imports

WIT import превращается в интерфейс, который должен реализовать сервер:

```wit
interface host {
    current-user: func() -> user;
}

world plugin {
    import host;
    export plugin-info;
}
```

Generated-конструктор принимает реализацию напрямую:

```go
type host struct{}

func (host) CurrentUser() contract.User {
	return contract.User{Name: "Server user"}
}

plugin, err := contract.OpenPlugin("plugin.wasm", host{})
```

Go-реализация import хранится внутри модели world. Фактическое связывание
component-model host imports пока ограничено возможностями `wasmtime-go/v47`.

## Core Wasm ABI

Текущий runtime вызывает **core WebAssembly modules**. Экспорт должен называться
полным WIT-именем:

```wat
(func
  (export "examples:contract/plugin-info@1.0.0#metadata")
  (result i64)
  ...
)
```

Для record-результата пример использует простой закрытый ABI:

- record кодируется как JSON в экспортированной памяти `memory`;
- функция возвращает `i64`;
- младшие 32 бита содержат offset;
- старшие 32 бита содержат длину JSON.

Generated library сама читает память и декодирует JSON в Go-модель. Пользователь
generated package этого уровня не видит.

Полная поддержка стандартного WIT Component Model потребует поддержки вызова
component exports и регистрации host functions со стороны `wasmtime-go`.

## Примеры

| Пример | Что показывает | Команда |
| --- | --- | --- |
| [generate](examples/generate) | Генерация Go package из нескольких WIT-файлов | `go run ./examples/generate` |
| [generation-errors](examples/generation-errors) | Локализованная ошибка парсинга через optional `witgen` | `cd cmd/witgen && go run . -wit ../../examples/generation-errors/invalid.wit -out /tmp/witgo-invalid -lang ru -auto-translate=false` |
| [plugin](examples/plugin) | Сборка настоящего core Wasm из WAT | `go run ./examples/plugin` |
| [server](examples/server) | Загрузка `.wasm` и типизированный вызов `Metadata()` | `go run ./examples/server` |

### Wasm plugin

Полный цикл генерации, сборки плагина и запуска сервера:

```sh
go run ./examples/generate
go run ./examples/plugin
go run ./examples/server
```

Ожидаемый результат:

```text
Plugin metadata
Name: image-resizer
Version: 1.4.0
Author: Example Team
Description: Resizes uploaded images and creates previews.
```

Полезные исходники:

- [WIT-контракт](examples/generate/wit)
- [generated library](examples/generate/out/bindings.gen.go)
- [Wasm-плагин](examples/plugin/plugin.wat)
- [сервер](examples/server/main.go)

## Ограничения

- Component Model exports пока нельзя вызвать через используемый
  `wasmtime-go/v47`.
- Component host imports пока не связываются с Wasm.
- Core Wasm record ABI сейчас основан на JSON и packed `i64`.
- Generated-файл рассчитан на Go 1.24 и новее.

## Проверка

```sh
go test ./...
```

Проект и generated-примеры проверяются на Go 1.24.
