package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/slavkiy/witgo"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve collections example directory")
	}
	pluginFile := filepath.Join(filepath.Dir(sourceFile), "component.wasm")

	runtimeValue, err := witgo.LoadRuntime(pluginFile)
	if err != nil {
		log.Fatal(err)
	}
	defer runtimeValue.Close()

	matrix := []any{[]any{uint32(1), uint32(2)}, []any{uint32(3)}, []any{}}
	matrixResult, err := runtimeValue.Call("examples:collections/api@1.0.0#roundtrip-list", matrix)
	if err != nil {
		log.Fatal(err)
	}

	variant := map[string]any{"case": "some", "value": map[string]any{"some": "ok"}}
	variantResult, err := runtimeValue.Call("examples:collections/api@1.0.0#roundtrip-variant", variant)
	if err != nil {
		log.Fatal(err)
	}

	var decodedVariant map[string]any
	variantData, err := json.Marshal(variantResult)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(variantData, &decodedVariant); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Roundtrip nested lists:", matrixResult)
	fmt.Println("Roundtrip variant:", decodedVariant)
}
