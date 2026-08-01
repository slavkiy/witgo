# witgen

`witgen` - необязательный generation-only CLI. Он рендерит детерминированные
диагностики локально, умеет русский и английский язык сообщений и не имеет
зависимостей, кроме `witgo`.

```sh
cd cmd/witgen
go install .

witgen \
  -wit ./wit \
  -out ./internal/contract \
  -lang ru
```

Флаги:

- `-wit`: WIT-файл или каталог;
- `-out`: каталог для output;
- `-package`: необязательное переопределение имени Go package;
- `-filename`: необязательное имя generated файла;
- `-lang`: язык диагностики; по умолчанию берётся из `LANG`.

CLI, библиотека `witgo` и generated code поддерживают Go 1.18 и новее.
