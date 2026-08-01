//go:build tinygo && cgo && windows

package witgo

/*
#include "native_bridge_tinygo_windows.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const nativeBridgeBackend = NativeBridgeBackendCGo

func loadNativeLibrary(path string) (*nativeLibrary, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	handle := C.witgo_library_open(cPath)
	if handle == nil {
		return nil, fmt.Errorf("open native bridge: Windows error %d", uint32(C.witgo_library_error()))
	}

	symbol := func(name string) (unsafe.Pointer, error) {
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		value := C.witgo_library_symbol(handle, cName)
		if value == nil {
			return nil, fmt.Errorf("resolve native bridge symbol %q: Windows error %d", name, uint32(C.witgo_library_error()))
		}
		return value, nil
	}

	newSymbol, err := symbol("witgo_bridge_new")
	if err != nil {
		C.witgo_library_close(handle)
		return nil, err
	}
	sendSymbol, err := symbol("witgo_bridge_send")
	if err != nil {
		C.witgo_library_close(handle)
		return nil, err
	}
	receiveSymbol, err := symbol("witgo_bridge_receive")
	if err != nil {
		C.witgo_library_close(handle)
		return nil, err
	}
	freeSymbol, err := symbol("witgo_bridge_free")
	if err != nil {
		C.witgo_library_close(handle)
		return nil, err
	}
	closeSymbol, err := symbol("witgo_bridge_close")
	if err != nil {
		C.witgo_library_close(handle)
		return nil, err
	}

	return &nativeLibrary{
		closeLibrary: func() error {
			if C.witgo_library_close(handle) != 0 {
				return fmt.Errorf("close native bridge library: Windows error %d", uint32(C.witgo_library_error()))
			}
			return nil
		},
		newHandle: func() uintptr { return uintptr(C.witgo_call_new(newSymbol)) },
		send: func(instance uintptr, data *byte, length uintptr) int32 {
			return int32(C.witgo_call_send(sendSymbol, unsafe.Pointer(instance), (*C.uint8_t)(unsafe.Pointer(data)), C.uintptr_t(length)))
		},
		receive: func(instance uintptr, length *uintptr) *byte {
			return (*byte)(unsafe.Pointer(C.witgo_call_receive(receiveSymbol, unsafe.Pointer(instance), (*C.uintptr_t)(unsafe.Pointer(length)))))
		},
		free: func(data *byte, length uintptr) {
			C.witgo_call_free(freeSymbol, (*C.uint8_t)(unsafe.Pointer(data)), C.uintptr_t(length))
		},
		closeHandle: func(instance uintptr) { C.witgo_call_close(closeSymbol, unsafe.Pointer(instance)) },
	}, nil
}
