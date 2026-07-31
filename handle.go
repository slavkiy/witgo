package witgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// HandleKind identifies an opaque Component Model handle.
type HandleKind string

const (
	HandleResource     HandleKind = "resource"
	HandleFuture       HandleKind = "future"
	HandleStream       HandleKind = "stream"
	HandleErrorContext HandleKind = "error-context"
)

// Handle is a live Component Model resource, future, stream, or error-context.
// Handles are bound to the Runtime that produced them and may be passed back to
// calls on that Runtime. Close releases the bridge-side handle explicitly.
// Copies of a Handle share the same lifecycle state.
type Handle struct {
	state *handleState
}

type handleState struct {
	mu     sync.Mutex
	bridge *componentBridge
	id     uint64
	kind   HandleKind
	owned  bool
	closed bool
}

type handleWire struct {
	ID    uint64     `json:"$witgo_handle"`
	Kind  HandleKind `json:"kind"`
	Owned bool       `json:"owned,omitempty"`
}

func newHandle(bridge *componentBridge, wire handleWire) Handle {
	return Handle{state: &handleState{bridge: bridge, id: wire.ID, kind: wire.Kind, owned: wire.Owned}}
}

// ID returns the runtime-local handle identifier. It is useful for logging,
// but it is not portable between Runtime values or process executions.
func (h Handle) ID() uint64 {
	if h.state == nil {
		return 0
	}
	return h.state.id
}

// Kind returns the Component Model handle kind.
func (h Handle) Kind() HandleKind {
	if h.state == nil {
		return ""
	}
	return h.state.kind
}

// IsKind reports whether this live handle has kind.
func (h Handle) IsKind(kind HandleKind) bool { return h.Kind() == kind && !h.IsClosed() }

// Valid checks that the handle is initialized, live, and has a known kind.
func (h Handle) Valid() error {
	if h.state == nil {
		return ErrHandleClosed
	}
	if h.IsClosed() {
		return ErrHandleClosed
	}
	switch h.Kind() {
	case HandleResource, HandleFuture, HandleStream, HandleErrorContext:
		return nil
	default:
		return fmt.Errorf("unknown component handle kind %q", h.Kind())
	}
}

func (h Handle) String() string {
	if h.state == nil {
		return "handle<closed>"
	}
	return fmt.Sprintf("%s<%d>", h.Kind(), h.ID())
}

// Owned reports whether a resource handle was lifted as own<T>. It is false
// for borrow<T> and for handle kinds where ownership is implicit.
func (h Handle) Owned() bool { return h.state != nil && h.state.owned }

// IsClosed reports whether the handle or its Runtime has been closed.
func (h Handle) IsClosed() bool {
	if h.state == nil {
		return true
	}
	h.state.mu.Lock()
	closed, bridge := h.state.closed, h.state.bridge
	h.state.mu.Unlock()
	return closed || bridge == nil || bridge.isClosed()
}

// Close explicitly drops a resource or closes a future/stream. Error-context
// handles are removed from the bridge table. Close is idempotent.
func (h Handle) Close() error {
	if h.state == nil {
		return nil
	}
	h.state.mu.Lock()
	if h.state.closed {
		h.state.mu.Unlock()
		return nil
	}
	bridge, id := h.state.bridge, h.state.id
	h.state.closed = true
	h.state.mu.Unlock()
	if bridge == nil || bridge.isClosed() {
		return ErrRuntimeClosed
	}
	if err := bridge.releaseHandle(id); err != nil {
		if !errors.Is(err, ErrRuntimeClosed) {
			h.state.mu.Lock()
			h.state.closed = false
			h.state.mu.Unlock()
		}
		return err
	}
	return nil
}

// CloseHandles closes every handle and returns the first error after all close
// operations have been attempted.
func CloseHandles(handles ...Handle) error {
	var first error
	for _, handle := range handles {
		if err := handle.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// MarshalJSON encodes the runtime-local token understood by the native bridge.
func (h Handle) MarshalJSON() ([]byte, error) {
	if h.state == nil {
		return nil, errors.New("component handle is not initialized")
	}
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	if h.state.closed || h.state.bridge == nil {
		return nil, ErrRuntimeClosed
	}
	return json.Marshal(handleWire{ID: h.state.id, Kind: h.state.kind, Owned: h.state.owned})
}

// UnmarshalJSON rejects detached handle tokens. Handles must be created by a
// Runtime so they cannot accidentally be attached to the wrong Store.
func (h *Handle) UnmarshalJSON(data []byte) error {
	var wire handleWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	return fmt.Errorf("component %s handle %d is detached from its runtime", wire.Kind, wire.ID)
}
