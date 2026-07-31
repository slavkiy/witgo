# Возможности и ограничения

Этот документ описывает текущее фактическое состояние `witgo`. Важно
различать три уровня поддержки:

1. **Parser** - конструкция WIT распознаётся и попадает в IR.
2. **Generator** - из неё получается компилируемый и удобный Go API.
3. **Runtime** - значение можно передать между Go и WebAssembly Component через
   Wasmtime bridge.

Поддержка на первом уровне не означает поддержку на двух остальных.

## Основной сценарий

`witgo` умеет:

- читать один `.wit`-файл или рекурсивно собирать `.wit`-файлы каталога;
- генерировать типизированные Go-типы, host-интерфейсы и clients для exports;
- генерировать для каждого world единый `<World>Imports`, конструкторы,
  `<World>Ping`, `Validate<World>` и `Check<World>`;
- загружать бинарный Component или текстовый `(component ...)`;
- принимать Component как путь или как `[]byte`;
- вызывать exports и синхронные Go host imports;
- проверять имена и структурные сигнатуры функций до instantiation;
- ограничивать fuel, время Wasm-вызова, linear memory, число instances и размер
  сообщений;
- использовать встроенный либо явно указанный native bridge.

Core WebAssembly modules не поддерживаются и отклоняются с `ErrCoreModule`.
Нужен именно WebAssembly Component Model component.

## Генератор

### Что генерируется

- Go package для Go 1.18+;
- константы WIT package namespace/name/version/ID;
- records, enums, flags, variants, aliases и часть resource-интерфейсов;
- типизированные интерфейсы для host imports;
- типизированные clients для interface и direct world exports;
- один imports-struct на world, поэтому большой контракт не раздувает сигнатуру
  конструктора;
- адаптеры `HostImport`, проверяющие число аргументов и преобразующие значения;
- `Close` для generated plugin client;
- точный contract manifest с imports, exports и сигнатурами;
- validation helpers для файла и runtime options.

Выходной файл форматируется через `go/format` и заменяется атомарно. По
умолчанию он называется `bindings.gen.go`.

### Ограничения генератора WIT

- Все входные `.wit`-файлы должны принадлежать одному package. Несколько package
  в одном запуске отклоняются.
- Полноценного package/dependency resolver пока нет. Внешние package paths не
  скачиваются и не разрешаются через registry.
- `use` и `include ... with` парсятся, но generator не выполняет полноценное
  раскрытие импортированных типов/world. Ссылочная interface должна находиться
  среди локально загруженных declarations.
- WIT attributes/annotations сохраняются parser-ом, но не влияют на generated
  API.
- `async` сохраняется в IR, но generated метод остаётся обычным синхронным Go
  методом.
- Resource constructors и static methods пока не превращаются в отдельные Go
  methods автоматически. Низкоуровневый вызов и generated functions возвращают
  runtime-bound `witgo.Handle` с явным lifecycle.
- Multi-result синтаксис функций как отдельный список именованных результатов
  не генерируется; используйте один result type, record или tuple.
- Коллизии declaration-имён после нормализации в Go identifier отклоняются с
  диагностикой обоих WIT-имён.

## Матрица WIT-типов

