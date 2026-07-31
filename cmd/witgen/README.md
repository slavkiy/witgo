# witgen

`witgen` is an optional generation-only CLI. It renders deterministic English
and Russian diagnostics locally and has no dependency besides `witgo`.

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
- `-auto-translate`: deprecated compatibility flag (no network translation is
  performed).

The CLI, `witgo` library, and generated code support Go 1.18 and newer.
