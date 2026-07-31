//go:build !(windows || linux || darwin)

package witgo

import "fmt"

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	return nil, fmt.Errorf("native component library %q is unsupported on this platform", path)
}
