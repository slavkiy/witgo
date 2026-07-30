package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v47"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve plugin directory")
	}
	pluginDir := filepath.Dir(sourceFile)

	source, err := os.ReadFile(filepath.Join(pluginDir, "plugin.wat"))
	if err != nil {
		log.Fatal(err)
	}
	wasm, err := wasmtime.Wat2Wasm(string(source))
	if err != nil {
		log.Fatal(err)
	}

	output := filepath.Join(pluginDir, "plugin.wasm")
	if err := os.WriteFile(output, wasm, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("built:", output)
}
