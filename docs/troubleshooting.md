# Troubleshooting

Ниже собраны типовые проблемы при работе с `witgo` и WebAssembly Components.

## 1. `ErrCoreModule` или сообщение про core module

Причина: вы передали обычный Wasm module, а не WebAssembly Component.

Что проверить:

- файл действительно собран как Component;
- ваш Rust toolchain не выдал plain `.wasm` без component wrapping;
- вы не перепутали промежуточный build artifact и финальный component.

## 2. `ErrBridgeProtocolMismatch` или несовместимый protocol version

Причина: Go-код и native bridge относятся к разным версиям протокола.

Что делать:

- убедитесь, что встроенные bridge-артефакты обновлены вместе с Go-кодом;
- если используете `BridgePath` или `WITGO_COMPONENT_LIBRARY`, проверьте, что
  путь указывает на bridge той же релизной линии;
- после изменения protocol/version пересоберите native bridge и embedded
  artifacts вместе.

## 3. `ErrBridgeVersionMismatch`

Причина: версия Go-библиотеки и версия Rust bridge не совпадают.

Что делать:

- не смешивайте bridge из старого релиза с новым Go package;
- проверьте `WITGO_COMPONENT_LIBRARY`, если bridge задаётся через env;
- пересоберите или замените bridge на совместимый.

## 4. `ErrContractMismatch`

Причина: plugin и generated Go contract расходятся по интерфейсу.

Самые частые случаи:

- missing import;
- missing export;
- лишний export или import;
- тот же символ, но другая structural signature;
- несовпадение версии package/interface в полном имени функции.

Что делать:

```go
report, err := contract.ValidatePlugin(path)
if err != nil {
	return err
}
if !report.Compatible {
	log.Println(report.Summary())
}
```

Сначала чините WIT и generated bindings, а не runtime code.

## 5. Plugin открывается, но падает на вызове

Возможные причины:

- ошибка внутри guest-кода;
- trap в Wasm;
- ошибка host import;
- проблема с преобразованием значений результата.

Что проверить:

- оборачивайте вызовы `plugin.<Interface>.<Method>()` логированием ошибок;
- если работаете через low-level `Runtime.Call`, проверьте точное имя функции;
- убедитесь, что аргументы соответствуют Component Model shape, а не только
  “похожи” на него.

## 6. Timeout не останавливает host callback

Это ожидаемое поведение.

`Timeout` в `RuntimeOptions` прерывает Wasm execution через Wasmtime epoch
interruption, но не может безопасно остановить зависший Go callback или
deadlock внутри доверенного native-кода.

Что делать:

- делайте timeout и cancellation внутри самих host-функций;
- не держите в host callback долгие блокирующие операции без собственных
  лимитов;
- не воспринимайте `Timeout` как универсальный kill-switch на всё.

## 7. Fuel не работает

Проверьте:

- вы используете либо `Fuel`, либо `FuelPerCall`, но не оба сразу;
- вы действительно читаете/меняете fuel на том же `Runtime`;
- ошибка не `ErrFuelDisabled`.

Если используете `FuelPerCall`, помните, что это budget отдельного вызова, а не
общий lifetime budget.

## 8. `resource`/`future`/`stream` ведут себя не так, как ожидалось

Важно понимать текущую модель:

- они представлены через `witgo.Handle`;
- handle привязан к Store конкретной коробки;
- WebAssembly provider автоматически компонуется с потребителем в эту же
  коробку, поэтому вложенный вызов может безопасно передавать handle;
- между независимыми коробками и через Go provider handle передавать нельзя;
- `future<T>` и `stream<T>` пока не имеют generic high-level API для typed
  payload.

Если нужен полноценный typed consumer API, это пока ограничение библиотеки, а
не ошибка в вашем коде.

## 9. Проблемы со сборкой examples или `wat2wasm`

В этом репозитории часть example `.wasm` уже лежит готовой, но локальная
перегенерация `.wat -> .wasm` на Windows может потребовать:

- 64-битный Go toolchain;
- доступный C toolchain для CGO;
- корректный `PATH`, если используется MSYS2/MinGW.

Если `go` в PATH указывает на 32-битный дистрибутив, часть вспомогательных
инструментов может ломаться даже при корректном исходном коде.

Для обычной Go-сборки `witgo` CGO не нужен. C toolchain обязателен только для
TinyGo backend-а и для отдельных вспомогательных инструментов примеров.

## 10. Ошибка Go overlay

Generator останавливается до записи output, если overlay содержит неизвестный
canonical ID, schema version, codec или несовместимый Go type/import.

Проверьте:

- ID имеет вид `namespace:package/interface@version#member`;
- alias действительно объявлен в указанном interface;
- `time.Time` и `time.Duration` используют `import: time`;
- codec соответствует WIT `s64`;
- mapping не находится внутри пока неподдержанного container или variant.

Путь overlay и проблемный ID входят в текст ошибки. Не исправляйте такую ошибку
изменением WIT на нестандартный Go-синтаксис.

## 11. Как быстро локализовать источник проблемы

Практичный порядок:

1. `InspectComponent` - убедиться, что это вообще Component и посмотреть manifest.
2. `ValidatePlugin` - убедиться, что контракт совпадает.
3. `OpenPlugin` - убедиться, что link/handshake проходят.
4. Один минимальный вызов export - убедиться, что runtime path жив.
5. Только потом отлаживать сложные типы, handles и production limits.

Обычно это быстрее, чем сразу идти в полную business-логику.
