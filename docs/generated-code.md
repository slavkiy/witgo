# Generated Go code

WIT world превращается в Go client с typed imports и exports.

```wit
interface host {
    process-string: func(value: string) -> string;
}

interface metadata {
    record info { name: string, version: string }
    get: func() -> info;
}

world plugin {
    import host;
    export metadata;
}
```

Основная форма generated API:

```go
type Host interface {
	ProcessString(value string) string
}

type Metadata interface {
	Get() (Info, error)
}

type Plugin struct {
	Metadata Metadata
}

type PluginImports struct {
	Host Host
}

func PluginPing() witgo.Contract
func ValidatePlugin(filename string) (witgo.ValidationReport, error)
func ValidatePluginWithOptions(filename string, options witgo.RuntimeOptions) (witgo.ValidationReport, error)
func CheckPlugin(filename string) error
func CheckPluginWithOptions(filename string, options witgo.RuntimeOptions) error
func OpenPlugin(filename string, imports PluginImports) (*Plugin, error)
func OpenPluginWithOptions(filename string, options witgo.RuntimeOptions, imports PluginImports) (*Plugin, error)
func (p *Plugin) Close() error
```

`PluginPing` is generated from the WIT world and contains sorted host import
and plugin export function names. Opening a plugin asks the bridge for the
component's actual function manifest and rejects missing or unexpected names.
Using one imports struct keeps constructor signatures stable as large contracts
gain more host interfaces.

Validation is read-only and does not instantiate the component or execute guest
code:

```go
report, err := contract.ValidatePlugin("./plugin.wasm")
if err != nil {
	return err
}
if !report.Compatible {
	return fmt.Errorf("incompatible plugin: %+v", report)
}
```

`report.Imports` and `report.Exports` contain separate `Missing` and
`Unexpected` function lists. `report.Signatures` contains structural parameter
or result type mismatches, including nested records, lists, options, results,
tuples, enums, flags and variants. Fully qualified interface names include WIT
package versions, so a version mismatch is visible in the same report.

For startup code that only needs success or failure, `CheckPlugin` converts an
incompatible report to `*witgo.ContractValidationError`:

```go
if err := contract.CheckPlugin("./plugin.wasm"); err != nil {
	if errors.Is(err, witgo.ErrContractMismatch) {
		var mismatch *witgo.ContractValidationError
		if errors.As(err, &mismatch) {
			log.Printf("contract problems: %s", mismatch.Report.Summary())
		}
	}
	return err
}
```

## Composite value representation

- WIT `char`, `option`, `result` and tuples use `witgo.Char`, `Option`,
  `Result`, typed `Tuple0...Tuple16`, and flexible `Tuple` with strict codecs.
- WIT maps use `witgo.Map[K,V]` and the Component Model pair-array wire form.
- WIT enums are named Go string types with `Parse`, `Valid`, `String` and
  values-list helpers.
- WIT flags remain named `uint64` bit sets and add `Parse`, `Valid`, `Has`,
  `Add`, `Remove` and `Names` helpers.
- WIT variants implement strict `{case,value}` encoding and generate a
  constructor, predicate and safe accessor for each case.
- Nested lists retain their ordinary recursive Go slice shape.

Resources, `future<T>`, `stream<T>` and `error-context` are represented by
`witgo.Handle`. A returned handle remains attached to its originating Runtime,
can be passed back into that Runtime, and has an idempotent `Close` method.
Resource ownership is enforced by the bridge. Future/stream payload reading
and writing are not generated yet; their current API is opaque transport and
lifecycle management.

Export вызывается удобно и соответствует владельцу из WIT:

```go
info, err := plugin.Metadata.Get()
```

Generated constructor создаёт `witgo.HostImport` adapters до instantiation.
Adapter проверяет число аргументов, поднимает каждое значение в Go type и
вызывает реализацию `Host`. Nil import отклоняется сразу.

Records передаются по Canonical ABI движком Wasmtime. Внутренний JSON-канал
между Go и bridge является implementation detail и не задаёт ABI плагина.

Ошибки export call возвращаются напрямую. Ошибка преобразования результата
оборачивается с полным WIT именем функции.

Generated package рассчитан на Go 1.18+. Файл рекомендуется хранить в Git,
чтобы изменения публичного API были видны в review.
