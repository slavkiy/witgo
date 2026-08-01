package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	contract "github.com/slavkiy/witgo/examples/contracts/basic/out"
)

type host struct{}

func (host) ProcessString(_ context.Context, value string) (string, error) {
	return "HOST:" + value, nil
}

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve server scenario directory")
	}
	wasmFile := filepath.Join(filepath.Dir(sourceFile), "..", "..", "components", "basic", "component.wasm")

	ctx := context.Background()
	plugin, err := contract.OpenPluginContext(ctx, wasmFile, contract.PluginImports{Host: host{}})
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()
	metadata, err := plugin.PluginInfo.Metadata(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Plugin metadata")
	fmt.Println("Name:", metadata.Name)
	fmt.Println("Version:", metadata.Version)
	fmt.Println("Author:", metadata.Author)
	fmt.Println("Description:", metadata.Description)
}
