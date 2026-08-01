//go:build freebsd && !cgo && !tinygo

package witgo

import "fmt"

const nativeBridgeBackend = NativeBridgeBackendUnsupported

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	return nil, fmt.Errorf("load native component library %q: FreeBSD backend requires CGo", path)
}
