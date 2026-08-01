//go:build !wasm

package witgo

import (
	"errors"
	"testing"
)

func TestNativeBuildUsesHostRole(t *testing.T) {
	if CurrentExecutionRole() != ExecutionRoleHost || !IsHostBuild() || IsPluginBuild() {
		t.Fatalf("unexpected execution role %q", CurrentExecutionRole())
	}
	if err := RequireHostBuild(); err != nil {
		t.Fatalf("RequireHostBuild() = %v", err)
	}
	if err := RequirePluginBuild(); !errors.Is(err, ErrPluginOnlyAPI) {
		t.Fatalf("RequirePluginBuild() = %v", err)
	}
}
