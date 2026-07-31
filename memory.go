package witgo

import (
	"errors"
	"fmt"
)

var ErrResultTooLarge = errors.New("WebAssembly result exceeds configured limit")

// ReadMemory copies bytes from the default exported memory of a core module.
func (r *Runtime) ReadMemory(offset, length uint32) ([]byte, error) {
	if r == nil || r.Module == nil {
		return nil, fmt.Errorf("runtime core module is not initialized")
	}
	return r.Module.ReadMemory(offset, length)
}

// ReadMemory copies bytes from the "memory" export.
func (r *ModuleRuntime) ReadMemory(offset, length uint32) ([]byte, error) {
	if r == nil || r.Store == nil || r.Instance == nil {
		return nil, fmt.Errorf("module runtime is not initialized")
	}
	if r.limits != nil && r.limits.maxResultBytes > 0 && uint64(length) > r.limits.maxResultBytes {
		return nil, fmt.Errorf("%w: %d bytes requested, limit is %d", ErrResultTooLarge, length, r.limits.maxResultBytes)
	}
	export := r.Instance.GetExport(r.Store, "memory")
	if export == nil || export.Memory() == nil {
		return nil, fmt.Errorf("wasm memory export not found")
	}
	data := export.Memory().UnsafeData(r.Store)
	end := uint64(offset) + uint64(length)
	if end > uint64(len(data)) {
		return nil, fmt.Errorf("wasm memory range [%d:%d] is out of bounds", offset, end)
	}
	result := make([]byte, length)
	copy(result, data[offset:uint32(end)])
	return result, nil
}
