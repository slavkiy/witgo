# witgen

`witgen` is an optional generation-only CLI. It uses
[`digreyt`](https://github.com/slavkiy/digreyt) to render localized diagnostics.
Neither the `witgo` generator, runtime, nor generated packages import
`digreyt`.

```sh
cd cmd/witgen
go install .

witgen \
  -wit ./wit \
  -out ./internal/contract \
  -lang ru
```

Flags:

- `-wit`: WIT file or directory;
- `-out`: output directory;
- `-package`: optional Go package override;
- `-filename`: optional generated filename;
- `-lang`: diagnostic language; defaults to `LANG`;
- `-auto-translate`: automatically translate missing diagnostic text.

Automatic translation is used only when generation fails. It may access the
translation provider over the network. Use `-auto-translate=false` for
deterministic offline diagnostics.

This CLI is a separate Go module because the current `digreyt` release requires
Go 1.25.6. The `witgo` library and generated code continue to support Go 1.24.
