package witgo

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Result is a type-safe WIT result<OK, Err>. Exactly one branch is active.
type Result[T, E any] struct {
	ok    T
	err   E
	isErr bool
}

// Ok constructs a successful result.
func Ok[T, E any](value T) Result[T, E] { return Result[T, E]{ok: value} }

// Err constructs a failed result.
func Err[T, E any](value E) Result[T, E] {
	return Result[T, E]{err: value, isErr: true}
}

// IsOK reports whether this is the successful branch.
func (r Result[T, E]) IsOK() bool { return !r.isErr }

// IsErr reports whether this is the error branch.
func (r Result[T, E]) IsErr() bool { return r.isErr }

// GetOK returns the success value and whether that branch is active.
func (r Result[T, E]) GetOK() (T, bool) { return r.ok, !r.isErr }

// GetErr returns the error value and whether that branch is active.
func (r Result[T, E]) GetErr() (E, bool) { return r.err, r.isErr }

// Or returns the success value or fallback.
func (r Result[T, E]) Or(fallback T) T {
	if r.isErr {
		return fallback
	}
	return r.ok
}

// MapResult transforms the successful branch.
func MapResult[T, E, U any](result Result[T, E], transform func(T) U) Result[U, E] {
	if result.isErr {
		return Err[U](result.err)
	}
	return Ok[U, E](transform(result.ok))
}

// MapResultErr transforms the error branch.
func MapResultErr[T, E, F any](result Result[T, E], transform func(E) F) Result[T, F] {
	if result.isErr {
		return Err[T](transform(result.err))
	}
	return Ok[T, F](result.ok)
}

// Match evaluates the callback for the active branch.
func MatchResult[T, E, Value any](r Result[T, E], ok func(T) Value, fail func(E) Value) Value {
	if r.isErr {
		return fail(r.err)
	}
	return ok(r.ok)
}

func (r Result[T, E]) MarshalJSON() ([]byte, error) {
	if r.isErr {
		return json.Marshal(struct {
			Err E `json:"err"`
		}{Err: r.err})
	}
	return json.Marshal(struct {
		OK T `json:"ok"`
	}{OK: r.ok})
}

func (r *Result[T, E]) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("cannot decode result into nil receiver")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	okData, hasOK := fields["ok"]
	errData, hasErr := fields["err"]
	if hasOK == hasErr || len(fields) != 1 {
		return fmt.Errorf("WIT result must contain exactly one of ok or err")
	}
	if hasErr {
		var value E
		if err := json.Unmarshal(errData, &value); err != nil {
			return fmt.Errorf("decode WIT result err: %w", err)
		}
		*r = Err[T](value)
		return nil
	}
	var value T
	if err := json.Unmarshal(okData, &value); err != nil {
		return fmt.Errorf("decode WIT result ok: %w", err)
	}
	*r = Ok[T, E](value)
	return nil
}
