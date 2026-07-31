# Public API

Сводная таблица поддерживаемых WIT-конструкций и runtime-ограничений находится
в [«Возможности и ограничения»](capabilities.md).

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

Generated bindings use `LoadRuntimeWithContract`. The runtime performs a
versioned `ping`/`pong` handshake with the native bridge and compares sorted
function names before allowing calls. A mismatch wraps
`ErrBridgeProtocolMismatch`, `ErrBridgeVersionMismatch`, or
`ErrContractMismatch`, so callers can use `errors.Is`.

Inspection and validation stop after the bridge's contract `pong`; they do not
send `start`, instantiate the component, link host callbacks, or execute guest
code. A non-compatible component produces `ValidationReport{Compatible:false}`
with missing and unexpected imports/exports. Operational failures and bridge
version mismatches are returned as `error`. Structural signatures cover
primitive values and nested records, lists, options, results, tuples, enums,
flags and variants.

`Contract` provides defensive, sorted accessors and lookup helpers:

```go
imports := manifest.ImportNames()
exports := manifest.ExportNames()
all := manifest.FunctionNames()
required := manifest.Requires("example:plugins/host@1.0.0#read")
provided := manifest.Provides("example:plugins/api@1.0.0#run")
signature, ok := manifest.Signature("example:plugins/api@1.0.0#run")
```

`ValidationReport.Err()` returns nil for a compatible component and a
`*ContractValidationError` otherwise. `Summary()` provides deterministic text,
and `ProblemCount()` counts name and signature mismatches.

`LoadRuntimeFromBytes*` сохраняет Component во временный файл, потому что
Wasmtime bridge загружает его по пути. `Runtime.Close` завершает bridge и удаляет
этот файл.

```go
func (r *Runtime) Call(name string, args ...any) (any, error)
func (r *Runtime) Close() error
func (r *Runtime) IsClosed() bool
func (r *Runtime) FuelRemaining() (uint64, error)
func (r *Runtime) SetFuel(fuel uint64) error
```

Live Component Model handles use one runtime-bound value:

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

`HandleKind` is one of `HandleResource`, `HandleFuture`, `HandleStream`, or
`HandleErrorContext`. A handle can only be sent back to the Runtime that
created it. `Close` invokes `resource_drop`, closes a future/stream, or removes
an error-context token. Future/stream payload reader and writer operations are
not part of the dynamic API yet.

Value helpers used by generated bindings:

- `Option[T]`: `Some`, `None`, `OptionFromPointer`, `Get`, `Or`, `Pointer`,
  `MapOption`, `FlatMapOption`;
- `Result[T,E]`: `Ok`, `Err`, `GetOK`, `GetErr`, `Or`, `MatchResult`,
  `MapResult`, `MapResultErr`;
- `Char`: `NewChar`, `ParseChar`, `Rune`, `String`;
- `Tuple0` ... `Tuple16`: `NewTupleN`, typed `V0...` fields, `Values` and
  strict array codecs.

Имя interface export имеет вид
`namespace:package/interface@version#function`. Direct world functions передают
только имя функции.

## Host capabilities

```go
type HostFunc func(args []any) (any, error)

type HostImport struct {
	Interface string
	Function  string
	Call      HostFunc
}
```

Linker регистрирует только переданный список. Повторяющийся import, пустое имя
или nil callback являются ошибкой. Generated package скрывает этот
низкоуровневый API за типизированным Go interface.

## RuntimeOptions

```go
type RuntimeOptions struct {
	Fuel             uint64
	FuelPerCall      uint64
	Timeout          time.Duration
	MemoryLimitBytes int64
	MaxResultBytes   uint64
	InstanceLimit    int64
	BridgePath       string
	BridgeSHA256     string
	DisableEmbeddedBridge bool
}
```

`Fuel` - общий остаток Store. `FuelPerCall` сбрасывает budget перед каждым
export call. `Timeout` прерывает Wasm epoch interrupt, но не блокирующий host
callback. Memory и instance limits применяются внутри Store. `MaxResultBytes`
ограничивает сообщения постоянного Go↔bridge канала.

`BridgePath` может указывать на заранее установленную и подписанную shared
library; по умолчанию используется библиотека, уже содержащаяся в Go module.

`BridgeSHA256` pins an explicit `BridgePath`.
`DisableEmbeddedBridge` guarantees that a distribution-provided embedded
library is not extracted.
See [Runtime architecture](architecture.md) for environment-variable controls
and the complete resolution order.

## Errors

- `ErrRuntimeClosed` - runtime is already closed.
- `ErrBridgeProtocolMismatch` - protocol version or a required feature differs.
- `ErrBridgeVersionMismatch` - native bridge and Go package versions differ.
- `ErrContractMismatch` - imports, exports, or structural signatures differ.
- `ErrHandleClosed` - handle already transferred, closed, or unknown to this Runtime.

- `ErrCoreModule` - передан core module вместо Component.
- `ErrFuelDisabled` - fuel не был включён.
- `ErrFuelExhausted` - Wasmtime остановил call по fuel.
- `ErrCallTimeout` - epoch deadline остановил call.
- `ErrResultTooLarge` - сообщение превысило configured limit.
- `ExecutionLimitError` и `FuelDisabledError` сохраняют исходную причину через
  `Unwrap` и поддерживают `errors.Is`.
