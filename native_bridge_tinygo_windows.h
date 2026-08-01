#ifndef WITGO_NATIVE_BRIDGE_TINYGO_WINDOWS_H
#define WITGO_NATIVE_BRIDGE_TINYGO_WINDOWS_H

#include <stdint.h>

void* witgo_library_open(const char* path);
void* witgo_library_symbol(void* library, const char* name);
int witgo_library_close(void* library);
uint32_t witgo_library_error(void);

void* witgo_call_new(void* function);
int32_t witgo_call_send(void* function, void* handle, const uint8_t* data, uintptr_t length);
uint8_t* witgo_call_receive(void* function, void* handle, uintptr_t* length);
void witgo_call_free(void* function, uint8_t* data, uintptr_t length);
void witgo_call_close(void* function, void* handle);

#endif
