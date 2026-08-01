# Возможности и ограничения

Здесь только практические границы текущего `witgo`.

## Что уже надёжно поддерживается

- генерация typed Go bindings из одного WIT package;
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

## Что важно про безопасность

- plugin не получает прямого доступа к registry и runtime internals;
- системный runtime API доступен только через явный opt-in;
- unsafe fuel request не даёт guest контроля над fuel, а только создаёт запрос host policy;
- capability model не обходится через nested composition или system API.
