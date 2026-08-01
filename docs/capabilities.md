# Возможности и ограничения

Этот документ описывает фактическое текущее состояние `witgo`. Здесь важно
разделять три уровня поддержки:

1. Parser: WIT-конструкция распознаётся и попадает в IR.
2. Generator: из неё получается компилируемый и удобный Go API.
3. Runtime: значение можно реально передать между Go и WebAssembly Component.

Поддержка на первом уровне не означает автоматическую поддержку на двух
остальных.

## Основной сценарий

`witgo` умеет:

- читать один `.wit`-файл или рекурсивно собирать каталог с `.wit`;
- генерировать типизированные Go-типы, host-интерфейсы и clients для exports;
- генерировать для каждого `world` единый `<World>Imports`, конструкторы,
  `<World>Ping`, `Validate<World>` и `Check<World>`;
- загружать binary component или текстовый `(component ...)`;
- принимать Component как путь или как `[]byte`;
- вызывать exports и синхронные Go host imports;
- проверять имена и structural signatures функций до instantiation;
- ограничивать fuel, время Wasm-вызова, linear memory, число instances и размер
  protocol messages;
- использовать встроенный или явно указанный native bridge.

Core WebAssembly modules не поддерживаются и отклоняются с `ErrCoreModule`.
Нужен именно WebAssembly Component Model component.

## Генератор

### Что генерируется

- Go package для Go 1.18+;
- константы WIT package namespace, name, version и ID;
- records, enums, flags, variants, aliases и часть resource-интерфейсов;
- типизированные интерфейсы для host imports;
- типизированные clients для interface exports и direct world exports;
- один imports-struct на `world`, чтобы большие контракты не раздували сигнатуру
  конструктора;
- адаптеры `HostImport`, которые проверяют число аргументов и преобразуют
  значения;
- `Close` для generated plugin client;
- точный contract manifest с imports, exports и structural signatures;
- validation helpers для файла и runtime options.

Выходной файл форматируется через `go/format` и заменяется атомарно. По
умолчанию он называется `bindings.gen.go`.

### Ограничения генератора

- Все входные `.wit`-файлы должны принадлежать одному package. Несколько
  package в одном запуске отклоняются.
- Полноценного package/dependency resolver пока нет. Внешние package paths не
  скачиваются и не резолвятся через registry.
- `use` и `include ... with` парсятся, но generator пока не выполняет полное
  раскрытие импортированных типов и world.
- WIT attributes/annotations сохраняются parser-ом, но не влияют на generated
  API.
- `async` сохраняется в IR, но generated метод остаётся обычным синхронным
  Go-методом.
- Generated constructors и static methods для resources пока не выделяются в
  отдельный высокоуровневый API автоматически.
- Multi-result синтаксис в виде отдельного списка именованных результатов не
  генерируется как отдельная форма Go API. Используйте один result type,
  `record` или `tuple`.
- Коллизии WIT-имен после нормализации в Go identifier отклоняются с точной
  диагностикой.

## Матрица WIT-типов

