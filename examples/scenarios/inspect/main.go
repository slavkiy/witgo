package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/slavkiy/witgo"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve inspect example directory")
	}
	pluginFile := filepath.Join(filepath.Dir(sourceFile), "..", "..", "components", "basic", "component.wasm")

	contract, err := witgo.InspectComponent(pluginFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Imports:")
	for _, name := range contract.ImportNames() {
		signature, _ := contract.Signature(name)
		fmt.Printf("  %s %s\n", name, signature)
	}

	fmt.Println("Exports:")
	for _, name := range contract.ExportNames() {
		signature, _ := contract.Signature(name)
		fmt.Printf("  %s %s\n", name, signature)
	}
}
