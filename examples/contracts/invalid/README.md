# Диагностика генерации

В этом каталоге лежит заведомо невалидный WIT-контракт для проверки ошибок
генерации, которые рендерит необязательный CLI `witgen`.

Из корня репозитория:

```sh
cd cmd/witgen
go run . \
  -wit ../../examples/contracts/invalid/invalid.wit \
  -out ../../examples/contracts/invalid/out \
  -lang ru
```

Команда должна завершиться с локализованной диагностикой `GenerationFailed`.
Выходной package при этом не создаётся.
