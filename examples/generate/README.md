# Component plugin example

```powershell
go run ./examples/generate
go run ./examples/server
```

Пример показывает полный путь:

- WIT объявляет `host.process-string` и exported `plugin-info.metadata`;
- generated Go package создаёт typed host adapter;
- стандартный Component из `examples/plugin/plugin.wat` вызывает Go host;
- server получает `PluginMetadata` через `plugin.PluginInfo.Metadata()`.

В production вместо текстового WAT передаётся собранный
`plugin.component.wasm`.
