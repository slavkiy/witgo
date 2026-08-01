# TinyGo

## Роль и доступность runtime

Execution role определяется во время сборки:

- native targets получают `witgo.ExecutionRoleHost`;
- `GOARCH=wasm`, включая WASI, получает `witgo.ExecutionRolePlugin`.

`ExecutionRolePlugin` - только compile-time признак. Он не генерирует Canonical
ABI exports/imports, Component Type metadata и функцию регистрации экспортов.
Для рабочего Go-плагина сначала нужны bindings с `Mode: GenerateGuest`, затем
регистрация через `Export<World>` и сборка TinyGo `wasip2`.

Роль также не означает автоматическую доступность native runtime. Parser,
generator, generated types и plugin-role API компилируются на TinyGo targets, но загрузка
Rust bridge зависит от платформы:

- Linux: CGo loader через `dlopen`/`dlsym`;
- Windows: CGo loader через `LoadLibraryW`/`GetProcAddress`;
- macOS с TinyGo 0.41: generation-only, потому что toolchain не связывает
  системные `dlopen` symbols;
- без CGo: generation-only с явной ошибкой при `LoadRuntime`.

Проверить состояние можно через `CurrentExecutionRole()` и
`CurrentNativeBridgeBackend()`. Backend возвращает `cgo` или `unsupported` для
TinyGo; generated API от него не зависит.

## Сборка Go guest-компонента

Установите инструменты:

```sh
go install go.bytecodealliance.org/cmd/wit-bindgen-go@v0.7.0
go get go.bytecodealliance.org/cm@v0.7.0
```

Сгенерируйте guest bindings:

```sh
witgen -mode guest -wit ./wit -wit-mode package -world plugin \
  -out ./internal/contract -package contract
```

После вызова `contract.ExportPlugin(...)` из `init` соберите Component:

```sh
witgen -mode guest -wit ./wit -world plugin \
  -out ./internal/contract -package contract \
  -build-main ./cmd/plugin \
  -wit-package ./wit/plugin.wit.package.wasm \
  -component-out ./dist/plugin.component.wasm -no-debug
```

Library-вариант - `witgo.BuildGuestComponent(GuestBuildConfig{...})`.
`-target=wasip2` относится к guest build и не использует desktop Rust bridge.
Напротив, TinyGo+CGo loader ниже относится к host-программе, которая загружает
чужие Components. Это два независимых сценария.

## Context API host-приложения

Для server-кода используйте контекстные операции:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

report, err := contract.ValidatePluginContext(ctx, componentPath)
plugin, err := contract.OpenPluginContext(ctx, componentPath, imports)
metadata, err := plugin.Metadata.Get(ctx)
err = plugin.RestartContext(ctx)
```

Generated host interfaces также принимают `context.Context`. Отмена проверяется
на Go/bridge boundary. Для прерывания уже выполняющегося Wasm дополнительно
задайте `RuntimeOptions.Timeout`; зависший доверенный Go callback он не убивает.

## Проверка

```sh
make tinygo-test
make tinygo-e2e
make wasm-role-test
```

Эквивалент compile-only команды CI:

```sh
CGO_ENABLED=1 tinygo test -run '^$' ./...
```

`tinygo-e2e` поддерживается на Linux CI: он собирает Rust bridge и проверяет C
ABI/version handshake через TinyGo CGo backend. Fallback без CGo можно проверить
обычным Go compiler:

```sh
CGO_ENABLED=0 go test -tags tinygo -run '^$' ./...
```

Bridge выбирается через embedded artifact, `RuntimeOptions.BridgePath` или
`WITGO_COMPONENT_LIBRARY`; handshake и C ABI `witgo_bridge_*` одинаковы для Go
и TinyGo.
