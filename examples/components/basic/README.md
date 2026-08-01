# Базовый component

Этот каталог хранит общий component-артефакт для базового типизированного
сценария.

Состав:

- `component.wat` - текстовое представление component;
- `component.wasm` - готовый бинарный артефакт;
- `main.go` - helper, который печатает ожидаемый путь к `.wasm`.

Этот component используют:

- `examples/scenarios/server`;
- `examples/scenarios/validate`;
- `examples/scenarios/inspect`.
