# Scenario: Collections

Этот сценарий показывает low-level `witgo.Runtime` на self-contained component.

Локальные файлы:

- `component.wat` - текст component;
- `component.wasm` - готовый бинарный артефакт;
- `main.go` - вызовы nested lists и variant через `Runtime.Call`.

Запуск:

```sh
go run ./examples/scenarios/collections
```
