# TinyGo

## Роль и доступность runtime

Execution role определяется во время сборки:

- native targets получают `witgo.ExecutionRoleHost`;
- `GOARCH=wasm`, включая WASI, получает `witgo.ExecutionRolePlugin`.

Роль не означает автоматическую доступность native runtime. Parser, generator,
generated types и plugin-role API компилируются на TinyGo targets, но загрузка
Rust bridge зависит от платформы:

- Linux: CGo loader через `dlopen`/`dlsym`;
- Windows: CGo loader через `LoadLibraryW`/`GetProcAddress`;
- macOS с TinyGo 0.41: generation-only, потому что toolchain не связывает
  системные `dlopen` symbols;
- без CGo: generation-only с явной ошибкой при `LoadRuntime`.

Проверить состояние можно через `CurrentExecutionRole()` и
`CurrentNativeBridgeBackend()`. Backend возвращает `cgo` или `unsupported` для
TinyGo; generated API от него не зависит.

## Context API

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
