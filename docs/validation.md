# Contract validation

Generated packages validate a WebAssembly Component before it is added to a
pool or exposed to application traffic:

```go
report, err := contract.ValidatePlugin("./plugin.wasm")
if err != nil {
	return fmt.Errorf("inspect plugin: %w", err)
}
if err := report.Err(); err != nil {
	return err
}
```

Inspection loads and parses the Component but stops at the bridge `pong`. It
does not instantiate the Component, link application host callbacks, run a
start function, or invoke guest code.

## What is compared

The generated contract and actual Component manifest contain:

- fully qualified import function names;
- fully qualified export function names;
- package and interface versions as part of those names;
- deterministic structural parameter and result signatures.

Signatures recursively describe primitives, records, lists, tuples, options,
results, variants, enums and flags. A plugin cannot pass merely by exporting a
function with the correct name and a different ABI.

Resource ownership appears in signatures as `own` or `borrow`, allowing ABI
validation without constructing a resource or executing guest code.

## Detailed reports

```go
if !report.Compatible {
	log.Printf("missing imports: %v", report.Imports.Missing)
	log.Printf("unexpected imports: %v", report.Imports.Unexpected)
	log.Printf("missing exports: %v", report.Exports.Missing)
	log.Printf("unexpected exports: %v", report.Exports.Unexpected)
	for _, mismatch := range report.Signatures {
		log.Printf("%s: expected %s, actual %s",
			mismatch.Function, mismatch.Expected, mismatch.Actual)
	}
}
```

`ValidatePlugin` reserves its second return value for operational errors such
as an unreadable file, invalid Wasm, or an incompatible native bridge. Contract
incompatibility is represented by a successful report with `Compatible ==
false`.

## Error-only startup checks

Generated `CheckPlugin` is convenient for command startup paths:

```go
if err := contract.CheckPlugin(path); err != nil {
	if errors.Is(err, witgo.ErrContractMismatch) {
		var mismatch *witgo.ContractValidationError
		if errors.As(err, &mismatch) {
			log.Print(mismatch.Report.Summary())
		}
	}
	return err
}
```

`ValidationReport.Err()` and `witgo.RequireCompatible(report)` provide the same
conversion when the detailed API is used directly.

## In-memory components

Applications receiving Components from an existing trusted storage layer do
not need to create their own temporary file:

```go
report, err := witgo.ValidateComponentBytes(componentBytes, contract.PluginPing())
```

The library creates a private temporary component file because Wasmtime loads
Components by path, removes it after inspection, and never instantiates it.

## Inspecting capabilities

The raw manifest is useful before a capability policy is introduced:

```go
manifest, err := witgo.InspectComponent(path)
if err != nil {
	return err
}
for _, required := range manifest.ImportNames() {
	log.Printf("plugin requires %s", required)
}
```

Use `ImportNames`, `ExportNames`, and `FunctionNames` instead of mutating the
slices in `Contract`; the accessor methods return sorted copies.

Two cached or remotely supplied manifests can be compared without opening the
bridge:

```go
report, err := witgo.CompareContracts(expected, actual)
```
