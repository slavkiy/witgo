package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	contract "github.com/slavkiy/witgo/examples/generate/out"
)

type host struct{}

func (host) ProcessString(value string) string {
	return "HOST:" + value
}

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve server directory")
	}
	wasmFile := filepath.Join(filepath.Dir(sourceFile), "..", "plugin", "component.wasm")

	plugin, err := contract.OpenPlugin(wasmFile, contract.PluginImports{Host: host{}})
	if err != nil {
		log.Fatal(err)
	}
	defer plugin.Close()
	metadata, err := plugin.PluginInfo.Metadata()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Plugin metadata")
	fmt.Println("Name:", metadata.Name)
	fmt.Println("Version:", metadata.Version)
	fmt.Println("Author:", metadata.Author)
	fmt.Println("Description:", metadata.Description)
}
