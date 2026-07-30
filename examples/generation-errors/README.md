# Generation diagnostics

This directory contains an intentionally invalid WIT contract for checking
generation errors rendered by the optional `witgen` CLI.

From the repository root:

```sh
cd cmd/witgen
go run . \
  -wit ../../examples/generation-errors/invalid.wit \
  -out ../../examples/generation-errors/out \
  -lang ru \
  -auto-translate=false
```

The command must fail with a localized `GenerationFailed` diagnostic. No output
package is generated.

To check automatic translation of a parser error, remove
`-auto-translate=false`. Automatic translation can use the network. This
affects only CLI diagnostics during generation, never the generated library.
