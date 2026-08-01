//go:build tinygo && cgo && darwin && !ios

package witgo

import "fmt"

const nativeBridgeBackend = NativeBridgeBackendUnsupported

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	return nil, fmt.Errorf("load native component library %q: TinyGo for macOS cannot dynamically link dlopen symbols", path)
}
