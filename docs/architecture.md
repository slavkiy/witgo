# Архитектура runtime

`witgo` строится вокруг простой границы доверия:

- Go host и `witgo` core считаются доверенными;
- Rust bridge считается доверенным системным слоем;
- WebAssembly Component plugin считается недоверенным кодом;
- policy и capability-решения всегда принимаются на стороне host.

## Как устроен вызов

1. Generated bindings проверяют контракт плагина до запуска.
2. Go runtime открывает доверенный native bridge.
3. Rust bridge поднимает Wasmtime и инстанцирует component graph.
4. Вызов export идёт через typed Go API.
5. Если plugin вызывает import, host маршрутизирует его либо в Go provider,
   либо в другой зарегистрированный plugin.

Для consumer-плагина другой plugin выглядит как обычный WIT import. Он не видит
registry, runtime options, bridge internals или чужие файлы.

## Contract, package и Go overlay

WIT остаётся единственным wire-контрактом. Несколько `.wit` одного package
детерминированно объединяются до lowering. Каталог `deps/` считается границей
другого package и не смешивается с корневым package в `GeneratePackage`.

Необязательный Go overlay применяется после WIT lowering. Он меняет публичные
Go-типы, но не structural signatures, manifest или Component ABI. Generator
создаёт private wire types и прямые lower/lift adapters без runtime registry и
reflection в call hot path.

## Host и guest generation

Один WIT world используется двумя независимыми generated surfaces:

```text
WIT world
├── GenerateHost  → Open/Validate/Check, typed clients, composition
└── GenerateGuest → Guest interfaces, Export<World>, typed Imports
                    ↓
                 Canonical ABI glue (wit-bindgen-go)
                    ↓
                 TinyGo wasip2 + Component Type metadata
                    ↓
                 plugin.component.wasm
```

Host package работает поверх `witgo.Runtime` и native bridge. Guest package не
может и не должен загружать Components: `OpenPlugin` там отсутствует. Его
`Export<World>` только связывает пользовательскую Go-реализацию с generated
Canonical ABI entry points.

`ExecutionRolePlugin` не участвует в lowering/lifting и componentization. Он
лишь сообщает, что текущая сборка имеет `GOARCH=wasm`; полноценный ABI создаёт
guest generator, а metadata и итоговый Component - TinyGo `wasip2` build.

## Composition model

Прозрачная композиция держится на `Host`, который хранит registry providers и
маршрутизирует вызовы по полному WIT ID. Короткие имена не участвуют в
разрешении.

Каждая top-level загрузка создаёт независимую runtime-box:

- свой набор instances;
- свой Store;
- свои handle;
- свой call chain и лимиты.

`resource`, `future`, `stream` и `error-context` можно безопасно передавать
только внутри одной такой коробки. Между независимыми runtime они не
переносятся.

## Runtime system API

Системный vendor interface называется `witgo:runtime/runtime@1.0.0`.

Он не меняет пользовательский `.wit` автоматически и подключается только через
явный opt-in:

```go
witgo.Config{EnableRuntimeAPI: true}
```

Когда API включён, guest получает только call-local возможности:

- `CallInfo`
- `FuelInfo`
- `Limits`
- `IsCancelled`
- `UnsafeRequestAdditionalFuel`

Первые четыре операции read-only. `UnsafeRequestAdditionalFuel` специально
назван опасно, потому что plugin не управляет fuel сам, а только просит host
рассмотреть запрос.

Plugin не получает доступ к:

- `Runtime.SetFuel`
- `Runtime.Close`
- registry
- `RuntimeOptions`
- `BridgePath`
- raw `Runtime.Call`
- native handle bridge
- host-only административным API

## Fuel и policy

Fuel budget остаётся host-controlled ресурсом.

- `Fuel` задаёт budget Store.
- `FuelPerCall` сбрасывает budget на каждый export call.
- guest не может сам увеличить budget.
- дополнительные grants проходят через `FuelRequestPolicy`.
- default policy - `DenyFuelRequests`.

Даже при включённом системном API host может:

- полностью отклонить запрос;
- выдать меньше запрошенного;
- выдать `0` через отказ;
- остановить запрос по timeout, deadline или лимитам.

## Почему bridge остаётся нативным

Rust-модуль здесь не обычный plugin. Это доверенный bridge-слой, который:

- управляет Wasmtime;
- собирает component composition graph;
- держит Store и linker;
- контролирует fuel, timeout и limits;
- реализует handshake и protocol features.

Поэтому переводить bridge целиком в guest wasm-plugin нельзя без разрушения
границы доверия. В wasm можно выносить только непривилегированную прикладную
логику, но не сам системный runtime bridge.

## Delivery model

`witgo` работает in-process и не требует sidecar-процесса.

- обычный Go использует `purego` loader без CGO;
- TinyGo использует CGo-based dynamic loader на Linux и Windows;
- TinyGo 0.41 на macOS остаётся generation-only из-за невозможности связать
  системные `dlopen` symbols;
- встроенные bridge-библиотеки поставляются для Linux, macOS и Windows на
  `amd64` и `arm64`;
- при запуске используется строгий version handshake без fallback-режима.

Если нужен внешний bridge, host может закрепить его через `BridgePath` и
`BridgeSHA256` или полностью запретить встроенный artifact через
`DisableEmbeddedBridge`.
