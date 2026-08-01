# Tutorial: Rust Component + Go host

Этот tutorial показывает минимальный, но реалистичный путь:

1. описать контракт в WIT;
2. сгенерировать Go bindings;
3. собрать Rust Component plugin;
4. загрузить его из Go;
5. проверить контракт до запуска.

Пример использует текущую публичную модель `witgo` на релизной линии `v0.2.x`.

## Что получится

На стороне Rust будет plugin, который экспортирует `metadata.get`, а на стороне
Go - host, который реализует import `host.process-string`.

Идея та же, что и в `examples/contracts/basic`, `examples/components/basic` и
`examples/scenarios/server`, но здесь шаги расписаны подряд.

## 1. Создайте WIT-контракт

Например, `wit/plugin.wit`:

```wit
package example:plugins@1.0.0;

interface metadata {
    record info {
        name: string,
        version: string,
        author: string,
        description: string,
    }

    get: func() -> info;
}

interface host {
    process-string: func(value: string) -> string;
}

world plugin {
    import host;
    export metadata;
}
```

## 2. Сгенерируйте Go bindings

Создайте `generate.go`:

```go
//go:build ignore

package main

import (
	"log"

	"github.com/slavkiy/witgo"
)

func main() {
	err := witgo.Generate(witgo.Config{
		WIT:     "./wit",
		Output:  "./internal/contract",
		Package: "contract",
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Запуск:

```sh
go run generate.go
```

После этого появится `internal/contract/bindings.gen.go`.

## 3. Подготовьте Rust plugin

Для Rust Component удобнее всего использовать `cargo component`.

Ожидаемая структура проекта:

```text
plugin-rs/
  Cargo.toml
  src/lib.rs
  wit/
    plugin.wit
```

Скопируйте ваш `plugin.wit` в каталог `wit/`.

### Пример `Cargo.toml`

```toml
[package]
name = "plugin-rs"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
wit-bindgen = "0.34"
```

Точные версии crate могут меняться, поэтому перед production-сборкой лучше
свериться с актуальной документацией `cargo component` и `wit-bindgen`.

### Пример `src/lib.rs`

```rust
use wit_bindgen::generate;

generate!({
    world: "plugin",
});

struct Component;

impl Guest for Component {
    fn get() -> exports::example::plugins::metadata::Info {
        let name = example::plugins::host::process_string("image-resizer");
        exports::example::plugins::metadata::Info {
            name,
            version: "1.4.0".to_string(),
            author: "Example Team".to_string(),
            description: "Resizes uploaded images and creates previews.".to_string(),
        }
    }
}

export!(Component);
```

## 4. Соберите Component

Типичный путь:

```sh
cargo component build --release
```

На выходе нужен именно WebAssembly Component `.wasm`, а не обычный core Wasm
module.

Если ваш toolchain кладёт артефакт по другому пути, это нормально. Для `witgo`
важен сам файл component.

## 5. Реализуйте Go host

```go
package main

import (
	"context"
	"fmt"
	"log"

	contract "example.com/myapp/internal/contract"
)

type pluginHost struct{}

func (pluginHost) ProcessString(_ context.Context, value string) (string, error) {
	return "HOST:" + value, nil
}

func main() {
	ctx := context.Background()
	report, err := contract.ValidatePluginContext(ctx, "./plugin.component.wasm")
	if err != nil {
		log.Fatal(err)
	}
	if err := report.Err(); err != nil {
		log.Fatal(err)
	}

	plugin, err := contract.OpenPluginContext(ctx, "./plugin.component.wasm", contract.PluginImports{
		Host: pluginHost{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()

	info, err := plugin.Metadata.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info.Name)
	fmt.Println(info.Version)
	fmt.Println(info.Author)
}
```

## 6. Что именно проверяет `ValidatePlugin`

До instantiation `witgo` сравнивает:

- import-функции;
- export-функции;
- версии package/interface как часть имени;
- structural signatures параметров и результатов.

Это означает, что несовместимый plugin отсекается до реального запуска guest
кода.

## 7. Полезный workflow для разработки

Во время разработки удобно держать такой цикл:

1. правите `wit/*.wit`;
2. запускаете `go run generate.go`;
3. пересобираете Rust Component;
4. запускаете `contract.ValidatePlugin(...)`;
5. только потом гоняете end-to-end сценарии.

Так быстрее ловятся ошибки контракта, чем если сразу идти в runtime call.

## 8. Что делать дальше

- Если plugin не открывается, читайте [Troubleshooting](troubleshooting.md).
- Если нужна матрица типов и ограничений, откройте [capabilities.md](capabilities.md).
- Если обновляетесь со старой версии, откройте [Migration guide](migration-guide.md).