| WIT | Generated Go | Runtime | Статус и замечания |
| --- | --- | --- | --- |
| `bool` | `bool` | да | Поддерживается. |
| `u8/u16/u32/u64` | соответствующий `uint*` | да | Поддерживается, без промежуточного `float64` при JSON-декодировании. |
| `s8/s16/s32/s64` | соответствующий `int*` | да | Поддерживается. |
| `f32/f64` | `float32/float64` | да | `NaN` и `Inf` не кодируются стандартным JSON codec. |
| `string` | `string` | да | Поддерживается. |
| `char` | `witgo.Char` | да | Проверяется как Unicode scalar value; codec использует требуемую WIT односимвольную строку. |
| `record` | `struct` с WIT JSON tags | да | Поддерживаются вложенные поля. |
| `list<T>` | `[]T` | да | Поддерживаются вложенные и сложные списки. |
| `option<T>` | `witgo.Option[T]` | да | Есть `Some`, `None`, getters, pointer conversion и map helpers; строгий envelope сохраняет вложенные options. |
| `tuple<...>` | `witgo.Tuple0`...`witgo.Tuple16`, затем `witgo.Tuple` | да | До 16 элементов доступны типизированные поля; дальше используется динамический tuple с `At`, `Set`, `TupleValue`. |
| `result<T,E>` | `witgo.Result[T,E]` | да | Ровно одна активная ветвь; есть constructors, getters, `MatchResult`, `MapResult`, `MapResultErr` и строгий codec. |
| `enum` | именованный `string` + constants | да | Генерируются `Parse<Type>`, `Valid`, `String` и `<Type>Values`. |
| `flags` | именованный `uint64` bitset | да | Генерируются `Parse<Type>`, `Valid`, `Has`, `Add`, `Remove`, `Names`. Предел: 64 flags. |
| `variant` | `struct` с `Kind` и payload pointers | да | Для каждого case генерируются constructor, predicate и безопасный accessor, плюс строгий codec. |
| aliases | Go alias | зависит от target | Поведение совпадает с базовым типом. |
| `map<K,V>` | `witgo.Map[K,V]` | да | Pair-array codec соответствует Component Model map ABI; доступны `NewMap`, `Get`, `Put`, `Delete`, `Clone`. Ключ должен быть `comparable` в Go. |
| `own<T>` / `borrow<T>` | `witgo.Handle` | да | Bridge проверяет Store и тип, переносит `own`, временно заимствует `borrow` и не даёт использовать закрытый handle. |
| `resource` | `witgo.Handle` | частично | Guest resource можно получить, передать обратно и закрыть через `Close`. Конкретная resource identity проверяется в bridge, но разные resource-типы пока имеют один Go-тип. |
| `future<T>` | `witgo.Handle` | частично | Handle можно передавать и закрывать. Generic Go API для ожидания и чтения typed payload пока нет. |
| `stream<T>` | `witgo.Handle` | частично | Handle можно передавать и закрывать. `Next`/`Read`/`Write` для typed payload пока нет. |
| `error-context` | `witgo.Handle` | частично | Opaque context можно передавать между вызовами и освобождать, но debug message из Wasmtime dynamic API недоступен. |

То, что тип парсится, ещё не означает, что generated package для него удобно
скомпилируется и пройдёт полноценный round-trip.

## Runtime и вызовы

### Поддерживается

- вызов interface export по полному имени
  `namespace:package/interface@version#function`;
- direct world functions по простому имени;
- ноль, один или несколько результатов в low-level `Runtime.Call`;
- синхронный callback из Component в зарегистрированный Go host import;
- runtime-bound handles с проверкой kind, Store и явным `Close`;
- преобразование host callback error в ошибку Component call;
- несколько независимых `Runtime`, выполняющихся параллельно;
- сериализация вызовов внутри одного `Runtime`;
- идемпотентный `Close` и `IsClosed`;
- удаление временного файла после `Close` при загрузке из `[]byte`;
- классификация fuel exhaustion и timeout через `errors.Is`;
- чтение и изменение остатка fuel, если metering включён.

### Ограничения runtime

- Go API вызовов синхронный. Bridge использует async Wasmtime Store, но
  `context.Context` и типизированный consumer API для `future`/`stream` пока
  отсутствуют.
- Вызовы одного `Runtime` не выполняются параллельно. Для параллелизма нужны
  отдельные instances/runtime.
- Reentrant-вызов того же `Runtime` из его собственного host callback не
  поддерживается и может упереться в mutex.
- Instance pool, hot reload и автоматическая замена упавшего instance пока не
  реализованы.
- Timeout прерывает Wasm через Wasmtime epoch interruption, но не может
  остановить зависший Go callback или deadlock внутри доверенной native library.
- Автоматического WASI linker нет. Component получает только явно
  зарегистрированные function imports.
- Low-level `Runtime.Call` принимает `any`, поэтому корректность типов там
  проверяется только в момент преобразования. Generated package даёт
  compile-time типизацию.
- После trap или ошибки нет автоматического восстановления instance.
- Streaming больших аргументов и результатов пока нет: сообщение целиком
  кодируется в JSON и находится в памяти.

## Проверка контракта

`InspectComponent*` и `ValidateComponent*`:

- загружают и разбирают Component;
- выполняют version handshake с bridge;
- получают отсортированные imports, exports и structural signatures;
- не выполняют `start`, не instantiate-ят Component и не вызывают guest code;
- возвращают missing/unexpected imports и exports отдельно;
- возвращают несовпавшие сигнатуры с `expected` и `actual`;
- понимают resource ownership как `own` и `borrow` без создания handle.

