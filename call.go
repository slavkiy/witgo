package witgo

import (
	"context"
	"errors"
	"fmt"
)

func (r *Runtime) Call(name string, args ...any) (any, error) {
	return r.CallContext(context.Background(), name, args...)
}

// CallContext invokes a component export and propagates ctx to host imports.
// Cancellation is observed before bridge operations and between bridge messages.
func (r *Runtime) CallContext(ctx context.Context, name string, args ...any) (any, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.bridge == nil {
		return nil, errors.New("component runtime is not initialized")
	}
	if r.compositionHost != nil {
		var cancel context.CancelFunc
		var err error
		ctx, cancel, err = r.compositionHost.enterRuntimeCall(ctx, r.componentFile, name)
		if err != nil {
			return nil, err
		}
		defer cancel()
	}
	if r.effectiveOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.effectiveOptions.Timeout)
		defer cancel()
	}
	if r.bridge.isClosed() {
		return nil, ErrRuntimeClosed
	}
	values, err := r.bridge.call(ctx, name, args)
	if err != nil {
		return nil, classifyCallError(name, err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) == 1 {
		return values[0], nil
	}
	return values, nil
}

func classifyCallError(name string, err error) error {
	text := err.Error()
	if containsAny(text, "all fuel consumed", "out of fuel") {
		return &ExecutionLimitError{Function: name, Limit: ErrFuelExhausted, Cause: err}
	}
	if containsAny(text, "epoch deadline", "interrupt") {
		return &ExecutionLimitError{Function: name, Limit: ErrCallTimeout, Cause: err}
	}
	return fmt.Errorf("call component function %q: %w", name, err)
}

func containsAny(value string, values ...string) bool {
	for _, candidate := range values {
		if len(candidate) <= len(value) {
			for i := 0; i+len(candidate) <= len(value); i++ {
				if equalFoldASCII(value[i:i+len(candidate)], candidate) {
					return true
				}
			}
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