| WIT | Generated Go | Runtime | Статус и замечания |
| --- | --- | --- | --- |
| `bool` | `bool` | да | Поддерживается. |
| `u8/u16/u32/u64` | соответствующий `uint*` | да | Поддерживается; JSON не использует `float64` как промежуточный тип при чтении. |
| `s8/s16/s32/s64` | соответствующий `int*` | да | Поддерживается. |
| `f32/f64` | `float32/float64` | да | `NaN` и `Inf` не представимы стандартным JSON codec. |
| `string` | `string` | да | Поддерживается. |
| `char` | `witgo.Char` | да | Проверяется Unicode scalar value; codec использует требуемую WIT односимвольную строку. |
| `record` | struct с WIT JSON tags | да | Поддерживаются вложенные поля. |
| `list<T>` | `[]T` | да | Поддерживаются вложенные/сложные списки. |
| `option<T>` | `witgo.Option[T]` | да | `Some`, `None`, getters, pointer conversion и map helpers; строгие `{some:...}`/`{none:true}` envelopes сохраняют nested options. |
| `tuple<...>` | `witgo.Tuple0` ... `witgo.Tuple16` | да | Typed fields, constructors, `Values` и строгий JSON-array codec. Arity выше 16 отклоняется. |
| `result<T,E>` | `witgo.Result[T,E]` | да | Ровно одна активная ветвь; constructors, getters, match/map helpers и строгий codec. |
| `enum` | named `string` + constants | да | Генерируются `Parse<Type>`, `Valid`, `String` и `<Type>Values`. |
| `flags` | named `uint64` bitset | да | Генерируются `Parse<Type>`, `Valid`, `Has`, `Add`, `Remove`, `Names` и строгий codec. Предел - 64 flags. |
| `variant` | struct с `Kind` и payload pointers | да | Для каждого case генерируются constructor, predicate и безопасный accessor, плюс строгий codec. |
| aliases | Go alias | зависит от target | Возможности совпадают с базовым типом. |
| `map<K,V>` | `map[K]V` | нет | Parser/generator extension существует, но текущий Wasmtime Component type и bridge не обеспечивают runtime ABI для map. |
| `own<T>` / `borrow<T>` | `witgo.Handle` | да | Bridge проверяет Store/type, переносит `own`, временно заимствует `borrow` и не позволяет использовать закрытый token. |
| `resource` | `witgo.Handle` | частично | Guest resource можно получить, передать обратно и уничтожить через `Close`. Конкретная resource identity проверяется bridge-ом во время вызова, но разные resources пока имеют один Go-тип. Generated constructors/method dispatch и host-defined resource imports не автоматизированы. |
| `future<T>` | `witgo.Handle` | частично | Handle можно передавать и закрывать. Generic Go API для ожидания/чтения typed payload пока отсутствует. |
| `stream<T>` | `witgo.Handle` | частично | Handle можно передавать и закрывать. `Next`/`Read`/`Write` typed payload пока отсутствуют. |
| `error-context` | `witgo.Handle` | частично | Opaque context можно передавать между calls и освобождать; извлечение debug message не предоставляется Wasmtime dynamic value API. |

Таким образом, утверждение «тип парсится» нельзя использовать как подтверждение
того, что generated package для него скомпилируется или выполнит round-trip.

## Runtime и вызовы

### Поддерживается

- вызов export по полному имени `namespace:package/interface@version#function`;
- direct world functions по простому имени;
- ноль, один или несколько результатов в low-level `Runtime.Call`;
- синхронный callback из Component в зарегистрированный Go host import;
- runtime-bound handles с проверкой kind, Store и явным `Close`;
- преобразование host callback error в ошибку Component call;
- несколько независимых `Runtime`, выполняющихся параллельно;
- сериализация вызовов внутри одного `Runtime`;
- идемпотентный `Close` и `IsClosed`;
- удаление временного файла после `Close` для загрузки из `[]byte`;
- классификация fuel exhaustion и timeout через `errors.Is`;
- чтение и изменение остатка fuel, когда metering включён.

### Ограничения runtime

- Go API вызовов синхронный. Bridge использует async Wasmtime Store, но
  `context.Context` и typed future/stream consumer API пока нет.
- Вызовы одного `Runtime` не выполняются параллельно. Для параллелизма нужны
  отдельные instances/runtime.
- Reentrant-вызов того же `Runtime` из его host callback не поддерживается и
  может ждать mutex.
- Pool instances, hot reload и автоматическая замена упавшего instance пока не
  реализованы.
- Timeout прерывает Wasm через Wasmtime epoch interruption, но не способен
  остановить зависший Go host callback или зависание внутри доверенной native
  shared library.
- Автоматического WASI linker нет. Component получает только явно
  зарегистрированные function imports; resource-heavy WASI interfaces требуют
  отдельной регистрации host-defined resource types, которой пока нет.
- Low-level `Runtime.Call` принимает `any`, поэтому корректность типов там
  проверяется только во время преобразования. Generated package даёт
  compile-time типизацию.
- После ошибки/trap нет автоматического восстановления instance.
- Нет streaming больших arguments/results: сообщение целиком кодируется в
  JSON и находится в памяти.

## Проверка контракта

`InspectComponent*` и `ValidateComponent*`:

- загружают и разбирают Component;
- выполняют version handshake с bridge;
- получают отсортированные imports/exports и structural signatures;
- не выполняют `start`, не instantiate-ят Component и не вызывают guest code;
- возвращают missing/unexpected imports и exports отдельно;
- возвращают несовпавшие сигнатуры с expected/actual;
- поддерживают проверку resource ownership как `own`/`borrow` без создания
  handle.

Ограничения validation:

- это проверка интерфейса, а не поведения плагина;
- она не доказывает, что instantiation или последующий вызов завершится успешно;
- версии WIT package/interface сравниваются как часть точного имени. SemVer
  ranges, backward-compatible negotiation и автоматические adapters отсутствуют;
