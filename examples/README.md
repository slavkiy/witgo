# Примеры

Теперь каталог `examples` разбит по ролям, а не по случайным именам:

- `contracts/` - WIT-контракты, generator entrypoint и generated Go bindings;
- `components/` - готовые reusable component-артефакты (`.wat` и `.wasm`);
- `scenarios/` - runnable примеры хоста и low-level runtime сценарии.

Текущая раскладка:

- [contracts/basic](contracts/basic/README.md): базовый WIT-контракт и
  generated bindings;
- [contracts/invalid](contracts/invalid/README.md): заведомо невалидный
  контракт для проверки диагностики;
- [components/basic](components/basic/README.md): общий component-артефакт для
  типизированных host-сценариев;
- [scenarios/server](scenarios/server/main.go): открытие plugin через generated
  client;
- [scenarios/nested](scenarios/nested/main.go): автоматическая вложенность без
  ручной реализации host interface;
- [scenarios/validate](scenarios/validate/main.go): validation и короткий
  startup-check;
- [scenarios/inspect](scenarios/inspect/main.go): inspection imports/exports и
  structural signatures;
- [scenarios/collections](scenarios/collections/main.go): self-contained пример
  с nested lists и variant;
- [scenarios/handles](scenarios/handles/main.go): self-contained пример с живым
  `resource` handle.

Если коротко:

- `contracts` отвечает за “что описано в WIT”;
- `components` отвечает за “какой guest binary используем”;
- `scenarios` отвечает за “как это выглядит со стороны приложения”.

Быстрые команды:

```sh
go run ./examples/contracts/basic
go run ./examples/scenarios/server
go run ./examples/scenarios/nested
go run ./examples/scenarios/validate
go run ./examples/scenarios/inspect
go run ./examples/scenarios/collections
go run ./examples/scenarios/handles
```
