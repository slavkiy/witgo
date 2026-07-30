# Plugin metadata example

Generate the contract from WIT:

```sh
go run ./examples/generate
```

Build the real Wasm plugin and run the server:

```sh
go run ./examples/plugin
go run ./examples/server
```

The example contains three layers:

- `generate/wit` declares `plugin-metadata` and the exported `plugin-info`;
- `plugin/plugin.wat` implements the metadata export and is compiled to
  `plugin/plugin.wasm`;
- `server` loads the Wasm file through `contract.OpenPlugin` and prints the
  typed metadata.

The server only imports the generated contract:

```go
client, err := contract.OpenPlugin("plugin.wasm")
metadata, err := client.Metadata()
```