- лишняя функция считается несовместимостью, как и отсутствующая;
- подпись/доверие самого plugin `.wasm` не проверяются;
- capability policy пока не является отдельным объектом API: приложение может
  инспектировать `ImportNames`, но должно самостоятельно принять решение.

`ValidationReport` предоставляет `Err`, `Summary`, `ProblemCount`, а `Contract`
- `ImportNames`, `ExportNames`, `FunctionNames`, `Requires`, `Provides` и
`Signature`.

## Version handshake

До instantiation Go и Rust обмениваются:

- `protocol_version`;
- версией Go-модуля `witgo_version`;
- точной `bridge_version`;
- версией Wasmtime;
- списком обязательных protocol features;
- contract manifest.

Rust проверяет требования Go, а Go независимо проверяет ответ Rust. Любое
несовпадение останавливает загрузку без fallback. Это намеренно строгая
проверка: диапазоны совместимых bridge-версий и downgrade protocol отсутствуют.
После изменения protocol/version встроенные native artifacts обязательно надо
пересобрать вместе с Go package.

## Лимиты выполнения

| Опция | Что делает | Ограничение |
| --- | --- | --- |
| `Fuel` | Общий бюджет Store | Нельзя использовать вместе с `FuelPerCall`; host callback fuel не потребляет. |
| `FuelPerCall` | Сбрасывает бюджет перед каждым export call | Это budget отдельного вызова, а не deadline. |
| `Timeout` | Epoch deadline Wasm-вызова | Не отменяет Go callback/native deadlock. |
| `MemoryLimitBytes` | Лимит каждой linear memory | Не является общим лимитом памяти процесса. |
| `InstanceLimit` | Лимит Component instances в Store | Один `Runtime` всё равно создаёт один основной instance. |
| `MaxResultBytes` | Лимит in-memory protocol message | Фактически защищает и входящие, и исходящие сообщения, включая arguments. Streaming нет. |

Отдельных лимитов для tables, component file size, количества функций,
длительности host callback и памяти Go-процесса сейчас нет.

## Bridge, платформы и доставка

Поддерживаемая матрица:

| ОС | amd64 | arm64 |
| --- | --- | --- |
| Linux | да | да |
| macOS | да | да |
| Windows | да | да |

Bridge работает in-process, вызывается через `purego` и не требует CGO или
отдельного sidecar process. Встроенная библиотека распаковывается в user cache,
потому что ОС не умеет загружать shared library прямо из Go byte slice.

Порядок поиска:

1. `RuntimeOptions.BridgePath`;
2. `WITGO_COMPONENT_LIBRARY`;
3. встроенная библиотека, если она не отключена;
4. platform library из `PATH`;
5. локальные `bridge/target/release` и `bridge/target/debug`.

`BridgeSHA256` проверяет явно выбранный bridge. Встроенные compressed artifacts
и cache проверяются по встроенному SHA-256. `DisableEmbeddedBridge` или
`WITGO_DISABLE_EMBEDDED_BRIDGE=1` запрещает только встроенную копию, но не
запрещает остальные источники.

CI собирает и тестирует все шесть OS/architecture combinations. Release flow
также формирует checksums, SPDX SBOM и build attestations.

## Граница безопасности

- WebAssembly-код изолируется Wasmtime и получает только зарегистрированные
  host imports.
- `witgo` не выдаёт filesystem/network/environment автоматически.
- Go host implementations являются доверенным кодом и сами отвечают за
  авторизацию, timeout, rate limits и проверку данных.
- Native Rust bridge загружается внутрь Go-процесса и тоже является доверенным
  кодом. Ошибка памяти или deadlock в native code влияет на весь процесс.
- `BridgeSHA256` проверяет целостность bridge, но не является подписью plugin.
- Windows Authenticode и Apple signing/notarization не выполняются библиотекой
  во время запуска и остаются задачей release/distribution процесса.

## Пока отсутствует

- generated mock plugin, host recorder и fixture generator;
- capability policy с allow/deny правилами;
- hooks/observer, метрики duration/fuel/memory и OpenTelemetry adapter;
- instance pool;
- hot reload;
- plugin registry, marketplace и автоматическое скачивание plugins;
- автоматическое скачивание bridge во время запуска;
- context cancellation;
- generated host-defined resource constructors/destructors;
- typed future await и stream reader/writer;
- built-in WASI implementation;
- semantic-version negotiation и compatibility adapters.

Эти пункты не следует считать скрытыми или экспериментальными API - их пока
нет в публичной библиотеке.
