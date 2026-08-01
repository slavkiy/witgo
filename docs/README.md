# Документация

Ниже только документы, которые реально нужны для работы с `witgo`.

## База

- [README](../README.md) - быстрый старт и модель использования.
- [Архитектура runtime](architecture.md) - доверенные границы, composition и bridge.
- [Публичный API](public-api.md) - что считается основным API хоста и генератора.
- [Возможности и ограничения](capabilities.md) - текущие гарантии и сознательные ограничения.

## Для ежедневной работы

- [Проверка контрактов](validation.md)
- [Сгенерированный Go-код](generated-code.md)
- [Go type overlays](go-overlays.md)
- [Прозрачная композиция плагинов](plugin-composition.md)
- [Автоматические вложенные плагины](nested-plugins.md)
- [Troubleshooting](troubleshooting.md)

## Для специальных сценариев

- [Tutorial: Rust Component + Go host](tutorial-rust-component.md)
- [Совместимость с TinyGo и контекстный API](tinygo.md)
- [Migration guide](migration-guide.md)
- [Релизный процесс](releasing.md)
- [Security model](../SECURITY.md)
