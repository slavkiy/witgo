# Сценарий написания Go-плагина

Ниже полный путь от контракта хоста до загрузки готового Go/TinyGo-плагина.
Хост и плагин используют один WIT package, но генерируют разные Go API:

- хост получает `OpenPluginWithPolicy`, typed imports и клиенты exports;
- плагин получает `<Interface>Guest`, `ExportPlugin` и `Imports.<Interface>`;
- готовый артефакт - WebAssembly Component, а не core Wasm module.

## 1. Структура проектов

Контракт обычно принадлежит хосту и версионируется отдельно. Для первого
плагина достаточно скопировать один и тот же каталог `wit` в оба проекта:

```text
my-host/
  wit/
    plugin.wit
  internal/contract/       # generated host API
  plugins/
  cmd/host/main.go

my-plugin/
  wit/
    plugin.wit              # та же версия контракта
  internal/contract/        # generated guest API
  cmd/plugin/main.go
  tools/build.go
  dist/
```

Изменение namespace, package version, world, импортов или экспортов означает
изменение контракта. Хост проверит его до запуска guest-кода.

## 2. Напишите контракт на стороне хоста

`my-host/wit/plugin.wit`:

```wit
package example:plugins@1.0.0;

interface host {
    log: func(message: string);
}

interface plugin-api {
    record info {
        name: string,
        version: string,
    }

    metadata: func() -> info;
    transform: func(value: string) -> string;
}

world plugin {
    import host;
    export plugin-api;
}
```

Направление читается со стороны плагина:

- `import host` - функция, которую предоставляет host-приложение;
- `export plugin-api` - функции, которые обязан реализовать плагин.

## 3. Сгенерируйте API хоста

`my-host/generate.go`:

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

```sh
cd my-host
go run generate.go
```

Host package теперь содержит `Host`, `PluginImports`, `Plugin`,
`PluginPing`, `ValidatePlugin` и `OpenPluginWithPolicy`.

## 4. Создайте проект плагина

```sh
mkdir -p my-plugin/{wit,cmd/plugin,internal/contract,tools,dist}
cd my-plugin
go mod init example.com/my-plugin
go get github.com/slavkiy/witgo
```

Скопируйте `my-host/wit/plugin.wit` в `my-plugin/wit/plugin.wit`. Не создавайте
отдельную изменённую копию контракта под реализацию: host и plugin должны
собираться против одной версии WIT.

Для guest generation и build нужны TinyGo и `wit-bindgen-go`:

```sh
tinygo version
go install github.com/bytecodealliance/wit-bindgen-go/cmd/wit-bindgen-go
wit-bindgen-go version
```

Если бинарник установлен не в `PATH`, передайте его абсолютный путь через
`Config.GuestBindgen` или CLI-флаг `-guest-bindgen`.

## 5. Сгенерируйте guest API

Можно сразу использовать единый build-файл. `my-plugin/tools/build.go`:

```go
package main

import (
	"log"

	"github.com/slavkiy/witgo"
)

func main() {
	err := witgo.BuildPlugin(witgo.PluginBuildConfig{
		Generate: witgo.Config{
			WIT:         "./wit",
			WITMode:     witgo.WITInputPackage,
			World:       "plugin",
			Output:      "./internal/contract",
			Package:     "contract",
			PackageRoot: "example.com/my-plugin/internal/contract",
		},
		Build: witgo.GuestBuildConfig{
			Main:       "./cmd/plugin",
			World:      "plugin",
			WITPackage: "./wit",
			Output:     "./dist/my-plugin.component.wasm",
			NoDebug:    true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Первый запуск пока завершится ошибкой компиляции, потому что реализация
`cmd/plugin` ещё не написана, но guest bindings уже появятся. Если хочется
разделить операции, сначала вызовите `witgo.Generate` с `Mode:
witgo.GenerateGuest`, а после реализации - `witgo.BuildGuestComponent`.

## 6. Реализуйте generated guest-интерфейс

После generation посмотрите точную сигнатуру `PluginAPIGuest` в
`internal/contract/bindings.gen.go`. Для контракта выше реализация выглядит
так (`my-plugin/cmd/plugin/main.go`):

```go
package main

import (
	contract "example.com/my-plugin/internal/contract"
	pluginapi "example.com/my-plugin/internal/contract/example/plugins/plugin-api"
)

type plugin struct{}

func (plugin) Metadata() pluginapi.Info {
	contract.Imports.Host.Log("metadata requested")
	return pluginapi.Info{
		Name:    "uppercase",
		Version: "1.0.0",
	}
}

func (plugin) Transform(value string) string {
	contract.Imports.Host.Log("transform requested")
	return value + " from plugin"
}

func init() {
	err := contract.ExportPlugin(contract.PluginGuest{
		PluginAPI: plugin{},
	})
	if err != nil {
		panic(err)
	}
}

