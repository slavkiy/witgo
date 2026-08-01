# Scenario: Validate

Этот сценарий показывает предзапусковую проверку component-контракта.

Он использует:

- generated contract из `examples/contracts/basic/out`;
- общий component из `examples/components/basic/component.wasm`.

Что демонстрирует:

- `ValidatePlugin` с подробным отчётом;
- `CheckPlugin` для короткого startup-check;
- чтение `ImportNames` и `ExportNames` из generated manifest.

Запуск:

```sh
go run ./examples/scenarios/validate
```
