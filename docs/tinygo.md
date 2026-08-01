# TinyGo

## Автоматическое определение роли

Библиотека различает host и plugin во время компиляции:

- нативные Linux, macOS и Windows builds получают роль `witgo.ExecutionRoleHost`;
- `GOARCH=wasm`, включая TinyGo WASI, получает роль `witgo.ExecutionRolePlugin`.

Проверить роль можно через `witgo.CurrentExecutionRole()`, `witgo.IsHostBuild()` и `witgo.IsPluginBuild()`. Пользовательская настройка или runtime-флаг не нужны.

Публичные типы `witgo` и код, создаваемый `witgen`, совместимы с TinyGo. Генератор использует только стандартные типы Go, `context.Context` и обобщённые helper-типы, поддерживаемые актуальным TinyGo.

## Контекст во всех операциях

Для серверного кода следует использовать контекстные варианты:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

report, err := contract.ValidatePluginContext(ctx, componentPath)
plugin, err := contract.OpenPluginContext(ctx, componentPath, imports)
metadata, err := plugin.PluginInfo.Metadata(ctx)
err = plugin.RestartContext(ctx)
```

Generated host-интерфейсы тоже принимают контекст:

```go
type Host interface {
	ProcessString(ctx context.Context, value string) (string, error)
}
```

Контекст передаётся из export-вызова в host callback. Отмена проверяется до загрузки, до вызова, после ожидания внутренней блокировки и между сообщениями bridge. Для жёсткого прерывания уже выполняющегося Wasm-кода задавайте также `RuntimeOptions.Timeout`: это ограничение реализовано Wasmtime через epoch interruption.

Методы без суффикса `Context` сохранены как удобные обёртки с `context.Background()`.

## Проверка сборки

```sh
make tinygo-test
make tinygo-e2e
```

`tinygo-test` компилирует все пакеты, а `tinygo-e2e` сначала собирает Rust bridge и затем выполняет version handshake через CGo-loader. CI выполняет оба шага на TinyGo 0.41.1. Локально без установленного TinyGo можно проверить fallback без CGo:

```sh
CGO_ENABLED=0 go test -tags tinygo ./... -run '^$'
```

## Два backend-а native bridge

- обычный Go использует `purego` и не требует CGO;
- TinyGo использует компактный CGo-loader на `dlopen`/`dlsym` для Linux и macOS и на `LoadLibrary`/`GetProcAddress` для Windows.

Оба backend-а загружают один и тот же version-matched Rust bridge (`.so`, `.dylib` или `.dll`) и используют одинаковый C ABI `witgo_bridge_*`. Generated API от выбранного backend-а не зависит.

Выбранный вариант можно вывести в диагностике через `witgo.CurrentNativeBridgeBackend()`: он возвращает `purego`, `cgo` или `unsupported`.

Для TinyGo CGo должен быть включён. Если он отключён, пакет всё равно компилируется, но `LoadRuntime` возвращает явную ошибку `TinyGo backend requires CGo`. Путь к внешней библиотеке можно передать через `RuntimeOptions.BridgePath` или `WITGO_COMPONENT_LIBRARY`; встроенный bridge также поддерживается.
