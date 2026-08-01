# Пример Component-плагина

```powershell
go run ./examples/generate
go run ./examples/server
```

Пример показывает полный путь:

- WIT объявляет `host.process-string` и exported `plugin-info.metadata`;
- generated Go package создаёт типизированный host adapter и проверки
  контракта;
- Component из `examples/plugin/component.wasm` вызывает Go host;
- server получает `PluginMetadata` через `plugin.PluginInfo.Metadata()`.

В production здесь уже можно использовать готовый
`examples/plugin/component.wasm`, а исходный текст component лежит в
`examples/plugin/component.wat`.

Дополнительные сценарии использования лежат в соседних каталогах:

- `examples/validate` показывает проверку контракта до запуска;
- `examples/inspect` выводит imports/exports/signatures без instantiation;
- `examples/collections` демонстрирует `map`, вложенные `list` и `variant`;
- `examples/handles` показывает lifecycle `resource` handle.
