//go:build ignore

// Command package-bridge gzip-compresses a native bridge for go:embed.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	hash := sha256.New()
	_, err = io.Copy(writer, io.TeeReader(source, hash))
	check(err)
	check(writer.Close())
	check(destination.Close())
	rawChecksumPath := strings.TrimSuffix(*output, ".gz") + ".sha256"
	rawName := strings.TrimSuffix(filepath.Base(*output), ".gz")
	check(os.WriteFile(rawChecksumPath, []byte(fmt.Sprintf("%x  %s\n", hash.Sum(nil), rawName)), 0o644))
	packaged, err := os.Open(*output)
	check(err)
	packagedHash := sha256.New()
	_, err = io.Copy(packagedHash, packaged)
	check(err)
	check(packaged.Close())
	check(os.WriteFile(*output+".sha256", []byte(fmt.Sprintf("%x  %s\n", packagedHash.Sum(nil), filepath.Base(*output))), 0o644))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
