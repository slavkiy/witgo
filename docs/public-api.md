# Публичный API

Этот документ фиксирует только те части API, которые важно знать большинству
пользователей `witgo`.

## Что считается основным API

- `Generate`, `GenerateFile`, `GenerateFiles`, `GeneratePackage`, `GenerateTree`
  и `Config` для генерации typed bindings;
- generated `Open*`, `Validate*`, `Check*` и typed interfaces;
- `Host` для composition routing;
- `RuntimeOptions` для лимитов и policy;
- `Handle` для runtime-bound Component handles.

Низкоуровневый `Runtime.Call` и ручная сборка `HostImport` остаются рабочими,
но для обычного кода предпочтителен generated typed слой.

## Generator API

Главный вход:

```go
witgo.Generate(witgo.Config{
	WIT:              "./wit",
	WITMode:          witgo.WITInputPackage,
	GoOverlay:        "./wit/plugin.witgo.yaml",
	Output:           "./internal/contract",
	Package:          "contract",
	EnableRuntimeAPI: true,
})
```

Что важно:

- generator не переписывает исходный `.wit`;
- `WITFiles` задаёт явный набор файлов, а `WITMode` различает file, package и
  recursive tree; `WIT` и `WITFiles` одновременно не используются;
- `GoOverlay` подключает versioned YAML mapping публичных Go-типов, не меняя
  wire contract; формат описан в [go-overlays.md](go-overlays.md);
- `EnableRuntimeAPI` только добавляет generated facade для vendor capability;
- generated package всегда знает свой contract manifest и использует его при `Open*`.

`TypeCodec[Wire, Go]` является публичной typed абстракцией для codec tooling.
Встроенные overlay codecs генерируются прямыми функциями без runtime reflection.

## RuntimeOptions

Главные опции runtime:

- `Fuel` или `FuelPerCall` для instruction budget;
- `Timeout` для остановки Wasm-вызова;
- `MemoryLimitBytes` и `InstanceLimit` для store limits;
- `MaxResultBytes` и `ValueLimits` для ограничения размера сообщений;
- `EnableRuntimeAPI` для guest system facade;
- `Capabilities` для allow/deny host imports;
- `FuelRequestPolicy` и `FuelRequestLimits` для контроля unsafe fuel requests;
- `PluginHost` и `CompositionHost` для nested/plugin composition;
- `BridgePath`, `BridgeSHA256`, `DisableEmbeddedBridge` для выбора bridge.

## Host API

`Host` нужен, когда один plugin должен безопасно вызывать другой plugin через
обычный WIT import.

Host отвечает за:

- registry providers;
- resolution по полному WIT ID;
- call depth и cycle protection;
- fuel-request policy и security observer;
- сохранение capability boundaries между plugin calls.

Обычному приложению редко нужен ручной `ProviderCall`: generated adapters
делают типизированную маршрутизацию сами.

## Handles

`Handle` - это единый тип для:

- `resource`
- `future`
- `stream`
- `error-context`

Это не сериализуемый бизнес-объект, а runtime-bound token. Его можно вернуть в
тот же runtime-box или передать вложенному plugin в том же Store, но нельзя
безопасно переносить между независимыми runtime.

## Ошибки, на которые стоит ориентироваться

- `ErrContractMismatch` - контракт не совпал.
- `ErrBridgeProtocolMismatch` или `ErrBridgeVersionMismatch` - host и bridge не совпали по протоколу.
- `ErrCapabilityDenied` - policy запретила импорт.
- `ErrFuelDisabled` - fuel metering не включён.
- `ErrRuntimeClosed` - runtime уже закрыт.
- `ErrHandleClosed` - handle недействителен для текущего runtime.
- `ErrFuelRequestDenied` и связанные denial reasons - host не одобрил unsafe fuel request.
- `WITError[E]` - typed payload ветки WIT `result` при включённом
  overlay `result_error`; transport error этим типом не подменяется.
