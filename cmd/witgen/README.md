# witgen

`witgen` - необязательный generation-only CLI. Он рендерит детерминированные
диагностики локально, умеет русский и английский язык сообщений и не имеет
зависимостей, кроме `witgo`.

```sh
cd cmd/witgen
go install .

witgen \
  -wit ./wit \
  -wit-mode package \
  -out ./internal/contract \
  -lang ru
```

Флаги:

- `-wit`: WIT-файл или каталог;
- `-wit-mode`: `file`, `package` (только файлы в корне), `tree`
  (рекурсивно) или совместимый режим `auto`;
- `-out`: каталог для output;
- `-package`: необязательное переопределение имени Go package;
- `-filename`: необязательное имя generated файла;
- `-lang`: язык диагностики; по умолчанию берётся из `LANG`.

CLI, библиотека `witgo` и generated code поддерживают Go 1.18 и новее.
