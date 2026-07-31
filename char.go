package witgo

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Char is a Unicode scalar value encoded according to the WIT char ABI.
type Char rune

// NewChar validates value and constructs a Char.
func NewChar(value rune) (Char, error) {
	if !utf8.ValidRune(value) || value >= 0xD800 && value <= 0xDFFF {
		return 0, fmt.Errorf("invalid WIT char U+%04X", value)
	}
	return Char(value), nil
}

// ParseChar decodes a string containing exactly one Unicode scalar value.
func ParseChar(value string) (Char, error) {
	decoded, size := utf8.DecodeRuneInString(value)
	if size == 0 || size != len(value) {
		return 0, fmt.Errorf("WIT char must contain exactly one Unicode scalar value")
	}
	return NewChar(decoded)
}

// Rune returns the Go rune representation.
func (c Char) Rune() rune { return rune(c) }

func (c Char) String() string { return string(c) }

func (c Char) MarshalJSON() ([]byte, error) {
	if _, err := NewChar(rune(c)); err != nil {
		return nil, err
	}
	return json.Marshal(string(c))
}

func (c *Char) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	result, err := ParseChar(value)
	if err != nil {
		return err
	}
	*c = result
	return nil
}
