//go:build tinygo && !cgo && (windows || linux || darwin)

package witgo

import "fmt"

const nativeBridgeBackend = NativeBridgeBackendUnsupported

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	return nil, fmt.Errorf("load native component library %q: TinyGo backend requires CGo", path)
}
