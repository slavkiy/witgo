# Проверка контракта

Generated packages умеют проверять WebAssembly Component до того, как он попадёт
в production-путь приложения:

```go
report, err := contract.ValidatePlugin("./plugin.wasm")
if err != nil {
	return fmt.Errorf("inspect plugin: %w", err)
}
if err := report.Err(); err != nil {
	return err
}
```

Inspection загружает и разбирает Component, но останавливается на bridge
`pong`. Она не instantiate-ит Component, не линкует host callbacks, не
выполняет `start` и не вызывает guest code.

## Что сравнивается

Generated contract и фактический Component manifest содержат:

- полные имена import-функций;
- полные имена export-функций;
- версии package и interface как часть этих имён;
- детерминированные structural signatures параметров и результатов.

Signatures рекурсивно описывают primitive types, records, lists, maps, tuples,
options, results, variants, enums и flags. Плагин не может пройти проверку
только за счёт правильного имени функции при другом ABI.

Resource ownership появляется в сигнатурах как `own` или `borrow`, поэтому ABI
можно проверить без создания resource и без выполнения guest-кода.

## Подробный отчёт

```go
if !report.Compatible {
	log.Printf("missing imports: %v", report.Imports.Missing)
	log.Printf("unexpected imports: %v", report.Imports.Unexpected)
	log.Printf("missing exports: %v", report.Exports.Missing)
	log.Printf("unexpected exports: %v", report.Exports.Unexpected)
	for _, mismatch := range report.Signatures {
		log.Printf("%s: expected %s, actual %s",
			mismatch.Function, mismatch.Expected, mismatch.Actual)
	}
}
```

`ValidatePlugin` использует второй return value только для операционных ошибок:
нечитаемый файл, невалидный Wasm, несовместимый bridge и тому подобное.
Контрактная несовместимость возвращается как успешный `ValidationReport` с
`Compatible == false`.

## Проверка только на успех/ошибку

Generated `CheckPlugin` удобен для startup-кода:

```go
if err := contract.CheckPlugin(path); err != nil {
	if errors.Is(err, witgo.ErrContractMismatch) {
		var mismatch *witgo.ContractValidationError
		if errors.As(err, &mismatch) {
			log.Print(mismatch.Report.Summary())
		}
	}
	return err
}
```

`ValidationReport.Err()` и `witgo.RequireCompatible(report)` дают тот же
переход к `error`, если работаете напрямую с low-level API.

## Компоненты в памяти

Если приложение получает Component из уже доверенного storage layer, можно не
создавать собственный временный файл:

```go
report, err := witgo.ValidateComponentBytes(componentBytes, contract.PluginPing())
```

Библиотека сама создаст приватный временный component-файл, потому что
Wasmtime загружает Components по пути, а затем удалит его после inspection.

## Инспекция возможностей

Raw manifest полезен ещё до появления capability policy:

```go
manifest, err := witgo.InspectComponent(path)
if err != nil {
	return err
}
for _, required := range manifest.ImportNames() {
	log.Printf("plugin requires %s", required)
}
```

Используйте `ImportNames`, `ExportNames` и `FunctionNames`, а не прямую мутацию
полей `Contract`: accessor-методы возвращают отсортированные копии.

Два cached или удалённо полученных manifest можно сравнить и без открытия
bridge:

```go
report, err := witgo.CompareContracts(expected, actual)
```
