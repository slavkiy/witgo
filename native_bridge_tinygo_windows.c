//go:build tinygo && cgo && windows

#ifdef _WIN32

#include "native_bridge_tinygo_windows.h"

#include <stdlib.h>
#include <windows.h>

typedef void* (*witgo_new_fn)(void);
typedef int32_t (*witgo_send_fn)(void*, const uint8_t*, uintptr_t);
typedef uint8_t* (*witgo_receive_fn)(void*, uintptr_t*);
typedef void (*witgo_free_fn)(uint8_t*, uintptr_t);
typedef void (*witgo_close_fn)(void*);

void* witgo_library_open(const char* path) {
	int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
	if (length == 0) return NULL;
	wchar_t* wide = (wchar_t*)malloc((size_t)length * sizeof(wchar_t));
	if (wide == NULL) {
		SetLastError(ERROR_NOT_ENOUGH_MEMORY);
		return NULL;
	}
	if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, length) == 0) {
		free(wide);
		return NULL;
	}
	HMODULE library = LoadLibraryW(wide);
	free(wide);
	return (void*)library;
}

void* witgo_library_symbol(void* library, const char* name) {
	return (void*)GetProcAddress((HMODULE)library, name);
}

int witgo_library_close(void* library) {
	return FreeLibrary((HMODULE)library) ? 0 : -1;
}

uint32_t witgo_library_error(void) {
	return (uint32_t)GetLastError();
}

void* witgo_call_new(void* function) {
	return ((witgo_new_fn)function)();
}

int32_t witgo_call_send(void* function, void* handle, const uint8_t* data, uintptr_t length) {
	return ((witgo_send_fn)function)(handle, data, length);
}

uint8_t* witgo_call_receive(void* function, void* handle, uintptr_t* length) {
	return ((witgo_receive_fn)function)(handle, length);
}

void witgo_call_free(void* function, uint8_t* data, uintptr_t length) {
	((witgo_free_fn)function)(data, length);
}

void witgo_call_close(void* function, void* handle) {
	((witgo_close_fn)function)(handle);
}

#endif
