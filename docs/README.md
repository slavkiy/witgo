# Документация

Этот каталог собирает подробную документацию по `witgo`.

## С чего начать

- [Tutorial: Rust Component + Go host](tutorial-rust-component.md)
- [Проверка контрактов](validation.md)
- [Сгенерированный Go-код](generated-code.md)
- [Публичный API](public-api.md)
- [Совместимость с TinyGo и контекстный API](tinygo.md)
- [Автоматические вложенные плагины](nested-plugins.md)
- [Прозрачная композиция плагинов](plugin-composition.md)

## Эксплуатация

- [Troubleshooting](troubleshooting.md)
- [Возможности и ограничения](capabilities.md)
- [Архитектура runtime](architecture.md)
- [Релизный процесс](releasing.md)
- [Migration guide](migration-guide.md)

## Безопасность

- [Security model](../SECURITY.md)

## Как читать этот каталог

- Если вы только начинаете, идите в tutorial и потом в validation.
- Если у вас уже есть plugin, но он не стартует, идите в troubleshooting.
- Если вы обновляете версию `witgo`, сначала откройте migration guide.
- Если вы готовите production rollout, прочитайте architecture, capabilities и
  SECURITY.
