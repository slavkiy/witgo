# Возможности и ограничения

Здесь только практические границы текущего `witgo`.

## Платформы

Парсер и генератор являются переносимым Go-кодом. Для выполнения component нужен
Rust/Wasmtime bridge, собранный под конкретные OS и архитектуру.

- встроенный bridge: Linux, macOS и Windows на `amd64` и `arm64`;
- внешний `RuntimeOptions.BridgePath`: Linux, macOS, Windows и FreeBSD (FreeBSD
  требует CGo), если Rust и Wasmtime поддерживают выбранный target;
- TinyGo runtime поддерживает Linux и Windows с CGo. На macOS TinyGo 0.41 не
  позволяет связать `dlopen`; parser и generator при этом доступны. Без CGo
  динамический bridge не загружается.

На остальных Go/TinyGo targets доступны parsing и generation, но runtime требует
добавить загрузчик динамической библиотеки и Rust/Wasmtime build для этого target.
Android и iOS намеренно относятся к generation-only targets: их модель
распространения не допускает обычную загрузку внешнего desktop bridge.

## Что уже надёжно поддерживается

- генерация typed Go bindings из одного WIT package;
- загрузка одного WIT-файла, явного набора файлов, package-каталога или дерева;
- необязательные Go overlays для aliases/records и `result_error`;
- contract validation до instantiation;
- загрузка standard WebAssembly Components;
- host imports из Go;
- typed exports в generated API;
- прозрачная маршрутизация plugin-to-plugin вызовов через `Host`;
- лимиты по fuel, timeout, memory, instance count и размеру сообщений;
- in-process bridge для Linux, macOS и Windows на `amd64` и `arm64`.

## Что важно понимать про типы

Полноценно подходят для обычного typed API:

- числа, `bool`, `string`, `char`
- `record`, `list`, `tuple`
- `option`, `result`, `enum`, `flags`, `variant`
- `map<K,V>` при `comparable` ключе в Go

Особый случай:

- `resource`, `future`, `stream`, `error-context` идут как `witgo.Handle`

Это значит, что runtime умеет безопасно хранить и передавать такие значения, но
они пока не превращаются в богатый высокоуровневый Go API с typed payload для
`future<T>` и `stream<T>`.

## Что сознательно ограничено

- core Wasm modules не поддерживаются, нужен именно Component Model;
- один `Runtime` не рассчитан на параллельные reentrant-вызовы;
- timeout прерывает Wasm, но не убивает зависший доверенный host callback;
- streaming больших payload пока нет, сообщение полностью живёт в памяти;
- `future<T>` и `stream<T>` пока не имеют полноценного typed consumer API;
- handle нельзя безопасно переносить между независимыми runtime-box;
- bridge не является guest plugin и не должен им становиться.
- overlay mapping внутри `list`, `option`, `result`, `map`, tuple и variant пока
  отклоняется до генерации; custom codec symbols и `uuid-string` не реализованы;
- Android, iOS, WASI и targets без native loader поддерживают parser, generator
  и generated API, но не desktop Component runtime.

## Что важно про безопасность

- plugin не получает прямого доступа к registry и runtime internals;
- системный runtime API доступен только через явный opt-in;
- unsafe fuel request не даёт guest контроля над fuel, а только создаёт запрос host policy;
- capability model не обходится через nested composition или system API.
