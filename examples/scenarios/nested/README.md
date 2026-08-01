# Автоматический вложенный плагин

Root-компонент импортирует host-функцию, но Go-приложение передаёт пустой
`PluginImports{}`. `witgo` рекурсивно находит
`components/basic/dependencies/host.component.wasm`, проверяет имя и сигнатуру
export-функции, создаёт независимую box и маршрутизирует вызов автоматически.

```sh
go run ./examples/scenarios/nested
```
