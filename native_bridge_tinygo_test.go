//go:build tinygo && cgo && (linux || windows || freebsd)

package witgo

import "testing"

func TestTinyGoUsesCGoBridgeBackend(t *testing.T) {
	if got := CurrentNativeBridgeBackend(); got != NativeBridgeBackendCGo {
		t.Fatalf("CurrentNativeBridgeBackend() = %q, want %q", got, NativeBridgeBackendCGo)
	}
}
