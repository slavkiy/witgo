# Сгенерированный Go-код

Источник WIT выбирается явно:

```go
witgo.GenerateFile(config, "plugin.wit")
witgo.GenerateFiles(config, "types.wit", "plugin.wit")
witgo.GeneratePackage(config, "./wit")
witgo.GenerateTree(config, "./wit")
```

`GeneratePackage` читает все `.wit` непосредственно в каталоге. Вложенные
каталоги, включая `deps/`, считаются границами других packages. `GenerateTree`
обходит дерево рекурсивно, поэтому все найденные файлы должны объявлять один WIT
package. Порядок переданных файлов не влияет на output.

Необязательные Go-specific mappings описаны отдельно в
[go-overlays.md](go-overlays.md); WIT-файлы при этом не изменяются.

Для каждого WIT interface создаются единый Go interface, `InterfaceDescriptor`,
typed provider client, `Register…`, `Resolve…`, `AutoResolve…` и `MustResolve…`.
Один interface используется для Go implementation, component export и import
bindings. Для world дополнительно генерируются `…Bindings`, `AutoBind…` и
`Open…WithHost`; подробный сценарий находится в
[plugin-composition.md](plugin-composition.md).

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
	ProcessString(ctx context.Context, value string) (string, error)
}

type Metadata interface {
	Get(ctx context.Context) (Info, error)
}

type Plugin struct {
	Metadata Metadata
}

type PluginImports struct {
	Host Host
}

func PluginPing() witgo.Contract
func ValidatePlugin(filename string) (witgo.ValidationReport, error)
func ValidatePluginContext(ctx context.Context, filename string) (witgo.ValidationReport, error)
func ValidatePluginWithOptions(filename string, options witgo.RuntimeOptions) (witgo.ValidationReport, error)
func ValidatePluginWithOptionsContext(ctx context.Context, filename string, options witgo.RuntimeOptions) (witgo.ValidationReport, error)
func CheckPlugin(filename string) error
func CheckPluginContext(ctx context.Context, filename string) error
func CheckPluginWithOptions(filename string, options witgo.RuntimeOptions) error
func CheckPluginWithOptionsContext(ctx context.Context, filename string, options witgo.RuntimeOptions) error
func OpenPlugin(filename string, imports PluginImports) (*Plugin, error)
func OpenPluginContext(ctx context.Context, filename string, imports ...PluginImports) (*Plugin, error)
func OpenPluginWithOptions(filename string, options witgo.RuntimeOptions, imports PluginImports) (*Plugin, error)
func OpenPluginWithOptionsContext(ctx context.Context, filename string, options witgo.RuntimeOptions, imports PluginImports) (*Plugin, error)
func (p *Plugin) Close() error
func (p *Plugin) Restart() error
func (p *Plugin) RestartContext(ctx context.Context) error
```

Поля `PluginImports` могут быть `nil`. В этом случае runtime автоматически ищет вложенный component, экспортирующий требуемый WIT interface. Переданная вручную реализация всегда имеет приоритет.

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
Store, может быть передан обратно в ту же runtime-коробку и имеет идемпотентный
`Close`. Если generated binding получает WebAssembly provider из `witgo.Host`,
он автоматически передаёт bridge точный граф same-Store композиции. Bridge и
Component Model проверяют корректность ownership. Для `future` и `stream`
высокоуровневое чтение/запись typed payload пока не генерируется.

Экспорт вызывается обычным типизированным методом:

```go
info, err := plugin.Metadata.Get(ctx)
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
