# Базовый контракт

Этот каталог показывает полный путь от WIT-контракта до generated Go bindings.

Запуск:

```sh
go run ./examples/contracts/basic
go run ./examples/scenarios/server
```

Что здесь лежит:

- `wit/` - исходный WIT-контракт;
- `main.go` - generator entrypoint для `witgo.Generate`;
- `out/bindings.gen.go` - сгенерированный Go package;
- `examples/components/basic/component.wasm` используется runnable-сценариями.

Этот базовый контракт затем используют:

- `examples/scenarios/server`;
- `examples/scenarios/validate`;
- `examples/scenarios/inspect`.