Ограничения validation:

- это проверка интерфейса, а не поведения плагина;
- она не доказывает, что instantiation и последующие вызовы обязательно будут
  успешны;
- версии WIT package/interface сравниваются как часть точного имени, без
  semver ranges и без compatibility adapters;
- лишняя функция считается несовместимостью так же, как и отсутствующая;
- подпись и доверие к самому `.wasm`-файлу validation не проверяет;
- capability policy как отдельный API-объект пока отсутствует: приложение
  может инспектировать `ImportNames`, но решение принимает само.

`ValidationReport` даёт `Err`, `Summary`, `ProblemCount`, а `Contract` даёт
`ImportNames`, `ExportNames`, `FunctionNames`, `Requires`, `Provides` и
`Signature`.

## Version handshake

До instantiation Go и Rust обмениваются:

- `protocol_version`;
- `witgo_version`;
- точной `bridge_version`;
- версией Wasmtime;
- списком обязательных protocol features;
- contract manifest.

Rust проверяет требования Go, а Go независимо перепроверяет ответ Rust. Любое
несовпадение останавливает загрузку без fallback. После изменения
protocol/version встроенные native artifacts нужно пересобирать вместе с Go
package.

## Лимиты выполнения

| Опция | Что делает | Ограничение |
| --- | --- | --- |
| `Fuel` | Общий budget Store | Нельзя использовать вместе с `FuelPerCall`; host callback fuel не потребляет. |
| `FuelPerCall` | Сбрасывает budget перед каждым export call | Это budget отдельного вызова, а не общий deadline. |
| `Timeout` | Epoch deadline для Wasm-вызова | Не отменяет Go callback и native deadlock. |
| `MemoryLimitBytes` | Лимит каждой linear memory | Не является общим лимитом памяти процесса. |
| `InstanceLimit` | Лимит Component instances в Store | Один `Runtime` всё равно создаёт один основной instance. |
| `MaxResultBytes` | Лимит in-memory protocol message | Защищает и входящие, и исходящие сообщения; отдельного streaming API нет. |

Сейчас нет отдельных лимитов для tables, размера component-файла, количества
функций, длительности host callback и общей памяти Go-процесса.

## Bridge, платформы и доставка

Поддерживаемая матрица:

| ОС | amd64 | arm64 |
| --- | --- | --- |
| Linux | да | да |
| macOS | да | да |
| Windows | да | да |

Bridge работает in-process, вызывается через `purego` и не требует CGO или
отдельного sidecar-процесса. Встроенная библиотека распаковывается в user
cache, потому что ОС не умеет загружать shared library прямо из Go byte slice.

Порядок поиска:

1. `RuntimeOptions.BridgePath`;
2. `WITGO_COMPONENT_LIBRARY`;
3. встроенная библиотека, если она не отключена;
4. platform library из `PATH`;
5. локальные `bridge/target/release` и `bridge/target/debug`.

`BridgeSHA256` проверяет явно выбранный bridge. Встроенные compressed artifacts
и cache-файлы проверяются по встроенному SHA-256. `DisableEmbeddedBridge` или
`WITGO_DISABLE_EMBEDDED_BRIDGE=1` запрещают только встроенную копию, но не
остальные источники.

CI собирает и тестирует все шесть OS/architecture combinations. Release flow
также генерирует checksums, SPDX SBOM и build attestations.

## Граница безопасности

- WebAssembly-код изолируется Wasmtime и получает только зарегистрированные
  host imports.
- `witgo` не выдаёт filesystem, network или environment автоматически.
- Go host implementations остаются доверенным кодом и сами отвечают за
  авторизацию, timeout, rate limits и проверку данных.
- Native Rust bridge загружается внутрь Go-процесса и тоже остаётся доверенным
  кодом. Ошибка памяти или deadlock в native-коде влияет на весь процесс.
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
- `context.Context` cancellation;
- generated host-defined resource constructors/destructors;
- typed future await и stream reader/writer;
- встроенная WASI-реализация;
- semantic-version negotiation и compatibility adapters.

Эти возможности не стоит считать скрытыми или экспериментальными API. Их
просто пока нет в публичной библиотеке.
