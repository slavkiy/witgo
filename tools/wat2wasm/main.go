package main

import (
	"flag"
	"fmt"
	"os"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v28"
)

func main() {
	input := flag.String("in", "", "path to .wat input")
	output := flag.String("out", "", "path to .wasm output")
	flag.Parse()

	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: go run . -in input.wat -out output.wasm")
		os.Exit(2)
	}

	source, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *input, err)
		os.Exit(1)
	}

	wasm, err := wasmtime.Wat2Wasm(string(source))
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert %s: %v\n", *input, err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, wasm, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *output, err)
		os.Exit(1)
	}
}
