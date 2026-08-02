<p align="center"><img src="assets/art.png" alt="art" width="300"></p>

`witgo` генерирует типизированный Go API из WIT-контракта для обеих сторон
Component Model. Go-host может загружать WebAssembly Component-плагины,
вызывать их exports и предоставлять host-функции, а guest-режим позволяет
реализовать и собрать сам плагин на Go/TinyGo. Один и тот же WIT import может
обслуживаться Go-кодом или другим зарегистрированным WebAssembly Component без
изменения consumer-кода.

> Статус: beta. Библиотека уже покрывает основной сценарий Component Model,
> строгую проверку контракта до запуска, version handshake с Rust bridge и
> end-to-end тесты для сложных типов. Перед production-использованием всё равно
> стоит прогнать свои контракты и плагины отдельными интеграционными тестами.

## Требования

- Go 1.18 или новее;
- плагин в формате WebAssembly Component (`.wasm`), а не core Wasm module;
- встроенный native bridge уже лежит в модуле для Linux, macOS и Windows на
  `amd64` и `arm64`, отдельная установка при запуске не нужна.

Для написания Go guest-плагинов дополнительно нужны `wit-bindgen-go`, пакет
`go.bytecodealliance.org/cm` и TinyGo с target `wasip2`. Они не нужны обычному
host-приложению.

Parser, generator и generated packages также компилируются на остальных Go
targets. Запуск Component зависит от native loader и Rust/Wasmtime bridge для
выбранной платформы.

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
	err := witgo.GeneratePackage(witgo.Config{
		Output:  "./internal/contract",
		Package: "contract",
	}, "./wit")
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

Это host-режим по умолчанию. Для кода самого плагина задайте
`Mode: witgo.GenerateGuest` и `World: "plugin"`. В таком package будут
`MetadataGuest`, `PluginGuest`, `ExportPlugin` и `Imports.Host`, но не будет
host-only функций `OpenPlugin`, `ValidatePlugin` и composition API. Полный
пример находится в [руководстве по Go guest-плагинам](docs/go-guest-plugins.md).

Для одного файла используйте `GenerateFile`, для явно выбранного набора файлов
одного package - `GenerateFiles`, для рекурсивного дерева одного package -
`GenerateTree`. Старый `Generate(Config{WIT: ...})` сохранён для совместимости.

### Необязательные Go-типы

Стандартный WIT можно дополнить отдельным `plugin.witgo.yaml`. Например, для
`package example:users@1.0.0` и alias `timestamp` в interface `users` WIT `s64`
может выглядеть в публичном Go API как `time.Time`, оставаясь `s64` в Component
ABI:

```yaml
version: 1
types:
  example:users/users@1.0.0#timestamp:
    go_type: time.Time
    import: time
    codec: unix-seconds
```

Overlay подключается через `Config.GoOverlay`; без него generated output и
runtime behavior остаются прежними. Полный формат описан в
[docs/go-overlays.md](docs/go-overlays.md).

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

### 5. Задайте права и топливо на хосте

Для нескольких плагинов удобнее один раз описать host policy. Нулевая policy
безопасна: она запрещает ambient capabilities и загрузку зависимостей.

```go
policy := witgo.HostPolicy{
	Public: witgo.PluginGrant{
		// Эти разрешения и лимиты получит каждый плагин.
		Permissions: witgo.Permissions{
			System: true,
			Allow: []string{"example:plugins/host@1.0.0"},
		},
		Limits: witgo.PluginLimits{
			FuelPerCall:      1_000_000,
			Timeout:          2 * time.Second,
			MemoryLimitBytes: 64 << 20,
		},
	},
	Plugins: map[string]witgo.PluginGrant{
		"downloader": {
			Permissions: witgo.Permissions{Network: true},
		},
		"orchestrator": {
			Permissions: witgo.Permissions{LoadPlugin: true},
			AllowedPluginRoots: []string{"./plugins"},
		},
	},
}

plugin, err := contract.OpenPluginWithPolicyContext(
	ctx, policy, "downloader", "./plugins/downloader.wasm",
	contract.PluginImports{Host: pluginHost{}},
)
```

`System`, `Network` и `Files` разрешают соответствующие WASI namespaces,
`Allow` принимает точные WIT interface/function patterns, а `Deny` всегда
имеет приоритет. `LoadPlugin` разрешает только зависимости из plugin manifest
и ограничивается host-owned `AllowedPluginRoots`.

Если plugin умеет запрашивать дополнительное топливо через opt-in runtime API,
хост также задаёт `FuelRequests` и `FuelPolicy`; сам plugin не может назначить
себе fuel или расширить права.

Низкоуровневый вариант через `RuntimeOptions` тоже сохранён:

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

### 6. Сгенерируйте и соберите Go-плагин

Один вызов может сначала создать guest bindings из того же WIT-контракта, а
затем собрать готовый Component и встроить manifest зависимостей:

```go
err := witgo.BuildPlugin(witgo.PluginBuildConfig{
	Generate: witgo.Config{
		WIT: "./wit", WITMode: witgo.WITInputPackage,
		World: "plugin", Output: "./internal/contract",
		Package: "contract",
	},
	Build: witgo.GuestBuildConfig{
		Main: "./cmd/plugin", World: "plugin",
		WITPackage: "./wit",
		Output: "./dist/plugin.component.wasm",
		Manifest: &witgo.PluginManifest{Dependencies: map[string]string{
			"example:plugins/cache@1.0.0": "cache.component.wasm",
		}},
	},
})
```

В коде плагина остаётся только реализовать generated `<Interface>Guest` и
вызвать `Export<World>`; собранный файл затем открывается generated host API.

Если подробный отчёт не нужен, используйте короткую проверку:

```go
if err := contract.CheckPlugin("./plugins/plugin.component.wasm"); err != nil {
	log.Fatal(err)
}
```

Ошибка от `CheckPlugin` поддерживает `errors.Is(err, witgo.ErrContractMismatch)`
и `errors.As` к `*witgo.ContractValidationError`.

## Что важно знать

- runtime работает in-process через доверенный Rust bridge, а не через sidecar;
- перед запуском всегда проверяются contract manifest и version handshake;
- вложенные плагины связываются через WIT imports/exports, но не получают
  прямой доступ к registry, runtime options или чужим runtime;
- системный vendor API `witgo:runtime/runtime@1.0.0` подключается только явно
  через `EnableRuntimeAPI` и даёт guest-коду только локальное состояние вызова;
- запрос дополнительного fuel возможен только через
  `UnsafeRequestAdditionalFuel`, а решение всегда остаётся за host policy;
- `resource`, `future`, `stream` и `error-context` передаются как
  runtime-bound `witgo.Handle` и не могут безопасно мигрировать между
  независимыми runtime-box;
- обычный Go использует `purego` без CGO на desktop Linux/macOS/Windows;
- TinyGo native runtime поддерживается на Linux и Windows с CGo; TinyGo 0.41 на
  macOS компилирует API и generator, но не может связать `dlopen` backend.

Ключевые ограничения и поведение собраны в [docs/capabilities.md](docs/capabilities.md),
архитектура и модель доверия - в [docs/architecture.md](docs/architecture.md).

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
- [Go WASM plugin + Go host](docs/go-guest-plugins.md)
- [Архитектура runtime](docs/architecture.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Проверка контрактов](docs/validation.md)
- [Generated code](docs/generated-code.md)
- [Go type overlays](docs/go-overlays.md)
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