func main() {}
```

В guest API нет `context.Context` и `error` у обычных WIT-функций: сигнатуру
определяет WIT. Доменные ошибки следует описывать через WIT `result<T, E>`.
`Imports.Host.Log` является реальным вызовом из Wasm в host и сработает только
если host передаст реализацию импорта и разрешит capability policy.

После generation обновите зависимости проекта:

```sh
go mod tidy
```

## 7. Соберите Component

```sh
go run ./tools/build.go
```

`BuildPlugin` выполняет две операции:

1. повторно генерирует guest bindings из актуального WIT;
2. запускает TinyGo с `-target=wasip2`, `--wit-package ./wit` и выбранным world.

Результат:

```text
dist/my-plugin.component.wasm
```

Это готовый Component Model plugin. Его не нужно дополнительно оборачивать
через `wasm-tools component new`.

## 8. При необходимости объявите plugin-зависимости

Если этот плагин импортирует интерфейс другого плагина, manifest можно встроить
во время сборки:

```go
Build: witgo.GuestBuildConfig{
	Main:       "./cmd/plugin",
	World:      "plugin",
	WITPackage: "./wit",
	Output:     "./dist/my-plugin.component.wasm",
	Manifest: &witgo.PluginManifest{
		Dependencies: map[string]string{
			"example:plugins/cache@1.0.0": "cache.component.wasm",
		},
	},
```

Путь разрешается относительно основного component. Manifest не выдаёт права:
хост всё равно должен включить `LoadPlugin` и ограничить допустимые корни.

## 9. Реализуйте импорт и загрузите плагин на хосте

`my-host/cmd/host/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/slavkiy/witgo"
	contract "example.com/my-host/internal/contract"
)

type pluginHost struct{}

func (pluginHost) Log(_ context.Context, message string) error {
	log.Printf("plugin: %s", message)
	return nil
}

func main() {
	ctx := context.Background()

	policy := witgo.HostPolicy{
		Public: witgo.PluginGrant{
			Permissions: witgo.Permissions{
				// Разрешаем typed import, который реализован ниже.
				Allow: []string{"example:plugins/host@1.0.0"},
			},
			Limits: witgo.PluginLimits{
				FuelPerCall:      1_000_000,
				Timeout:          2 * time.Second,
				MemoryLimitBytes: 64 << 20,
				MaxResultBytes:   1 << 20,
			},
		},
	}

	instance, err := contract.OpenPluginWithPolicyContext(
		ctx,
		policy,
		"uppercase",
		"./plugins/my-plugin.component.wasm",
		contract.PluginImports{Host: pluginHost{}},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer instance.Close()

	info, err := instance.PluginAPI.Metadata(ctx)
	if err != nil {
		log.Fatal(err)
	}

	result, err := instance.PluginAPI.Transform(ctx, "hello")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s %s: %s\n", info.Name, info.Version, result)
}
```

Скопируйте собранный component и запустите host:

```sh
cp ../my-plugin/dist/my-plugin.component.wasm ./plugins/
go run ./cmd/host
```

## 10. Как выдаются права

Нулевая `HostPolicy` запрещает ambient capabilities и nested plugin loading.
Разрешения задаёт только host:

```go
policy := witgo.HostPolicy{
	Public: witgo.PluginGrant{
		Permissions: witgo.Permissions{System: true},
		Limits:      witgo.PluginLimits{FuelPerCall: 500_000},
	},
	Plugins: map[string]witgo.PluginGrant{
		"http-client": {
			Permissions: witgo.Permissions{Network: true},
		},
		"indexer": {
			Permissions: witgo.Permissions{Files: true},
		},
		"orchestrator": {
			Permissions:       witgo.Permissions{LoadPlugin: true},
			AllowedPluginRoots: []string{"./plugins"},
		},
	},
}
```

- `Public` применяется ко всем plugin ID;
- named grant добавляет разрешения конкретному plugin ID;
- `System`, `Network`, `Files` разрешают соответствующие WASI namespaces;
- `Allow` разрешает custom WIT imports, `Deny` всегда имеет приоритет;
- `LoadPlugin` разрешает только manifest-based зависимости;
- fuel, memory, timeout и остальные лимиты принадлежат хосту и не могут быть
  увеличены manifest или кодом плагина.

## 11. Быстрая диагностика

Перед загрузкой можно проверить Component отдельно:

```go
report, err := contract.ValidatePlugin("./plugins/my-plugin.component.wasm")
if err != nil {
	log.Fatal(err)
}
if err := report.Err(); err != nil {
	log.Fatal(err)
}
```

Типовые ошибки:

- `ErrCoreModule` - собран core Wasm вместо Component;
- `ErrContractMismatch` - host и plugin используют разные WIT-контракты;
- `ErrCapabilityDenied` - импорт реализован, но не разрешён policy;
- `ErrFuelExhausted` - plugin исчерпал host-owned budget;
- `ErrNestedPluginPathDenied` - manifest указывает за разрешённые корни.

Для дополнительных деталей смотрите [generated-code.md](generated-code.md),
[nested-plugins.md](nested-plugins.md) и [troubleshooting.md](troubleshooting.md).
