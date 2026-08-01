# Сгенерированный Go-код

WIT `world` превращается в Go client с типизированными imports и exports.

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

`PluginPing` генерируется из WIT `world` и содержит отсортированный manifest
host import и plugin export функций. При открытии плагина bridge сначала
возвращает фактический manifest компонента, а bindings отклоняют missing или
unexpected функции ещё до запуска guest-кода.

Использование одного imports-struct держит сигнатуры конструкторов стабильными
даже у очень больших контрактов.

Validation работает только на чтение и не instantiate-ит component:

```go
report, err := contract.ValidatePlugin("./plugin.wasm")
if err != nil {
	return err
}
if !report.Compatible {
	return fmt.Errorf("incompatible plugin: %+v", report)
}
```

`report.Imports` и `report.Exports` содержат отдельные списки `Missing` и
`Unexpected`. `report.Signatures` содержит структурные несовпадения параметров
или результатов, включая вложенные records, lists, options, results, tuples,
maps, enums, flags и variants.

Для startup-кода, где нужен только success/failure, удобен `CheckPlugin`:

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

## Представление сложных значений

- WIT `char`, `option`, `result` и tuples используют `witgo.Char`,
  `witgo.Option`, `witgo.Result`, типизированные `Tuple0...Tuple16` и
  динамический `Tuple` со строгим codec.
- WIT maps используют `witgo.Map[K,V]` и pair-array wire form из Component
  Model ABI.
- WIT enums генерируются как именованные Go string-типы с helper-функциями
  `Parse`, `Valid`, `String` и `<Type>Values`.
- WIT flags остаются именованными `uint64` bitset и получают `Parse`, `Valid`,
  `Has`, `Add`, `Remove` и `Names`.
- WIT variants используют строгую форму `{case,value}` и получают constructor,
  predicate и безопасный accessor для каждого case.
- Вложенные lists сохраняют обычную рекурсивную форму `[]T`.

`resource`, `future<T>`, `stream<T>` и `error-context` представлены через
`witgo.Handle`. Возвращённый handle остаётся привязан к породившему его
`Runtime`, может быть передан обратно в тот же runtime и имеет идемпотентный
`Close`. Bridge проверяет корректность ownership. Для `future` и `stream`
высокоуровневое чтение/запись typed payload пока не генерируется.

Экспорт вызывается обычным типизированным методом:

```go
info, err := plugin.Metadata.Get()
```

Generated constructor создаёт `witgo.HostImport` adapters до instantiation.
Adapter проверяет число аргументов, поднимает каждое значение в Go-тип и
вызывает реализацию `Host`. `nil` import отклоняется сразу.

Records и остальные составные типы передаются по Canonical ABI движком
Wasmtime. Внутренний JSON-канал между Go и bridge остаётся implementation
detail и не задаёт ABI плагина.

Ошибки export-вызова возвращаются напрямую. Ошибка преобразования результата
оборачивается с полным WIT-именем функции.

Generated package рассчитан на Go 1.18+. Файл имеет смысл хранить в Git, чтобы
изменения публичного API были видны в review.
