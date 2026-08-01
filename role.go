package witgo

import "errors"

// ExecutionRole identifies which side of the Component Model boundary this
// binary implements.
type ExecutionRole string

const (
	ExecutionRoleHost   ExecutionRole = "host"
	ExecutionRolePlugin ExecutionRole = "plugin"
)

var (
	ErrHostOnlyAPI   = errors.New("witgo API is only available in a host build")
	ErrPluginOnlyAPI = errors.New("witgo API is only available in a WebAssembly plugin build")
)

// CurrentExecutionRole is selected at compile time. Native targets are hosts;
// GOARCH=wasm targets are plugins.
func CurrentExecutionRole() ExecutionRole { return executionRole }

func IsHostBuild() bool   { return executionRole == ExecutionRoleHost }
func IsPluginBuild() bool { return executionRole == ExecutionRolePlugin }

// RequireHostBuild rejects APIs that need the native Component Model runtime
// when the current binary is itself a WebAssembly plugin.
func RequireHostBuild() error {
	if !IsHostBuild() {
		return ErrHostOnlyAPI
	}
	return nil
}

// RequirePluginBuild can be used by generated guest entry points to reject an
// accidental native build.
func RequirePluginBuild() error {
	if !IsPluginBuild() {
		return ErrPluginOnlyAPI
	}
	return nil
}
