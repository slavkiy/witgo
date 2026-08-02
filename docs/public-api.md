# Публичный API

Этот документ фиксирует только те части API, которые важно знать большинству
пользователей `witgo`.

## Что считается основным API

- `Generate`, `GenerateFile`, `GenerateFiles`, `GeneratePackage`, `GenerateTree`
  и `Config` для генерации typed bindings;
- `GenerateHost` и `GenerateGuest` для явного выбора стороны Component Model;
- generated host API: `Open*`, `Validate*`, `Check*` и typed interfaces;
- generated guest API: `<Interface>Guest`, `<World>Guest`, `Export<World>` и
  `Imports`;
- `BuildGuestComponent` и `GuestBuildConfig` для TinyGo `wasip2` build;
- `BuildPlugin` и `PluginBuildConfig` для generation + build + manifest;
- `Host` для composition routing;
- `RuntimeOptions` для лимитов и policy;
- `HostPolicy`, `Permissions`, `PluginGrant` и `PluginLimits` для обычной
  настройки прав и fuel по plugin ID;
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

### Guest generator

```go
witgo.Generate(witgo.Config{
	WIT:         "./wit",
	WITMode:     witgo.WITInputPackage,
	Mode:        witgo.GenerateGuest,
	World:       "plugin",
	Output:      "./internal/contract",
	Package:     "contract",
	PackageRoot: "example.com/plugin/internal/contract",
})
```

Поля guest-конфигурации:

- `Mode` - `GenerateHost` по умолчанию или `GenerateGuest`;
- `World` - имя реализуемого world; обязательно при нескольких worlds;
- `PackageRoot` - import path каталога `Output`; обычно выводится из ближайшего
  `go.mod`;
- `GuestBindgen` - необязательный путь к `wit-bindgen-go`.

Guest mode не использует native Wasmtime loader и не генерирует `Open*`.
Низкоуровневые Canonical ABI declarations создаются `wit-bindgen-go`, а
сгенерированный root package предоставляет единый API регистрации реализации.

### Guest component build

```go
err := witgo.BuildGuestComponent(witgo.GuestBuildConfig{
	Main:       "./cmd/plugin",
	WITPackage: "./wit",
	World:      "plugin",
	Output:     "./dist/plugin.component.wasm",
	NoDebug:    true,
})
```

Команда запускает TinyGo с `-target=wasip2`, `--wit-package` и `--wit-world`.
`WITPackage` может указывать непосредственно на каталог WIT package; заранее
создавать отдельный packaged `.wasm` для обычного сценария не требуется.
TinyGo добавляет Component Type metadata и выдаёт готовый Component, а не core
Wasm module.

## RuntimeOptions

Для прикладного кода предпочтителен `HostPolicy.Options(pluginID)` или
generated `Open<World>WithPolicy`: public grant применяется ко всем плагинам,
а named grant добавляет права и переопределяет ненулевые лимиты. Нулевая
`HostPolicy` запрещает все ambient imports и nested plugin loading.

`Permissions.System`, `Network` и `Files` соответствуют WASI namespaces;
`Allow`/`Deny` задают произвольные WIT patterns. `LoadPlugin` разрешает
manifest-based dependency loading только внутри `AllowedPluginRoots`.

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
