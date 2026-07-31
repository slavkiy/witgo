# Generation diagnostics

This directory contains an intentionally invalid WIT contract for checking
generation errors rendered by the optional `witgen` CLI.

From the repository root:

```sh
cd cmd/witgen
go run . \
  -wit ../../examples/generation-errors/invalid.wit \
  -out ../../examples/generation-errors/out \
  -lang ru
```

The command must fail with a localized `GenerationFailed` diagnostic. No output
package is generated.
