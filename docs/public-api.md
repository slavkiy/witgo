# Публичный API

## Роль сборки

Роль выбирается компилятором, ручная настройка не нужна:

```go
role := witgo.CurrentExecutionRole()
if witgo.IsHostBuild() {
	// Нативное приложение: доступны Runtime и PluginHost.
}
if witgo.IsPluginBuild() {
	// GOARCH=wasm: код является Component Model плагином.
}
```

`RequireHostBuild` и `RequirePluginBuild` позволяют явно проверить назначение
entry point. Host-only вызов из WASM возвращает `ErrHostOnlyAPI`, plugin-only
вызов из нативного приложения - `ErrPluginOnlyAPI`.

## Composition host

```go
host, err := witgo.NewHost(witgo.HostOptions{
	MaxCallDepth: 16,
	RejectCycles: true,
	CallTimeout:  5 * time.Second,
})
```

Core API включает `InterfaceDescriptor`, `ProviderHandle`, `RegisterProvider`,
`ResolveProvider`, `AutoResolveProvider`, `UnregisterProvider`, `ReplaceProvider`,
`PluginCallContext`, `PluginCallError`, `PluginDependencyError` и `CallObserver`.
Обычному приложению рекомендуется использовать generated typed helpers, а не
низкоуровневый `ProviderCall`.

Для same-Store композиции доступны `ComponentComposition`, `CompositionPlug`,
`ComponentProvider` и `Runtime.Composition`. Обычно их вызывает generated API:
при регистрации export-client граф сохраняется в `ProviderHandle`, а при
открытии потребителя превращается в `RuntimeOptions.CompositionPlugs`. Ручное
заполнение требуется только пользователям dynamic API. `CompositionPlug.Interface`
всегда содержит полный WIT ID, а не короткое имя.

Сводная таблица поддерживаемых WIT-конструкций и runtime-ограничений находится
в [docs/capabilities.md](capabilities.md).

## Runtime

`LoadRuntime` и `LoadRuntimeFromBytes` загружают только стандартные WebAssembly
Components. Core module возвращает `ErrCoreModule`.

```go
func LoadRuntime(filename string) (*Runtime, error)
func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error)
func LoadRuntimeWithImports(filename string, options RuntimeOptions, imports []HostImport) (*Runtime, error)
func LoadRuntimeWithContract(filename string, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error)

func LoadRuntimeFromBytes(data []byte) (*Runtime, error)
func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error)
func LoadRuntimeFromBytesWithImports(data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error)
func LoadRuntimeFromBytesWithContract(data []byte, options RuntimeOptions, imports []HostImport, contract Contract) (*Runtime, error)

func InspectComponent(filename string) (Contract, error)
func InspectComponentWithOptions(filename string, options RuntimeOptions) (Contract, error)
func InspectComponentBytes(data []byte) (Contract, error)
func InspectComponentBytesWithOptions(data []byte, options RuntimeOptions) (Contract, error)
func ValidateComponent(filename string, expected Contract) (ValidationReport, error)
func ValidateComponentWithOptions(filename string, options RuntimeOptions, expected Contract) (ValidationReport, error)
func ValidateComponentBytes(data []byte, expected Contract) (ValidationReport, error)
func ValidateComponentBytesWithOptions(data []byte, options RuntimeOptions, expected Contract) (ValidationReport, error)
func RequireCompatible(report ValidationReport) error
func CompareContracts(expected, actual Contract) (ValidationReport, error)
```

Generated bindings используют `LoadRuntimeWithContract`. Runtime выполняет
versioned `ping`/`pong` handshake с native bridge и сравнивает отсортированные
имена функций до разрешения вызовов. Несовпадение оборачивается в
`ErrBridgeProtocolMismatch`, `ErrBridgeVersionMismatch` или
`ErrContractMismatch`, поэтому можно использовать `errors.Is`.

Inspection и validation останавливаются после contract `pong`. Они не отправляют
`start`, не instantiate-ят component, не линкуют host callbacks и не запускают
guest code. Несовместимый component даёт
`ValidationReport{Compatible:false}` с missing/unexpected imports/exports.
Операционные сбои и version mismatch bridge возвращаются как `error`.

`Contract` даёт безопасные отсортированные accessors и lookup helpers:

```go
imports := manifest.ImportNames()
exports := manifest.ExportNames()
all := manifest.FunctionNames()
required := manifest.Requires("example:plugins/host@1.0.0#read")
provided := manifest.Provides("example:plugins/api@1.0.0#run")
signature, ok := manifest.Signature("example:plugins/api@1.0.0#run")
```

`ValidationReport.Err()` возвращает `nil` для совместимого компонента и
`*ContractValidationError` в противном случае. `Summary()` даёт детерминированный
текст, а `ProblemCount()` считает name и signature mismatches.

`LoadRuntimeFromBytes*` сохраняет Component во временный файл, потому что
Wasmtime bridge загружает его по пути. `Runtime.Close` завершает bridge и
удаляет этот файл.

```go
func (r *Runtime) Call(name string, args ...any) (any, error)
func (r *Runtime) Close() error
func (r *Runtime) IsClosed() bool
func (r *Runtime) FuelRemaining() (uint64, error)
func (r *Runtime) SetFuel(fuel uint64) error
```

Живые Component Model handles используют один runtime-bound тип:

```go
type Handle struct { /* unexported lifecycle state */ }

func (h Handle) ID() uint64
func (h Handle) Kind() HandleKind
func (h Handle) IsKind(kind HandleKind) bool
func (h Handle) Valid() error
func (h Handle) Owned() bool
func (h Handle) IsClosed() bool
func (h Handle) Close() error
func CloseHandles(handles ...Handle) error
```

