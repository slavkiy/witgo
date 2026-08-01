package witgo

import (
	"context"
	"errors"
	"testing"
)

func TestLoadRuntimeContextStopsBeforeIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := LoadRuntimeContext(ctx, "file-that-must-not-be-opened.wasm")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadRuntimeContext error = %v, want context.Canceled", err)
	}
}

func TestCallContextRejectsNilContext(t *testing.T) {
	var runtime *Runtime
	_, err := runtime.CallContext(nil, "test")
	if err == nil || err.Error() != "context is nil" {
		t.Fatalf("CallContext error = %v, want nil-context error", err)
	}
}
