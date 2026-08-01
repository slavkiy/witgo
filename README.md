<p align="center"><img src="assets/art.png" alt="art" width="300"></p>

# witgo

Любой сгенерированный WIT-интерфейс может быть реализован Go-кодом или другим
зарегистрированным WebAssembly Component. Плагин-потребитель использует тот же
import и не знает, какой provider находится за ним.

`witgo` генерирует типизированный Go API из WIT-контракта и позволяет Go-host
загружать WebAssembly Component-плагины, вызывать их exports и предоставлять им
host-функции.

> Статус: beta. Библиотека уже покрывает основной сценарий Component Model,
> строгую проверку контракта до запуска, version handshake с Rust bridge и
> end-to-end тесты для сложных типов. Перед production-использованием всё равно
> стоит прогнать свои контракты и плагины отдельными интеграционными тестами.

## Требования

- Go 1.18 или новее;
- плагин в формате WebAssembly Component (`.wasm`), а не core Wasm module;
- встроенный native bridge уже лежит в модуле для Linux, macOS и Windows на
  `amd64` и `arm64`, отдельная установка при запуске не нужна.

## Установка

```sh
go get github.com/slavkiy/witgo
```

## Быстрый старт

### 1. Опишите контракт в WIT

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

`import host` описывает функции, которые даёт приложение.

`export metadata` описывает функции, которые реализует плагин.

### 2. Сгенерируйте Go package

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

```sh
go run generate.go
```

Будет создан `internal/contract/bindings.gen.go` с типами `Info`, `Host`,
`Plugin`, `PluginImports`, а также helper-функциями `PluginPing`,
`ValidatePlugin`, `CheckPlugin`, `OpenPlugin` и `OpenPluginWithOptions`.

### 3. Реализуйте host-функции

```go
type pluginHost struct{}

func (pluginHost) ProcessString(_ context.Context, value string) (string, error) {
	return "HOST:" + value, nil
}
```

Go-компилятор сам проверит, что реализация соответствует generated interface
`contract.Host`.

### 4. Проверьте контракт и откройте плагин

```go
package main

import (
	"context"
	"fmt"
	"log"

	contract "example.com/myapp/internal/contract"
)

func main() {
	ctx := context.Background()
	report, err := contract.ValidatePluginContext(ctx, "./plugins/plugin.component.wasm")
	if err != nil {
		log.Fatal(err)
	}
	if !report.Compatible {
		log.Fatalf("incompatible plugin: %+v", report)
	}

	plugin, err := contract.OpenPluginContext(ctx, "./plugins/plugin.component.wasm", contract.PluginImports{
		Host: pluginHost{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()

	info, err := plugin.Metadata.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info.Name)
	fmt.Println(info.Version)
	fmt.Println(info.Author)
}
```

Вызовы export-функций остаются типизированными:

```go
info, err := plugin.Metadata.Get(ctx)
```

### 5. Ограничьте выполнение плагина

```go
plugin, err := contract.OpenPluginWithOptionsContext(
	ctx,
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

Ошибка от `CheckPlugin` поддерживает `errors.Is(err, witgo.ErrContractMismatch)`
и `errors.As` к `*witgo.ContractValidationError`.

## Что есть сейчас

- строгий version handshake между Go-библиотекой и Rust bridge;
- проверка контракта до instantiation: imports, exports, structural signatures;
- generated `Ping`-manifest для каждого `world`;
- типизированные bindings для records, enums, flags, variants, options, results,
  tuples, maps и вложенных списков;
- `witgo.Handle` для `resource`, `future`, `stream` и `error-context`;
- end-to-end тесты для maps, variants, resources и сложных списков;
- CI-матрица для Linux, macOS и Windows на `amd64` и `arm64`;
- встроенный native bridge без отдельного sidecar-процесса; обычный Go работает без CGO, TinyGo использует CGo-loader.
- автоматическое связывание вложенных плагинов по WIT imports/exports без ручных adapters;

## Ограничения на сегодня

- `resource`, `future`, `stream` и `error-context` поддерживаются как живые
  runtime-bound handle, включая автоматическую same-Store передачу между
  вложенными WebAssembly-плагинами, но не как полностью типизированный
  high-level API;
- для `future<T>` и `stream<T>` ещё нет generated Go API для чтения/записи typed
  payload;
- tuple с arity больше 16 переходят в динамический `witgo.Tuple`;
- `map<K,V>` поддерживается, но ключ обязан быть `comparable` в Go;
- observability hooks, instance pool и hot reload пока не входят в публичную
  библиотеку.

Полная матрица возможностей и ограничений собрана в
[docs/capabilities.md](docs/capabilities.md).

## Поддерживаемые значения

Runtime покрывает `bool`, все WIT-числа, `char`, `string`, records, lists,
options, results, tuples, maps, enums, flags и variants.

`resource`, `future`, `stream` и `error-context` передаются как `witgo.Handle`.
Такой handle привязан к Store runtime-коробки, его можно вернуть обратно в
Component, передать вложенному WebAssembly provider в той же коробке и явно
закрыть через `Handle.Close`. Между независимыми коробками handle не копируется.

## Пример

```sh
go run ./examples/contracts/basic
go run ./examples/scenarios/server
```

Ожидаемый вывод:

```text
Plugin metadata
Name: HOST:image-resizer
Version: 1.4.0
Author: Example Team
Description: Resizes uploaded images and creates previews.
```

- [WIT-контракт](examples/contracts/basic/wit/plugin.wit)
- [Сгенерированный package](examples/contracts/basic/out/bindings.gen.go)
- [Component plugin](examples/components/basic/component.wasm)
- [Go host](examples/scenarios/server/main.go)

## Как это работает

`witgo` загружает version-matched Wasmtime shared library прямо в Go-процесс.
Нет отдельного дочернего процесса, нет stdin/stdout IPC, нет download шага во
время запуска. Нативная библиотека уже встроена в Go-модуль и при первом
использовании распаковывается в content-addressed cache после SHA-256
проверки.

При инициализации Go и Rust обмениваются `protocol_version`,
`witgo_version`, `bridge_version`, `wasmtime_version` и обязательными
feature-флагами. До запуска `start` bridge отвечает на contract `ping`
отсортированными именами import/export-функций, а generated Go bindings
сравнивают их с ожидаемым контрактом и зарегистрированными host imports.

Подробности:

- [Оглавление документации](docs/README.md)
- [Tutorial: Rust Component + Go host](docs/tutorial-rust-component.md)
- [Архитектура runtime](docs/architecture.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Проверка контрактов](docs/validation.md)
- [Generated code](docs/generated-code.md)
- [Публичный API](docs/public-api.md)
- [TinyGo и контекстный API](docs/tinygo.md)
- [Автоматические вложенные плагины](docs/nested-plugins.md)
- [Прозрачная композиция плагинов](docs/plugin-composition.md)
- [Migration guide](docs/migration-guide.md)
- [Security model](SECURITY.md)
- [Релизный процесс](docs/releasing.md)
- [Список изменений](CHANGELOG.md)

## Проверка

```sh
go test ./...
```
