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
- TinyGo использует CGo-based dynamic loader;
- встроенные bridge-библиотеки поставляются для Linux, macOS и Windows на
  `amd64` и `arm64`;
- при запуске используется строгий version handshake без fallback-режима.

Если нужен внешний bridge, host может закрепить его через `BridgePath` и
`BridgeSHA256` или полностью запретить встроенный artifact через
`DisableEmbeddedBridge`.
