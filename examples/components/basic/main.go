// Command plugin prints the expected output path for the shared example
// component binary.
package main

import (
	"fmt"
	"path/filepath"
)

func main() {
	path, _ := filepath.Abs(filepath.Join("examples", "components", "basic", "component.wasm"))
	fmt.Println("build the WIT component and place it at:", path)
}
