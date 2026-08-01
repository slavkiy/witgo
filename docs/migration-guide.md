# Руководство по миграции

## WIT packages и Go overlays

Старый `Generate(Config{WIT: path})` продолжает работать. Для нового кода лучше
явно выбрать намерение:

```go
witgo.GenerateFile(config, "plugin.wit")
witgo.GenerateFiles(config, "types.wit", "plugin.wit")
witgo.GeneratePackage(config, "./wit")
witgo.GenerateTree(config, "./wit")
```

`GeneratePackage` не заходит в `deps/`; `GenerateTree` рекурсивен и требует один
package во всех найденных файлах.

`Config.GoOverlay` opt-in: без него перегенерация не меняет типы из-за появления
overlay feature. После включения overlay публичные aliases/records могут
измениться, поэтому generated diff и реализации host interfaces нужно проверить
как обычное изменение API. WIT и plugin binary пересобирать не требуется, если
wire contract не менялся.

## Переход на единые interfaces композиции

Generated методы WIT interfaces теперь всегда возвращают transport/runtime
`error`, независимо от того, используется interface как import или export.
Старая Go-реализация:

```go
func (codec) Decode(ctx context.Context, data []byte) Result { return result }
```

должна стать:

```go
func (codec) Decode(ctx context.Context, data []byte) (Result, error) {
	return result, nil
}
```

Это позволяет одному интерфейсу представлять Go object, plugin export client,
mock и зарегистрированный provider без несовместимых адаптеров.

## Переход на контекстный generated API

Generated export- и host-методы теперь принимают `context.Context` первым аргументом. После повторной генерации замените вызовы и реализации:

```go
// Раньше
info, err := plugin.Metadata.Get()
func (host) ProcessString(value string) string

// Теперь
info, err := plugin.Metadata.Get(ctx)
func (host) ProcessString(ctx context.Context, value string) (string, error)
```

Для загрузки, проверки контракта и полного перезапуска доступны `OpenPluginContext`, `ValidatePluginContext`, `CheckPluginContext` и `RestartContext`. Старые функции загрузки и проверки сохранены и используют `context.Background()`.

Этот документ описывает переход на актуальную релизную линию `witgo v0.2.x`.

Главный контекст: `v0.2.0` закрепил строгий handshake между Go и Rust bridge,
добавил contract validation и окончательно убрал legacy API до Component Model.

## Для кого этот guide

Он нужен, если вы:

- обновляетесь с ранних pre-`v0.2.0` сборок;
- пользовались старыми bridge-артефактами;
- ожидали, что generator и runtime будут терпимее к несовпадениям контракта;
- опирались на старые CLI-флаги или до-Component-Model API.

## Главное изменение №1: bridge и Go package теперь обновляются вместе

Раньше можно было случайно жить с устаревшим native bridge дольше, чем с Go
кодом. На `v0.2.x` это больше не считается допустимым состоянием.

Теперь при старте сравниваются:

- `protocol_version`;
- `witgo_version`;
- `bridge_version`;
- `features`.

Что это значит для миграции:

- нельзя брать bridge из старого релиза и ожидать fallback;
- нельзя обновить только Go package и не обновить native artifacts;
- embedded bridge и source tree должны быть синхронны.

## Главное изменение №2: validation стала частью нормального startup flow

Вместо модели “открыть plugin и надеяться, что совпадёт” теперь нормальный путь
такой:

```go
report, err := contract.ValidatePlugin(path)
if err != nil {
	return err
}
if err := report.Err(); err != nil {
	return err
}
```

Потом уже:

```go
plugin, err := contract.OpenPlugin(path, imports)
```

Если раньше вы сразу открывали plugin, стоит перенести validation в startup.

## Главное изменение №3: generated contract теперь строгий

Generator теперь производит:

- `PluginPing`;
- `ValidatePlugin`;
- `CheckPlugin`;
- structural signatures функций.

Это значит, что несовместимость теперь ловится не только по имени функции, но и
по форме параметров/результатов.

Если раньше plugin “случайно проходил”, хотя ABI уже разошёлся, теперь он будет
отклонён правильно.

## Главное изменение №4: legacy API больше не является частью публичной линии

Если вы обновляетесь с очень ранней версии, ориентируйтесь на такие опоры:

- используйте только Component Model `.wasm`;
- переходите на generated bindings вокруг `world`;
- убирайте старые helper-обёртки, если они были привязаны к pre-Component path;
- не рассчитывайте на старые deprecated CLI-flags.

## Изменения по типам

На `v0.2.x` появились или стали полноценнее:

- `Option[T]`;
- `Result[T,E]`;
- `Char`;
- `Tuple0`...`Tuple16`;
- динамический `Tuple`;
- `Map[K,V]`;
- helper-методы для enum, flags и variant;
- `Handle` для `resource`, `future`, `stream`, `error-context`.

Что важно:

- большие tuple теперь не отклоняются, а используют `witgo.Tuple`;
- `map<K,V>` поддерживается, но ключ должен быть `comparable` в Go;
- `future` и `stream` пока ещё не получили typed high-level consumer API.

## Что обновить в своём проекте

Минимальный чек-лист:

1. Обновить зависимость `github.com/slavkiy/witgo`.
2. Перегенерировать bindings из актуального WIT.
3. Обновить или пересобрать native bridge.
4. Встроить `ValidatePlugin` или `CheckPlugin` в startup path.
5. Перепроверить examples/CI на вашей платформе.
6. Проверить все пути, где используются custom `BridgePath` или env-переменные.
7. Если включён `GoOverlay`, проверить generated wire adapters и обработку
   `WITError[E]` через `errors.As`.

## Что делать, если после обновления всё сломалось

Самые частые причины:

- stale bridge;
- stale generated bindings;
- фактический plugin уже не совпадает с WIT;
- проект ожидал старое поведение legacy API;
- в CI/локально используются разные native artifacts.

Практичный порядок:

1. пересоберите plugin;
2. перегенерируйте Go bindings;
3. проверьте `ValidatePlugin`;
4. проверьте, какой bridge реально поднимается;
5. только потом разбирайте runtime call failures.

## Признаки успешной миграции

- `ValidatePlugin` даёт совместимый report;
- `OpenPlugin` проходит без version/protocol mismatch;
- generated examples и ваши внутренние integration tests проходят на текущем
  bridge;
- в окружении больше нет старых bridge-артефактов, которые могут случайно
  подхватиться раньше новых.
