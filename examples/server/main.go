package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	contract "github.com/slavkiy/witgo/examples/generate/out"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve server directory")
	}
	wasmFile := filepath.Join(filepath.Dir(sourceFile), "..", "plugin", "plugin.wasm")

	plugin, err := contract.OpenPlugin(wasmFile)
	if err != nil {
		log.Fatal(err)
	}
	metadata, err := plugin.Metadata()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Plugin metadata")
	fmt.Println("Name:", metadata.Name)
	fmt.Println("Version:", metadata.Version)
	fmt.Println("Author:", metadata.Author)
	fmt.Println("Description:", metadata.Description)
}
