//go:build tinygo && cgo && windows

package witgo

/*
#include <windows.h>
#include <stdint.h>
#include <stdlib.h>

typedef void* (*witgo_new_fn)(void);
typedef int32_t (*witgo_send_fn)(void*, const uint8_t*, uintptr_t);
typedef uint8_t* (*witgo_receive_fn)(void*, uintptr_t*);
typedef void (*witgo_free_fn)(uint8_t*, uintptr_t);
typedef void (*witgo_close_fn)(void*);

static void* witgo_library_open(const char* path) {
	int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
	if (length == 0) return NULL;
	wchar_t* wide = (wchar_t*)malloc((size_t)length * sizeof(wchar_t));
	if (wide == NULL) { SetLastError(ERROR_NOT_ENOUGH_MEMORY); return NULL; }
	if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, length) == 0) { free(wide); return NULL; }
	HMODULE library = LoadLibraryW(wide);
	free(wide);
	return (void*)library;
}
static void* witgo_library_symbol(void* library, const char* name) { return (void*)GetProcAddress((HMODULE)library, name); }
static int witgo_library_close(void* library) { return FreeLibrary((HMODULE)library) ? 0 : -1; }
static uint32_t witgo_library_error(void) { return (uint32_t)GetLastError(); }

static void* witgo_call_new(void* function) { return ((witgo_new_fn)function)(); }
static int32_t witgo_call_send(void* function, void* handle, const uint8_t* data, uintptr_t length) { return ((witgo_send_fn)function)(handle, data, length); }
static uint8_t* witgo_call_receive(void* function, void* handle, uintptr_t* length) { return ((witgo_receive_fn)function)(handle, length); }
static void witgo_call_free(void* function, uint8_t* data, uintptr_t length) { ((witgo_free_fn)function)(data, length); }
static void witgo_call_close(void* function, void* handle) { ((witgo_close_fn)function)(handle); }
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
