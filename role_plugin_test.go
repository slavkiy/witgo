//go:build wasm

package witgo

import (
	"errors"
	"testing"
)

func TestWasmBuildUsesPluginRole(t *testing.T) {
	if CurrentExecutionRole() != ExecutionRolePlugin || !IsPluginBuild() || IsHostBuild() {
		t.Fatalf("unexpected execution role %q", CurrentExecutionRole())
	}
	if err := RequirePluginBuild(); err != nil {
		t.Fatalf("RequirePluginBuild() = %v", err)
	}
	if err := RequireHostBuild(); !errors.Is(err, ErrHostOnlyAPI) {
		t.Fatalf("RequireHostBuild() = %v", err)
	}
}
