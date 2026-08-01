# Прозрачная композиция плагинов

Любой сгенерированный WIT-интерфейс может быть реализован Go-кодом или другим
зарегистрированным WebAssembly Component. Плагин-потребитель вызывает обычный
WIT import и не знает, какой provider находится за ним.

## Путь вызова

```text
processor.wasm
  → обычный WIT import image-codec#decode
  → generated host adapter
  → witgo.Host и ProviderHandle
  → типизированный ImageCodec provider
  → codec.wasm export image-codec#decode
```

Rust bridge и Component Model ABI не изменяются. Router расположен на стороне
Go в уже существующей точке обработки `host_call`. Аргументы поднимаются в
generated Go types до обращения к provider и опускаются обратно после вызова.

## Единый интерфейс

Генератор создаёт один контракт и для import, и для export:

```go
type ImageCodec interface {
	Decode(
		ctx context.Context,
		data []byte,
	) (witgo.Result[Image, string], error)
}
```

Его может реализовать обычный объект:

```go
type localCodec struct{}

func (localCodec) Decode(
	ctx context.Context,
	data []byte,
) (witgo.Result[contract.Image, string], error) {
	return witgo.Ok[contract.Image, string](decode(data)), nil
}
```

Generated export-client компонента реализует тот же интерфейс. Поэтому bindings
потребителя не меняются при замене Go provider на WASM provider.

## Регистрация и явное связывание

```go
host, err := witgo.NewHost(witgo.HostOptions{
	MaxCallDepth: 16,
	RejectCycles: true,
	CallTimeout:  5 * time.Second,
})
if err != nil {
	return err
}
defer host.Close()

codec, err := contract.OpenCodecPluginWithHost(
	host,
	"./plugins/codec.component.wasm",
)
if err != nil {
	return err
}

err = contract.RegisterImageCodec(
	host,
	"default",
	codec.ImageCodec,
	witgo.OwnedProvider(codec.Close),
)
if err != nil {
	return err
}

imageCodec, err := contract.ResolveImageCodec(host, "default")
if err != nil {
	return err
}

processor, err := contract.OpenProcessorPluginWithHost(
	host,
	"./plugins/processor.component.wasm",
	contract.ProcessorPluginBindings{ImageCodec: imageCodec},
)
```

Для локальной реализации меняется только регистрация:

```go
err := contract.RegisterImageCodec(host, "default", localCodec{})
```

## Автоматическое связывание

```go
processor, err := contract.OpenProcessorPluginAutoBoundWithHost(
	host,
	"./plugins/processor.component.wasm",
)
```

`AutoBindProcessorPlugin` и auto-bound constructor работают только когда для
каждого import зарегистрирован ровно один совместимый provider. Ноль providers
возвращает `ErrPluginNotRegistered`, несколько - ошибку с отсортированным списком
имён. Явные `ProcessorPluginBindings` всегда имеют приоритет.

## Цепочка A → B → C

Каждый provider регистрируется независимо. Например, storage export связывается
с import codec, затем codec export - с import processor. Один `context.Context`
проходит через всю цепочку:

```text
host → processor#process → codec#decode → storage#read
```

`PluginCallContextFromContext` возвращает ID, parent ID, глубину, deadline и
call path. `CallObserver` получает события начала и завершения вне registry lock.
Паника observer изолируется и не повреждает runtime.

## Циклы и reentrancy

По умолчанию `MaxCallDepth` равен 32, а циклы запрещены. Router отслеживает как
provider path, так и identity активных Runtime. Повторный вход `A → B → A`
отклоняется с `ErrPluginCallCycle` до попытки захватить mutex Runtime A. Поэтому
обратный вызов не превращается в deadlock. Последовательные вызовы после возврата
из предыдущего вызова циклами не считаются.

## Lifecycle и замена

- `OwnedProvider(plugin.Close)` передаёт lifecycle хосту.
- Без этой опции provider остаётся внешне управляемым.
- `UnregisterProvider` запрещает новые вызовы, ждёт уже начатые и закрывает owned
  provider.
- `ReplaceProvider` сначала проверяет descriptor, атомарно публикует новую
  реализацию, затем дожидается старых вызовов и закрывает старый owned provider.
- `Host.Close` идемпотентно закрывает все owned providers.

Generated provider client хранит `ProviderHandle`, а не голую ссылку на Runtime,
поэтому unregister не оставляет небезопасный dangling client.

## Контракты и ошибки

`InterfaceDescriptor` содержит полный package/interface/version ID и structural
signatures всех функций. `ResolveProvider` сравнивает весь descriptor до
instantiation потребителя. Несовместимость возвращается как
`PluginDependencyError`, поддерживающий `errors.Is` с
`ErrPluginDependencyMismatch`.

Вложенная ошибка оборачивается в `PluginCallError`, сохраняет исходную причину и
полный path. Поэтому классификация `ErrFuelExhausted`, `ErrCallTimeout` и других
ошибок продолжает работать через `errors.Is`.

## Fuel, timeout и память

- Fuel и память локальны одной runtime-коробке. Все скомпонованные WebAssembly
  instances расходуют общий budget Store.
- Go host callback сам fuel не расходует.
- Deadline наследуется по цепочке и никогда не продлевается.
- Effective deadline - минимум уже существующего deadline, `HostOptions.CallTimeout`
  и timeout конкретного Runtime.
- `MaxResultBytes` проверяется bridge каждого Runtime.
- Зависший Go callback нельзя принудительно остановить; он обязан соблюдать
  переданный `context.Context`.

## Handles

Primitive values, records, lists, nested lists, options, results, tuples, maps,
enums, flags и variants проходят через обычный typed codec. `resource`, `future`,
`stream` и `error-context` передаются между WebAssembly-провайдером и
WebAssembly-потребителем через композицию в одном Wasmtime Store. Генерируемый
API автоматически извлекает граф provider, а bridge связывает точный WIT import
с точным export до instantiation. Поэтому ownership и resource identity
проверяет Component Model, а handle вообще не сериализуется через Go.

Если provider реализован на Go, остаётся callback-маршрут. Живой handle нельзя
безопасно передать этим маршрутом или между двумя уже созданными Runtime: такая
попытка по-прежнему возвращает `ErrCrossRuntimeHandle`. Fake handle и копирование
числового ID не используются.

Идентичность ребра - полный ID `namespace:package/interface@version`. Два
интерфейса с одинаковым коротким именем не смешиваются. Два provider для одного
точного ID, несовместимый тип, отсутствующий export и цикл графа отклоняются до
запуска гостевого кода.

## Модель безопасности

Композиция не даёт плагину filesystem, network или registry. Плагин не загружает
`.wasm`, не выбирает provider и не получает ссылку на чужой Runtime. Он может
вызвать другой компонент только когда Go-host явно связал его WIT import с
зарегистрированным совместимым export. Capability policy каждого Runtime
применяется как раньше.
