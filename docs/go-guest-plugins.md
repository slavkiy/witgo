# Go WASM plugins (guest mode)

Обычный режим создаёт host SDK (`OpenPlugin`, validation и composition API).
Коду самого плагина нужна другая поверхность, поэтому используется guest mode:

- `<Interface>Guest` - интерфейс реализации export;
- `<World>Guest` и `Export<World>` - регистрация реализаций;
- `Imports.<Interface>` - типизированные вызовы host imports;
- Canonical ABI `wasmimport`/`wasmexport` glue от `wit-bindgen-go`.

Один раз установите guest toolchain и runtime-типы generated bindings:

```sh
go install go.bytecodealliance.org/cmd/wit-bindgen-go@v0.7.0
go get go.bytecodealliance.org/cm@v0.7.0
```

```sh
witgen -mode guest -wit ./wit -wit-mode package -world plugin \
  -out ./internal/contract -package contract
```

Если `wit-bindgen-go` отсутствует в `PATH`, генератор запускает закреплённую
версию через `go run`. Путь к бинарнику задаётся флагом `-guest-bindgen`.

```go
package main

import contract "example.com/plugin/internal/contract"

type metadata struct{}

func (metadata) Name() string {
	return contract.Imports.Host.Process("my-plugin")
}

func init() {
	if err := contract.ExportPlugin(contract.PluginGuest{Metadata: metadata{}}); err != nil {
		panic(err)
	}
}

func main() {}
```

## Сборка Component

TinyGo `wasip2` добавляет Component Type metadata и создаёт WebAssembly
Component. Генерацию и сборку можно выполнить одной командой:

```sh
witgen -mode guest -wit ./wit -world plugin \
  -out ./internal/contract -package contract \
  -build-main ./cmd/plugin \
  -wit-package ./wit/plugin.wit.package.wasm \
  -component-out ./dist/plugin.component.wasm -no-debug
```

Эквивалентный library API - `witgo.BuildGuestComponent`.

`ExecutionRolePlugin` ABI не создаёт: это только compile-time признак
`GOARCH=wasm`. Рабочий guest получается из guest bindings, регистрации
экспортов и TinyGo component build.