`HandleKind` принимает одно из значений `HandleResource`, `HandleFuture`,
`HandleStream` или `HandleErrorContext`. Handle можно отправлять обратно только
в ту же runtime-коробку, которая его создала. WebAssembly-провайдеры внутри
коробки связываются в одном Store автоматически. `Close` вызывает `resource_drop`,
закрывает future/stream или удаляет `error-context` token.

Helper-типы, которые активно используются generated bindings:

- `Option[T]`: `Some`, `None`, `OptionFromPointer`, `Get`, `Or`, `Pointer`,
  `MapOption`, `FlatMapOption`;
- `Result[T,E]`: `Ok`, `Err`, `GetOK`, `GetErr`, `Or`, `MatchResult`,
  `MapResult`, `MapResultErr`;
- `Char`: `NewChar`, `ParseChar`, `Rune`, `String`;
- `Tuple0`...`Tuple16`: `NewTupleN`, типизированные поля `V0...`, `Values` и
  строгий array codec; большие tuple используют `Tuple`, `At`, `Set`,
  `TupleValue`;
- `Map[K,V]`: `NewMap`, `Get`, `Put`, `Delete`, `Clone` и строгий pair codec.

Имя interface export имеет форму
`namespace:package/interface@version#function`. Direct world functions
передают только имя функции.

## Host capabilities

```go
type HostFunc func(args []any) (any, error)
type HostFuncContext func(ctx context.Context, args []any) (any, error)

type HostImport struct {
	Interface string
	Function  string
	Call      HostFunc
	CallContext HostFuncContext
}
```

Linker регистрирует только переданный список. Повторяющийся import, пустое имя
или `nil` callback считаются ошибкой. Generated package скрывает этот
низкоуровневый API за типизированным Go interface.

Rule-based capability filtering:

```go
type CapabilityPolicy struct {
	Allow []string
	Deny  []string
}

func InspectRequiredCapabilities(filename string) ([]string, error)
func InspectRequiredCapabilitiesWithOptions(filename string, options RuntimeOptions) ([]string, error)
func (p CapabilityPolicy) Allows(function string) bool
func (p CapabilityPolicy) ValidateImports(imports []string) error
```

`RuntimeOptions.Capabilities` применяет policy к фактическим component imports
до `start`. Pattern может быть полным именем функции, именем interface без `#`,
prefix wildcard с `*` или `"*"`.

## RuntimeOptions

Backend native bridge выбирается во время сборки: стандартный Go использует `purego` без CGO, TinyGo - системный CGo-loader. Оба варианта работают через тот же публичный API и проверяют одинаковый version handshake.

```go
type RuntimeOptions struct {
	Fuel                  uint64
	FuelPerCall           uint64
	Timeout               time.Duration
	MemoryLimitBytes      int64
	MaxResultBytes        uint64
	InstanceLimit         int64
	BridgePath            string
	BridgeSHA256          string
	DisableEmbeddedBridge bool
	Capabilities          CapabilityPolicy
	NestedPlugins         NestedPluginOptions
	PluginHost            *PluginHost
	CompositionHost       *Host
}
```

Для вложенных зависимостей основной механизм - манифест, принадлежащий плагину:

```go
type PluginManifest struct {
	Dependencies map[string]string
}

func EmbedPluginManifest(component []byte, manifest PluginManifest) ([]byte, error)
func ReadPluginManifest(filename string) (PluginManifest, bool, error)
```

Пути в манифесте относительны компоненту. `NestedPluginOptions.AllowedRoots`
задаёт верхнюю границу файловой политики хоста. `SearchPaths` и `Resolver`
используются только как fallback для старых компонентов без манифеста.

`PluginHost` хранит общую политику автоматического разрешения imports. Каждый top-level load создаёт независимую `PluginBox`; глубина ациклических зависимостей не ограничивается.

`Fuel` задаёт общий budget Store. `FuelPerCall` сбрасывает budget перед каждым
export call. `Timeout` прерывает Wasm через epoch interrupt, но не останавливает
зависший host callback. Memory и instance limits применяются внутри Store.
`MaxResultBytes` ограничивает сообщения внутреннего Go-bridge канала.

`BridgePath` может указывать на заранее установленную shared library; по
умолчанию используется библиотека, уже встроенная в Go module. `BridgeSHA256`
закрепляет явный `BridgePath`. `DisableEmbeddedBridge` гарантирует, что
встроенная копия не будет распакована.

Подробный порядок поиска и env-переменные описаны в
[docs/architecture.md](architecture.md).

## Ошибки

- `ErrRuntimeClosed`: runtime уже закрыт.
- `ErrBridgeProtocolMismatch`: version handshake или обязательные feature-флаги
  не совпали.
- `ErrBridgeVersionMismatch`: версия native bridge не совпала с версией Go
  package.
- `ErrContractMismatch`: imports, exports или structural signatures отличаются.
- `ErrHandleClosed`: handle уже передан, закрыт или неизвестен этому `Runtime`.
- `ErrCoreModule`: вместо Component передан core module.
- `ErrFuelDisabled`: fuel metering не был включён.
- `ErrFuelExhausted`: Wasmtime остановил вызов по fuel.
- `ErrCallTimeout`: epoch deadline остановил вызов.
- `ErrResultTooLarge`: protocol message превысило настроенный лимит.
- `ExecutionLimitError` и `FuelDisabledError` сохраняют исходную причину через
  `Unwrap` и поддерживают `errors.Is`.
