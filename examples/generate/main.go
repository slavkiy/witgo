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
		log.Fatal("cannot resolve example directory")
	}
	exampleDir := filepath.Dir(sourceFile)
	outputDir := filepath.Join(exampleDir, "out")

	err := witgo.Generate(witgo.Config{
		WIT:    filepath.Join(exampleDir, "wit"),
		Output: outputDir,
		// Package is intentionally omitted. It is derived from
		// `package examples:contract@1.0.0` as "contract".
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("generated:", filepath.Join(outputDir, "bindings.gen.go"))
}
