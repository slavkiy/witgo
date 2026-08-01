# Вложенные плагины

> Этот механизм предназначен для заранее упакованного дерева компонентов и
> сохранён для совместимости. Для модели, где plugin не знает путь provider и
> все связи явно контролирует Go-host, используйте
> [прозрачную композицию](plugin-composition.md).

`witgo` автоматически различает две роли сборки:

- нативная программа (`GOARCH` не равен `wasm`) - хост;
- WebAssembly-сборка (`GOARCH=wasm`) - плагин.

Роль доступна через `witgo.CurrentExecutionRole()`, `witgo.IsHostBuild()` и
`witgo.IsPluginBuild()`. Поднимать native bridge и создавать `PluginHost` может
только хост. При ошибочном вызове из WASM возвращается `ErrHostOnlyAPI`.

## Как плагин выбирает зависимости

WIT imports описывают, какие интерфейсы нужны плагину. Сам плагин связывает
интерфейс с относительным путём в манифесте `witgo:plugin-manifest`:

```json
{
  "dependencies": {
    "example:plugins/image@1.0.0": "dependencies/image.wasm",
    "example:plugins/storage@1.0.0#open": "dependencies/storage.wasm"
  }
}
```

Запись полного имени функции имеет приоритет над записью интерфейса. Путь всегда
отсчитывается от каталога родительского компонента. Абсолютные пути и выход через
`..` за разрешённую хостом область отклоняются с `ErrNestedPluginPathDenied`.

Для бинарного компонента манифест встраивается в custom section:

```go
component, err := os.ReadFile("application.wasm")
if err != nil {
	return err
}

component, err = witgo.EmbedPluginManifest(component, witgo.PluginManifest{
	Dependencies: map[string]string{
		"example:plugins/image@1.0.0": "dependencies/image.wasm",
	},
})
```

Для разработки поддерживается sidecar-файл
`application.wasm.witgo.json` с тем же JSON. В production рекомендуется
встроенный манифест.

## Использование хостом

Обычному приложению не нужны `SearchPaths`, resolver или пустой набор imports:

```go
plugin, err := contract.OpenPluginContext(ctx, "./plugins/application.wasm")
if err != nil {
	return err
}
defer plugin.Close()
```

Хост читает манифест, проверяет точные имена и структурные сигнатуры exports,
загружает дочерние компоненты и связывает вызовы. Ручной generated host всё ещё
имеет приоритет и передаётся только когда приложение намеренно заменяет import:

```go
plugin, err := contract.OpenPluginContext(ctx, path, contract.PluginImports{
	Image: imageHost,
})
```

`SearchPaths` и `NestedPluginResolver` сохранены только как совместимый fallback
для старых компонентов без манифеста. Новый код не должен зависеть от
сканирования каталогов.

## Политика хоста и независимые коробки

Плагин выбирает относительное расположение зависимостей, но не расширяет свои
права. Хост может лишь сузить или явно расширить допустимые корни:

```go
host, err := witgo.NewPluginHost(witgo.NestedPluginOptions{
	AllowedRoots: []string{"./plugins"},
})
if err != nil {
	return err
}
defer host.Close()

plugin, err := contract.OpenPluginWithOptionsContext(
	ctx,
	"./plugins/application.wasm",
	witgo.RuntimeOptions{
		PluginHost:       host,
		FuelPerCall:     1_000_000,
		MemoryLimitBytes: 256 << 20,
		InstanceLimit:    64,
		Timeout:          time.Second,
	},
)
```

Каждая top-level загрузка создаёт независимую `PluginBox`: собственные runtime,
handles, состояние и lifecycle. Все вызовы внутри коробки маршрутизируются одним
`PluginHost`, но состояние разных коробок не разделяется.

Бюджеты `Fuel`, `FuelPerCall`, `MemoryLimitBytes` и `InstanceLimit` делятся между
родителем и его прямыми детьми. Каждый ребёнок рекурсивно делит только полученную
долю. Поэтому потомок никогда не получает больше ресурса, чем было выделено его
родителю. Слишком маленький неделимый бюджет даёт `ErrNestedPluginBudget`.
`Timeout` передаётся через общий `context.Context`, поэтому вложенная цепочка не
может продлить исходный deadline. Capability policy остаётся политикой хоста.

## Глубина, циклы и lifecycle

Глубина ациклической цепочки библиотекой не ограничена:

```text
A → B → C → D → ...
```

Цикл `A → B → A` отклоняется с `ErrNestedPluginCycle`. Отсутствующий provider
даёт `ErrNestedPluginNotFound`, несовпадающая сигнатура - ошибку контракта.

`Close` корневого плагина закрывает всю коробку. `RestartContext` создаёт её с
нуля: повторно читает манифесты, выполняет contract validation и version
handshake, затем закрывает предыдущую коробку.

## Границы Store и живые handles

Между runtime безопасно передаются обычные WIT values: числа, строки, records,
lists, tuples, options, results, enums, flags и variants. Для живых `resource`,
`future`, `stream` и `error-context` runtime автоматически собирает всю цепочку
WebAssembly provider в один Component и создаёт один Wasmtime Store на коробку.
Внутри такой коробки handles проходят напрямую через Canonical ABI.

Между независимыми коробками и через Go callback handle не переносится:
возвращается `ErrCrossRuntimeHandle`. Это обязательная проверка ownership, а не
настраиваемое ограничение.

Автоматическая вложенность не является загрузкой native-библиотеки внутри WASM.
Плагин объявляет WIT imports и пути, а единственный внешний хост создаёт runtime,
применяет политику и маршрутизирует типизированные вызовы.
