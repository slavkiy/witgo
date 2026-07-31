package witgo

import (
	"bytes"
	"fmt"
)

// Unit represents a WIT type slot without a payload.
type Unit struct{}

// UnitValue constructs a WIT unit value.
func UnitValue() Unit { return Unit{} }

func (Unit) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func (u *Unit) UnmarshalJSON(data []byte) error {
	if u == nil {
		return fmt.Errorf("cannot decode unit into nil receiver")
	}
	if !bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("WIT unit must be null")
	}
	*u = Unit{}
	return nil
}
