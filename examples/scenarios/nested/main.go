package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	contract "github.com/slavkiy/witgo/examples/contracts/basic/out"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("не удалось определить каталог примера")
	}
	component := filepath.Join(filepath.Dir(sourceFile), "..", "..", "components", "basic", "component.wasm")

	ctx := context.Background()
	plugin, err := contract.OpenPluginContext(ctx, component)
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()

	metadata, err := plugin.PluginInfo.Metadata(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Имя из вложенного plugin-host:", metadata.Name)
}
