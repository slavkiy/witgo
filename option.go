package witgo

import (
	"encoding/json"
	"fmt"
)

// Option represents a WIT option<T> without using nil as application state.
type Option[T any] struct {
	Value T
	Some  bool
}

// Some constructs an option containing value.
func Some[T any](value T) Option[T] { return Option[T]{Value: value, Some: true} }

// None constructs an empty option.
func None[T any]() Option[T] { return Option[T]{} }

// OptionFromPointer converts nil to None and a non-nil pointer to Some.
func OptionFromPointer[T any](value *T) Option[T] {
	if value == nil {
		return None[T]()
	}
	return Some(*value)
}

// IsSome reports whether the option contains a value.
func (o Option[T]) IsSome() bool { return o.Some }

// IsNone reports whether the option is empty.
func (o Option[T]) IsNone() bool { return !o.Some }

// Get returns the value and whether it is present.
func (o Option[T]) Get() (T, bool) { return o.Value, o.Some }

// Or returns the contained value or fallback.
func (o Option[T]) Or(fallback T) T {
	if o.Some {
		return o.Value
	}
	return fallback
}

// Pointer returns a pointer to a copy of the contained value or nil.
func (o Option[T]) Pointer() *T {
	if !o.Some {
		return nil
	}
	value := o.Value
	return &value
}

// MapOption transforms a present value and preserves None.
func MapOption[T, U any](option Option[T], transform func(T) U) Option[U] {
	if option.IsNone() {
		return None[U]()
	}
	return Some(transform(option.Value))
}

// FlatMapOption chains an option-producing transformation.
func FlatMapOption[T, U any](option Option[T], transform func(T) Option[U]) Option[U] {
	if option.IsNone() {
		return None[U]()
	}
	return transform(option.Value)
}

func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.Some {
		return []byte(`{"none":true}`), nil
	}
	return json.Marshal(struct {
		Some T `json:"some"`
	}{Some: o.Value})
}

func (o *Option[T]) UnmarshalJSON(data []byte) error {
	if o == nil {
		return fmt.Errorf("cannot decode option into nil receiver")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if some, ok := fields["some"]; ok && len(fields) == 1 {
		var value T
		if err := json.Unmarshal(some, &value); err != nil {
			return err
		}
		*o = Some(value)
		return nil
	}
	if none, ok := fields["none"]; ok && len(fields) == 1 {
		var marker bool
		if err := json.Unmarshal(none, &marker); err != nil || !marker {
			return fmt.Errorf("WIT option none marker must be true")
		}
		*o = None[T]()
		return nil
	}
	return fmt.Errorf("WIT option must contain exactly one of some or none")
}
