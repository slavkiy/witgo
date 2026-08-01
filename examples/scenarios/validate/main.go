package main

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/slavkiy/witgo"
	contract "github.com/slavkiy/witgo/examples/generate/out"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve validate example directory")
	}
	pluginFile := filepath.Join(filepath.Dir(sourceFile), "..", "plugin", "component.wasm")

	report, err := contract.ValidatePlugin(pluginFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Compatible:", report.Compatible)
	fmt.Println("Summary:", report.Summary())
	fmt.Println("Required imports:", report.Expected.ImportNames())
	fmt.Println("Provided exports:", report.Expected.ExportNames())

	if err := contract.CheckPlugin(pluginFile); err != nil {
		if errors.Is(err, witgo.ErrContractMismatch) {
			log.Fatalf("plugin contract mismatch: %v", err)
		}
		log.Fatal(err)
	}
}
