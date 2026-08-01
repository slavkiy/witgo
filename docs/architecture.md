# Архитектура runtime

`witgo` запускает WebAssembly Components in-process через встроенную Wasmtime
shared library. Нет отдельного процесса, нет stdin/stdout IPC, нет сетевой
загрузки runtime и нет обязательной внешней установки.

Go package встраивает по одной gzip-сжатой библиотеке на каждую поддерживаемую
платформу:

- Windows `amd64`/`arm64`: `witgo_bridge.dll`;
- Linux `amd64`/`arm64`: `libwitgo_bridge.so`;
- macOS `amd64`/`arm64`: `libwitgo_bridge.dylib`.

При первом использовании библиотека распаковывается в пользовательский cache.
ОС не умеют загружать DLL/shared object прямо из Go byte slice, поэтому cache
файл здесь неизбежен. Имя файла включает content hash, установка выполняется
атомарно, а встроенный SHA-256 проверяется и перед записью, и на каждом cache
hit.

`BridgePath` или `WITGO_COMPONENT_LIBRARY` позволяют выбрать внешнюю
администраторскую библиотеку, `BridgeSHA256` закрепляет её по хэшу.
`DisableEmbeddedBridge` запрещает использовать встроенную копию.

## Внутрипроцессный протокол

Go вызывает небольшой стабильный C ABI через `purego`, без CGO. Внутри Rust
остаётся существующий JSON value protocol поверх in-memory каналов.

Инициализация выполняет строгий handshake, который включает:

- `protocol_version`;
- `witgo_version`;
- `bridge_version`;
- `wasmtime_version`;
- список обязательных `features`.

До instantiation bridge отвечает на contract `ping` отсортированными именами
component imports/exports. Generated Go bindings сравнивают этот manifest с WIT
контрактом и набором зарегистрированных host adapters ещё до отправки `start`.

Go передаёт в `init` требуемую bridge-версию и обязательные feature-флаги. Rust
отклоняет любое несовпадение, а Go независимо перепроверяет значения,
возвращённые в `pong`. Режима совместимости или fallback нет.

Вызовы внутри одного `Runtime` сериализуются. Разные `Runtime` можно выполнять
параллельно. `Close` освобождает состояние Wasmtime, дожидается worker thread,
по возможности выгружает библиотеку и остаётся идемпотентным. Timeout для Wasm
основан на Wasmtime epoch interruption. Если зависание произошло внутри
доверенного native-кода, безопасно остановить его без завершения процесса
нельзя, это осознанный trade-off in-process модели.

## Поддерживаемые значения

Codec покрывает:

- `bool`;
- все WIT integer и float типы;
- `char`;
- `string`;
- `list`;
- `map`;
- `record`;
- `tuple`;
- `variant`;
- `enum`;
- `option`;
- `result`;
- `flags`.

`resource`, `future`, `stream` и `error-context` представлены непрозрачными
runtime-bound handle-токенами. Bridge удерживает соответствующее Wasmtime
значение, проверяет его kind и Store, переносит `own`, сохраняет `borrow` на
время вызова и явно дропает удержанные handle.

Resource-типы входят и в pre-instantiation contract manifest. Dynamic API умеет
передавать `future`/`stream` handles между вызовами и закрывать их, но пока не
умеет читать или писать произвольный типизированный payload. Для этого нужна
generated Rust specialization, потому что type-erased `FutureAny` и `StreamAny`
из Wasmtime открывают reader/writer только для статически известных типов.

## Граница безопасности

Shared library остаётся доверенным native-кодом. Релизный процесс собирает все
шесть библиотек из `Cargo.lock`, тестирует каждую через Go, публикует raw и
compressed SHA-256 checksums, SPDX SBOM и GitHub build attestations, а затем
фиксирует точные compressed libraries в исходниках модуля перед созданием тега.

Для production-дистрибуции по-прежнему важны Windows Authenticode и Apple
signing/notarization, чтобы уменьшать ложные срабатывания антивирусов и
поддерживать привычную цепочку доверия.
