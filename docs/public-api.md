# Public API

## Runtime

`LoadRuntime` и `LoadRuntimeFromBytes` загружают только стандартные WebAssembly
Components. Core module возвращает `ErrCoreModule`.

```go
func LoadRuntime(filename string) (*Runtime, error)
func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error)
func LoadRuntimeWithImports(filename string, options RuntimeOptions, imports []HostImport) (*Runtime, error)

func LoadRuntimeFromBytes(data []byte) (*Runtime, error)
func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error)
func LoadRuntimeFromBytesWithImports(data []byte, options RuntimeOptions, imports []HostImport) (*Runtime, error)
```

`LoadRuntimeFromBytes*` сохраняет Component во временный файл, потому что
Wasmtime bridge загружает его по пути. `Runtime.Close` завершает bridge и удаляет
этот файл.

```go
func (r *Runtime) Call(name string, args ...any) (any, error)
func (r *Runtime) Close() error
func (r *Runtime) FuelRemaining() (uint64, error)
func (r *Runtime) SetFuel(fuel uint64) error
```

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
}
```

`Fuel` - общий остаток Store. `FuelPerCall` сбрасывает budget перед каждым
export call. `Timeout` прерывает Wasm epoch interrupt, но не блокирующий host
callback. Memory и instance limits применяются внутри Store. `MaxResultBytes`
ограничивает сообщения постоянного Go↔bridge канала.

`BridgePath` нужен для разработки; release обычно использует embedded binary.

## Errors

- `ErrCoreModule` - передан core module вместо Component.
- `ErrFuelDisabled` - fuel не был включён.
- `ErrFuelExhausted` - Wasmtime остановил call по fuel.
- `ErrCallTimeout` - epoch deadline остановил call.
- `ErrResultTooLarge` - сообщение превысило configured limit.
- `ExecutionLimitError` и `FuelDisabledError` сохраняют исходную причину через
  `Unwrap` и поддерживают `errors.Is`.

## Совместимость

`WitgoCtx` является alias `Runtime`; `NewEngine` и `NewEngineFromBytes` оставлены
как короткие compatibility wrappers. Старые `ModuleRuntime`, `ComponentRuntime`
и публичные объекты `wasmtime-go` удалены. `ReadMemory` оставлен только для
понятной migration error: Component values не читаются через custom memory ABI.
