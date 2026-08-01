//go:build android || ios

package witgo

import "fmt"

const nativeBridgeBackend = NativeBridgeBackendUnsupported

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	return nil, fmt.Errorf("load native component library %q: dynamic component bridges are unsupported on mobile targets", path)
}
