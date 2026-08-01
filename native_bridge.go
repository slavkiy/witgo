package witgo

import (
	"errors"
	"fmt"
	"unsafe"
)

// NativeBridgeBackend identifies the platform loader selected at build time.
type NativeBridgeBackend string

const (
	NativeBridgeBackendPureGo      NativeBridgeBackend = "purego"
	NativeBridgeBackendCGo         NativeBridgeBackend = "cgo"
	NativeBridgeBackendUnsupported NativeBridgeBackend = "unsupported"
)

// CurrentNativeBridgeBackend reports which loader this binary uses.
func CurrentNativeBridgeBackend() NativeBridgeBackend { return nativeBridgeBackend }

type nativeLibrary struct {
	closeLibrary func() error
	newHandle    func() uintptr
	send         func(uintptr, *byte, uintptr) int32
	receive      func(uintptr, *uintptr) *byte
	free         func(*byte, uintptr)
	closeHandle  func(uintptr)
}

type nativeBridge struct {
	library *nativeLibrary
	handle  uintptr
}

func openNativeBridge(path string) (*nativeBridge, error) {
	library, err := loadNativeLibrary(path)
	if err != nil {
		return nil, err
	}
	handle := library.newHandle()
	if handle == 0 {
		_ = library.closeLibrary()
		return nil, errors.New("create native bridge handle")
	}
	return &nativeBridge{library: library, handle: handle}, nil
}

func (b *nativeBridge) write(data []byte) error {
	if len(data) == 0 {
		return errors.New("native bridge message is empty")
	}
	if code := b.library.send(b.handle, &data[0], uintptr(len(data))); code != 0 {
		return fmt.Errorf("native bridge send failed with status %d", code)
	}
	return nil
}

func (b *nativeBridge) read() ([]byte, error) {
	var length uintptr
	pointer := b.library.receive(b.handle, &length)
	if pointer == nil {
		return nil, errors.New("native bridge channel closed")
	}
	defer b.library.free(pointer, length)
	return append([]byte(nil), unsafe.Slice(pointer, length)...), nil
}

func (b *nativeBridge) close() error {
	if b == nil || b.handle == 0 {
		return nil
	}
	b.library.closeHandle(b.handle)
	b.handle = 0
	return b.library.closeLibrary()
}
