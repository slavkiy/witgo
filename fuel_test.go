package witgo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRuntimeOptionsRejectAmbiguousFuel(t *testing.T) {
	err := validateRuntimeOptions(RuntimeOptions{Fuel: 1, FuelPerCall: 1})
	if err == nil {
		t.Fatal("expected ambiguous fuel configuration to fail")
	}
}

func TestRuntimeOptionsRejectNegativeLimits(t *testing.T) {
	tests := []RuntimeOptions{
		{Timeout: -time.Second},
		{MemoryLimitBytes: -1},
		{InstanceLimit: -1},
	}
	for _, options := range tests {
		if err := validateRuntimeOptions(options); err == nil {
			t.Fatalf("options %#v should fail", options)
		}
	}
}

func TestRuntimeOptionsRejectInvalidBridgeDigest(t *testing.T) {
	for _, digest := range []string{"short", strings.Repeat("z", 64)} {
		if err := validateRuntimeOptions(RuntimeOptions{BridgeSHA256: digest}); err == nil {
			t.Fatalf("digest %q should fail", digest)
		}
	}
}

func TestCallErrorClassification(t *testing.T) {
	fuel := classifyCallError("spin", errors.New("wasm trap: all fuel consumed"))
	if !errors.Is(fuel, ErrFuelExhausted) {
		t.Fatalf("error = %v, want ErrFuelExhausted", fuel)
	}
	timeout := classifyCallError("spin", errors.New("wasm trap: epoch deadline reached"))
	if !errors.Is(timeout, ErrCallTimeout) {
		t.Fatalf("error = %v, want ErrCallTimeout", timeout)
	}
}

func TestCoreModuleHasMigrationError(t *testing.T) {
	// Valid core-Wasm header followed by an empty module body.
	_, err := LoadRuntimeFromBytes([]byte{'\x00', 'a', 's', 'm', '\x01', '\x00', '\x00', '\x00'})
	if !errors.Is(err, ErrCoreModule) {
		t.Fatalf("error = %v, want ErrCoreModule", err)
	}
}
