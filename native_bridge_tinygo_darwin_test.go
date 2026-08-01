//go:build tinygo && cgo && darwin

package witgo

import "testing"

func TestTinyGoDarwinReportsUnsupportedBridgeBackend(t *testing.T) {
	if got := CurrentNativeBridgeBackend(); got != NativeBridgeBackendUnsupported {
		t.Fatalf("CurrentNativeBridgeBackend() = %q, want %q", got, NativeBridgeBackendUnsupported)
	}
}
