# Scenario: Server

Этот сценарий показывает типизированный happy path:

- используется generated package из `examples/contracts/basic/out`;
- открывается общий component из `examples/components/basic/component.wasm`;
- host реализует `process-string`;
- приложение читает metadata через generated client.

Запуск:

```sh
go run ./examples/scenarios/server
```
