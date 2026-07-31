// Command plugin prints the expected output path for the Component Model
// plugin. Build the guest with cargo-component (or another WIT-compatible
// toolchain) and place the resulting binary there.
package main

import (
	"fmt"
	"path/filepath"
)

func main() {
	path, _ := filepath.Abs(filepath.Join("examples", "plugin", "plugin.component.wasm"))
	fmt.Println("build the WIT component and place it at:", path)
}
