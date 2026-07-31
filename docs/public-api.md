# Public API

Этот документ описывает только публичный API корневого пакета `witgo`.
Внутренние пакеты, парсер и неэкспортируемые сущности сюда не входят.

## Config

`Config` хранит базовые параметры для работы `witgo`.

Поля:

- `WIT` - путь к директории с WIT-файлами.
- `Output` - путь для выходных артефактов.
- `Package` - имя целевого пакета.
- `Filename` - имя generated-файла; по умолчанию `bindings.gen.go`.

## Generate

```go
func Generate(config Config) error
```

Рекурсивно читает `.wit` из `Config.WIT`, строит IR и атомарно записывает
отформатированные Go bindings в `Config.Output`.

## NewGenerator

```go
func NewGenerator(config Config) (*Generator, error)
```

Создаёт проверенный reusable-генератор. Вызов `Generator.Generate()` создаёт
или обновляет configured generated-файл.

## Open

Сигнатура:

```go
func Open(config *Config) (*WitgoCtx, error)
```

`Open` - легаси-точка входа для открытия конфигурации пакета.
Сейчас функция использует `Config` и подготавливает загрузку пакета через корневой API.

Использовать в новом коде стоит осторожно: это старый интерфейс, оставленный для совместимости.

## LoadRuntime

Сигнатура:

```go
func LoadRuntime(filename string) (*Runtime, error)
```

`LoadRuntime` читает WebAssembly-бинарник с диска, определяет его тип и создает `Runtime`.

Поддерживаемые сценарии:

- загрузка core WebAssembly module;
- загрузка WebAssembly component.

Функция возвращает ошибку, если:

- путь не удалось нормализовать;
- файл не удалось прочитать;
- входные данные не являются корректным wasm-бинарником;
- тип wasm не удалось определить;
- модуль или компонент не удалось скомпилировать или инстанцировать.

## LoadRuntimeFromBytes

Сигнатура:

```go
func LoadRuntimeFromBytes(data []byte) (*Runtime, error)
```

`LoadRuntimeFromBytes` делает то же самое, что и `LoadRuntime`, но принимает бинарные данные из памяти, а не путь к файлу.

Подходит для случаев, когда wasm уже был считан заранее или получен по сети, из архива или из embed-ресурса.

## RuntimeOptions и ограничения

```go
type RuntimeOptions struct {
	Fuel            uint64
	FuelPerCall     uint64
	Timeout         time.Duration
	MemoryLimitBytes int64
	MaxResultBytes  uint64
	InstanceLimit   int64
}

func LoadRuntimeWithOptions(filename string, options RuntimeOptions) (*Runtime, error)
func LoadRuntimeFromBytesWithOptions(data []byte, options RuntimeOptions) (*Runtime, error)
```

`FuelPerCall` сбрасывает бюджет перед каждым вызовом; `Fuel` задаёт один
накопительный бюджет на весь runtime. Эти поля взаимоисключающие. При
исчерпании `errors.Is(err, ErrFuelExhausted)` истинно.

`Timeout` включает epoch interruption и распознаётся через `ErrCallTimeout`.
Он прерывает Wasm, но не блокирующий Go host import. `MemoryLimitBytes`
ограничивает каждую линейную память, `InstanceLimit` - число инстансов в store,
а `MaxResultBytes` - объём, который `ReadMemory` разрешит скопировать из Wasm.
Превышение последнего распознаётся через `ErrResultTooLarge`.

Ошибки fuel и timeout представлены `*ExecutionLimitError`: `errors.Is`
распознаёт соответствующий sentinel, а `errors.As` по цепочке `Unwrap` всё ещё
может получить исходный `*wasmtime.Trap`.

```go
func (r *Runtime) FuelRemaining() (uint64, error)
func (r *Runtime) SetFuel(fuel uint64) error
```

`FuelRemaining` читает остаток, `SetFuel` заменяет его. Если runtime был открыт
без fuel, оба метода возвращают `*FuelDisabledError`. Для него одновременно
работают `errors.Is(err, ErrFuelDisabled)`, `errors.As` и `errors.Unwrap`, поэтому
исходная ошибка Wasmtime не теряется.

Ни одна из этих настроек не является полной sandbox-моделью: host-память,
filesystem/network и поведение host imports контролируются отдельно через
capability-модель приложения.

## Runtime.Call

Сигнатура:

```go
func (r *Runtime) Call(name string, args ...interface{}) (interface{}, error)
```

`Runtime.Call` вызывает экспортированную функцию загруженного WebAssembly core module.

Особенности:

- если `Runtime` не инициализирован, вернется ошибка;
- если внутри загружен component, а не core module, вернется ошибка;
- если экспорт с таким именем не найден, вернется ошибка;
- аргументы передаются напрямую в `wasmtime-go`.

## ModuleRuntime.Call

Сигнатура:

```go
func (mr *ModuleRuntime) Call(name string, args ...interface{}) (interface{}, error)
```

`ModuleRuntime.Call` вызывает экспортированную функцию напрямую на уровне инстанса wasm-модуля.

Это более низкоуровневый API по сравнению с `Runtime.Call`, когда вызывающему коду нужен прямой доступ именно к module runtime.

## NewEngine

Сигнатура:

```go
func NewEngine(filename string) (*WitgoCtx, error)
```

`NewEngine` - легаси-обертка над `LoadRuntime`.

Функция оставлена для обратной совместимости и возвращает `WitgoCtx`, а не новый `Runtime`.
Для нового кода предпочтительнее использовать `LoadRuntime`.

## NewEngineFromBytes

Сигнатура:

```go
func NewEngineFromBytes(data []byte) (*WitgoCtx, error)
```

`NewEngineFromBytes` - легаси-обертка над `LoadRuntimeFromBytes`.

Как и `NewEngine`, она нужна в первую очередь для совместимости со старым кодом.

## WitgoCtx.Call

Сигнатура:

```go
func (wc *WitgoCtx) Call(name string, args ...interface{}) (interface{}, error)
```

`WitgoCtx.Call` - легаси-версия вызова экспортированной функции.

Поведение аналогично `Runtime.Call`, но работает через старый тип `WitgoCtx`.
Для нового кода предпочтительнее использовать `Runtime`.

## Legacy Types

Следующие публичные типы относятся к старому API и сохранены ради совместимости:

- `WitgoCtx`
- `ModuleCtx`
- `ComponentCtx`

Новые аналоги:

- `Runtime` вместо `WitgoCtx`
- `ModuleRuntime` вместо `ModuleCtx`
- `ComponentRuntime` вместо `ComponentCtx`

## Runtime Types

`Runtime`, `ModuleRuntime` и `ComponentRuntime` содержат низкоуровневые объекты `wasmtime-go`.

Их публичные поля позволяют:

- получить доступ к `Store`;
- работать с инстансом модуля или компонента;
- при необходимости строить более низкоуровневую интеграцию поверх `witgo`.

Это мощный, но менее стабильный слой API, потому что он тесно завязан на структуру `wasmtime-go`.
