//go:build ((linux && !android) || (darwin && !ios)) && !tinygo

package witgo

import "github.com/ebitengine/purego"

const nativeBridgeBackend = NativeBridgeBackendPureGo

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	library := &nativeLibrary{closeLibrary: func() error { return purego.Dlclose(handle) }}
	purego.RegisterLibFunc(&library.newHandle, handle, "witgo_bridge_new")
	purego.RegisterLibFunc(&library.send, handle, "witgo_bridge_send")
	purego.RegisterLibFunc(&library.receive, handle, "witgo_bridge_receive")
	purego.RegisterLibFunc(&library.free, handle, "witgo_bridge_free")
	purego.RegisterLibFunc(&library.closeHandle, handle, "witgo_bridge_close")
	return library, nil
}
