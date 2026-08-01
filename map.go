package witgo

import (
	"encoding/json"
	"fmt"
)

// Map represents a Component Model map<K,V> using its ordered pair wire form.
// Go map iteration order is intentionally not part of the ABI.
type Map[K comparable, V any] map[K]V

// NewMap constructs a map with optional initial capacity.
func NewMap[K comparable, V any](capacity ...int) Map[K, V] {
	size := 0
	if len(capacity) > 0 && capacity[0] > 0 {
		size = capacity[0]
	}
	return make(Map[K, V], size)
}

// Get returns a value and whether key exists.
func (m Map[K, V]) Get(key K) (V, bool) {
	value, ok := m[key]
	return value, ok
}

// Put stores value under key and returns the map for fluent construction.
func (m Map[K, V]) Put(key K, value V) Map[K, V] {
	if m == nil {
		m = NewMap[K, V]()
	}
	m[key] = value
	return m
}

// Delete removes key and reports whether it was present.
func (m Map[K, V]) Delete(key K) bool {
	if _, ok := m[key]; !ok {
		return false
	}
	delete(m, key)
	return true
}

// Clone returns an independent shallow copy.
func (m Map[K, V]) Clone() Map[K, V] {
	result := NewMap[K, V](len(m))
	for key, value := range m {
		result[key] = value
	}
	return result
}

func (m Map[K, V]) MarshalJSON() ([]byte, error) {
	pairs := make([][2]any, 0, len(m))
	for key, value := range m {
		pairs = append(pairs, [2]any{key, value})
	}
	return json.Marshal(pairs)
}

func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("cannot decode map into nil receiver")
	}
	var pairs []json.RawMessage
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}
	result := NewMap[K, V](len(pairs))
	for index, raw := range pairs {
		var pair []json.RawMessage
		if err := json.Unmarshal(raw, &pair); err != nil {
			return fmt.Errorf("decode WIT map pair %d: %w", index, err)
		}
		if len(pair) != 2 {
			return fmt.Errorf("WIT map pair %d has %d values, expected 2", index, len(pair))
		}
		var key K
		var value V
		if err := json.Unmarshal(pair[0], &key); err != nil {
			return fmt.Errorf("decode WIT map key %d: %w", index, err)
		}
		if _, duplicate := result[key]; duplicate {
			return fmt.Errorf("WIT map contains duplicate key at pair %d", index)
		}
		if err := json.Unmarshal(pair[1], &value); err != nil {
			return fmt.Errorf("decode WIT map value %d: %w", index, err)
		}
		result[key] = value
	}
	*m = result
	return nil
}
