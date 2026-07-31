//go:build windows

package witgo

import (
	"github.com/ebitengine/purego"
	"syscall"
)

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	library := &nativeLibrary{closeLibrary: dll.Release}
	bindings := []struct {
		name   string
		target any
	}{
		{"witgo_bridge_new", &library.newHandle}, {"witgo_bridge_send", &library.send},
		{"witgo_bridge_receive", &library.receive}, {"witgo_bridge_free", &library.free},
		{"witgo_bridge_close", &library.closeHandle},
	}
	for _, binding := range bindings {
		proc, err := dll.FindProc(binding.name)
		if err != nil {
			_ = dll.Release()
			return nil, err
		}
		purego.RegisterFunc(binding.target, proc.Addr())
	}
	return library, nil
}
