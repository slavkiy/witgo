# Scenario: Handles

Этот сценарий показывает lifecycle живого `resource` handle на low-level API.

Локальные файлы:

- `component.wat` - текст component;
- `component.wasm` - готовый бинарный артефакт;
- `main.go` - создание, borrow-вызов и `Close` для `witgo.Handle`.

Запуск:

```sh
go run ./examples/scenarios/handles
```
