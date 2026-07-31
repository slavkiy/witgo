//go:build ignore

// Command package-bridge gzip-compresses a native bridge for go:embed.
package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	input := flag.String("input", "", "native bridge executable")
	output := flag.String("output", "", "destination .gz file")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "input and output are required")
		os.Exit(2)
	}
	source, err := os.Open(*input)
	check(err)
	defer source.Close()
	check(os.MkdirAll(filepath.Dir(*output), 0o755))
	destination, err := os.Create(*output)
	check(err)
	writer, err := gzip.NewWriterLevel(destination, gzip.BestCompression)
	check(err)
	_, err = io.Copy(writer, source)
	check(err)
	check(writer.Close())
	check(destination.Close())
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
