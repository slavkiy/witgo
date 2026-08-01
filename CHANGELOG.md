# История изменений

## Следующий релиз

- Добавлена Transparent Plugin Composition: WIT import может обслуживаться
  одинаковым типизированным интерфейсом из Go или export другого компонента.
- Добавлены `Host`, provider registry, descriptors, AutoBind, безопасный
  lifecycle, hot replacement, call path, hooks и защита от циклов/reentrancy.
- Generated interfaces унифицированы: методы import и export возвращают `error`.
- WebAssembly providers автоматически компонуются с consumer в одном Wasmtime
  Store. `resource`, `future`, `stream` и `error-context` проходят внутри
  коробки без сериализации через Go; между независимыми Store они по-прежнему
  отклоняются с `ErrCrossRuntimeHandle`.
- Рёбра графа используют полный WIT ID, проверяются на дубликаты, несовместимые
  типы и циклы до instantiation. Одинаковые короткие имена безопасно различаются.

## v0.2.0

- Добавлен строгий version и feature handshake между Go и Rust bridge.
- Добавлена инспекция контракта, валидация компонента, generated ping-manifest
  и структурные сигнатуры функций.
- Добавлены живые runtime-bound handles для WIT `resource`, `future`, `stream`
  и `error-context`.
- Расширены generated bindings и e2e-тесты для enum, flags, variant, resource,
  maps, nested lists и больших tuple.
- Добавлены type-safe `Option`, `Result`, `Char`, `Tuple0`...`Tuple16`,
  динамический `Tuple`, generic `Map`, helper-методы для enum/flags/variant и
  collision-safe generated locals.
- Добавлена CI-матрица для Linux, macOS и Windows на `amd64` и `arm64`.
- Удалены legacy API до Component Model и устаревшие CLI-флаги.

Этот релиз требует bridge protocol `3` и bridge version `0.2.0`. Native
библиотеки из старых релизов отклоняются при запуске.
