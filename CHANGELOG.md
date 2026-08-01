# История изменений

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
