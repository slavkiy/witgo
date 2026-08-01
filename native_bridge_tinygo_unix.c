//go:build cgo && (freebsd || (tinygo && linux && !android))

#if !defined(_WIN32) && !defined(__APPLE__)

#include "native_bridge_tinygo_unix.h"

extern void* dlopen(const char*, int);
extern void* dlsym(void*, const char*);
extern int dlclose(void*);
extern char* dlerror(void);

#define WITGO_RTLD_NOW 2
#if defined(__APPLE__)
#define WITGO_RTLD_LOCAL 4
#else
#define WITGO_RTLD_LOCAL 0
#endif

typedef void* (*witgo_new_fn)(void);
typedef int32_t (*witgo_send_fn)(void*, const uint8_t*, uintptr_t);
typedef uint8_t* (*witgo_receive_fn)(void*, uintptr_t*);
typedef void (*witgo_free_fn)(uint8_t*, uintptr_t);
typedef void (*witgo_close_fn)(void*);

void* witgo_library_open(const char* path) { return dlopen(path, WITGO_RTLD_NOW | WITGO_RTLD_LOCAL); }
void* witgo_library_symbol(void* library, const char* name) { return dlsym(library, name); }
int witgo_library_close(void* library) { return dlclose(library); }
const char* witgo_library_error(void) {
	const char* value = dlerror();
	return value == NULL ? "unknown dynamic loader error" : value;
}

void* witgo_call_new(void* function) { return ((witgo_new_fn)function)(); }
int32_t witgo_call_send(void* function, void* handle, const uint8_t* data, uintptr_t length) { return ((witgo_send_fn)function)(handle, data, length); }
uint8_t* witgo_call_receive(void* function, void* handle, uintptr_t* length) { return ((witgo_receive_fn)function)(handle, length); }
void witgo_call_free(void* function, uint8_t* data, uintptr_t length) { ((witgo_free_fn)function)(data, length); }
void witgo_call_close(void* function, void* handle) { ((witgo_close_fn)function)(handle); }

#endif
