package witgo

import (
	"encoding/json"
	"fmt"
)

func decodeTuple(data []byte, targets ...any) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != len(targets) {
		return fmt.Errorf("WIT tuple has %d values, expected %d", len(values), len(targets))
	}
	for index := range targets {
		if err := json.Unmarshal(values[index], targets[index]); err != nil {
			return fmt.Errorf("decode WIT tuple value %d: %w", index, err)
		}
	}
	return nil
}

type Tuple0 struct{}

func NewTuple0() Tuple0                       { return Tuple0{} }
func (Tuple0) Values() []any                  { return []any{} }
func (t Tuple0) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple0) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data)
}

type Tuple1[A any] struct{ V0 A }

func NewTuple1[A any](v0 A) Tuple1[A]            { return Tuple1[A]{V0: v0} }
func (t Tuple1[A]) Values() []any                { return []any{t.V0} }
func (t Tuple1[A]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple1[A]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0)
}

type Tuple2[A, B any] struct {
	V0 A
	V1 B
}

func NewTuple2[A, B any](v0 A, v1 B) Tuple2[A, B]   { return Tuple2[A, B]{V0: v0, V1: v1} }
func (t Tuple2[A, B]) Values() []any                { return []any{t.V0, t.V1} }
func (t Tuple2[A, B]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple2[A, B]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1)
}

type Tuple3[A, B, C any] struct {
	V0 A
	V1 B
	V2 C
}

func NewTuple3[A, B, C any](v0 A, v1 B, v2 C) Tuple3[A, B, C] {
	return Tuple3[A, B, C]{V0: v0, V1: v1, V2: v2}
}
func (t Tuple3[A, B, C]) Values() []any                { return []any{t.V0, t.V1, t.V2} }
func (t Tuple3[A, B, C]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple3[A, B, C]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2)
}

type Tuple4[A, B, C, D any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
}

func NewTuple4[A, B, C, D any](v0 A, v1 B, v2 C, v3 D) Tuple4[A, B, C, D] {
	return Tuple4[A, B, C, D]{V0: v0, V1: v1, V2: v2, V3: v3}
}
func (t Tuple4[A, B, C, D]) Values() []any                { return []any{t.V0, t.V1, t.V2, t.V3} }
func (t Tuple4[A, B, C, D]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple4[A, B, C, D]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3)
}

type Tuple5[A, B, C, D, E any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
}

func NewTuple5[A, B, C, D, E any](v0 A, v1 B, v2 C, v3 D, v4 E) Tuple5[A, B, C, D, E] {
	return Tuple5[A, B, C, D, E]{V0: v0, V1: v1, V2: v2, V3: v3, V4: v4}
}
func (t Tuple5[A, B, C, D, E]) Values() []any                { return []any{t.V0, t.V1, t.V2, t.V3, t.V4} }
func (t Tuple5[A, B, C, D, E]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple5[A, B, C, D, E]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4)
}

type Tuple6[A, B, C, D, E, F any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
	V5 F
}

func NewTuple6[A, B, C, D, E, F any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F) Tuple6[A, B, C, D, E, F] {
	return Tuple6[A, B, C, D, E, F]{V0: v0, V1: v1, V2: v2, V3: v3, V4: v4, V5: v5}
}
func (t Tuple6[A, B, C, D, E, F]) Values() []any                { return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5} }
func (t Tuple6[A, B, C, D, E, F]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple6[A, B, C, D, E, F]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5)
}

type Tuple7[A, B, C, D, E, F, G any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
	V5 F
	V6 G
}

func NewTuple7[A, B, C, D, E, F, G any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G) Tuple7[A, B, C, D, E, F, G] {
	return Tuple7[A, B, C, D, E, F, G]{V0: v0, V1: v1, V2: v2, V3: v3, V4: v4, V5: v5, V6: v6}
}
func (t Tuple7[A, B, C, D, E, F, G]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6}
}
func (t Tuple7[A, B, C, D, E, F, G]) MarshalJSON() ([]byte, error) { return json.Marshal(t.Values()) }
func (t *Tuple7[A, B, C, D, E, F, G]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6)
}

type Tuple8[A, B, C, D, E, F, G, H any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
	V5 F
	V6 G
	V7 H
}

func NewTuple8[A, B, C, D, E, F, G, H any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H) Tuple8[A, B, C, D, E, F, G, H] {
	return Tuple8[A, B, C, D, E, F, G, H]{V0: v0, V1: v1, V2: v2, V3: v3, V4: v4, V5: v5, V6: v6, V7: v7}
}
func (t Tuple8[A, B, C, D, E, F, G, H]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7}
}
func (t Tuple8[A, B, C, D, E, F, G, H]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple8[A, B, C, D, E, F, G, H]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7)
}

type Tuple9[A, B, C, D, E, F, G, H, I any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
	V5 F
	V6 G
	V7 H
	V8 I
}

func NewTuple9[A, B, C, D, E, F, G, H, I any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I) Tuple9[A, B, C, D, E, F, G, H, I] {
	return Tuple9[A, B, C, D, E, F, G, H, I]{v0, v1, v2, v3, v4, v5, v6, v7, v8}
}
func (t Tuple9[A, B, C, D, E, F, G, H, I]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8}
}
func (t Tuple9[A, B, C, D, E, F, G, H, I]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple9[A, B, C, D, E, F, G, H, I]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8)
}

type Tuple10[A, B, C, D, E, F, G, H, I, J any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
	V5 F
	V6 G
	V7 H
	V8 I
	V9 J
}

func NewTuple10[A, B, C, D, E, F, G, H, I, J any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J) Tuple10[A, B, C, D, E, F, G, H, I, J] {
	return Tuple10[A, B, C, D, E, F, G, H, I, J]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9}
}
func (t Tuple10[A, B, C, D, E, F, G, H, I, J]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9}
}
func (t Tuple10[A, B, C, D, E, F, G, H, I, J]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple10[A, B, C, D, E, F, G, H, I, J]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9)
}

type Tuple11[A, B, C, D, E, F, G, H, I, J, K any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
}

func NewTuple11[A, B, C, D, E, F, G, H, I, J, K any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K) Tuple11[A, B, C, D, E, F, G, H, I, J, K] {
	return Tuple11[A, B, C, D, E, F, G, H, I, J, K]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10}
}
func (t Tuple11[A, B, C, D, E, F, G, H, I, J, K]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10}
}
func (t Tuple11[A, B, C, D, E, F, G, H, I, J, K]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple11[A, B, C, D, E, F, G, H, I, J, K]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10)
}

type Tuple12[A, B, C, D, E, F, G, H, I, J, K, L any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
	V11 L
}

func NewTuple12[A, B, C, D, E, F, G, H, I, J, K, L any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K, v11 L) Tuple12[A, B, C, D, E, F, G, H, I, J, K, L] {
	return Tuple12[A, B, C, D, E, F, G, H, I, J, K, L]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11}
}
func (t Tuple12[A, B, C, D, E, F, G, H, I, J, K, L]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10, t.V11}
}
func (t Tuple12[A, B, C, D, E, F, G, H, I, J, K, L]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple12[A, B, C, D, E, F, G, H, I, J, K, L]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10, &t.V11)
}

type Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
	V11 L
	V12 M
}

func NewTuple13[A, B, C, D, E, F, G, H, I, J, K, L, M any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K, v11 L, v12 M) Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M] {
	return Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12}
}
func (t Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10, t.V11, t.V12}
}
func (t Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple13[A, B, C, D, E, F, G, H, I, J, K, L, M]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10, &t.V11, &t.V12)
}

type Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
	V11 L
	V12 M
	V13 N
}

func NewTuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K, v11 L, v12 M, v13 N) Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N] {
	return Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13}
}
func (t Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10, t.V11, t.V12, t.V13}
}
func (t Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple14[A, B, C, D, E, F, G, H, I, J, K, L, M, N]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10, &t.V11, &t.V12, &t.V13)
}

type Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
	V11 L
	V12 M
	V13 N
	V14 O
}

func NewTuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K, v11 L, v12 M, v13 N, v14 O) Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O] {
	return Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14}
}
func (t Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10, t.V11, t.V12, t.V13, t.V14}
}
func (t Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple15[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10, &t.V11, &t.V12, &t.V13, &t.V14)
}

type Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P any] struct {
	V0  A
	V1  B
	V2  C
	V3  D
	V4  E
	V5  F
	V6  G
	V7  H
	V8  I
	V9  J
	V10 K
	V11 L
	V12 M
	V13 N
	V14 O
	V15 P
}

func NewTuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P any](v0 A, v1 B, v2 C, v3 D, v4 E, v5 F, v6 G, v7 H, v8 I, v9 J, v10 K, v11 L, v12 M, v13 N, v14 O, v15 P) Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P] {
	return Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P]{v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14, v15}
}
func (t Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P]) Values() []any {
	return []any{t.V0, t.V1, t.V2, t.V3, t.V4, t.V5, t.V6, t.V7, t.V8, t.V9, t.V10, t.V11, t.V12, t.V13, t.V14, t.V15}
}
func (t Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
}
func (t *Tuple16[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P]) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("cannot decode tuple into nil receiver")
	}
	return decodeTuple(data, &t.V0, &t.V1, &t.V2, &t.V3, &t.V4, &t.V5, &t.V6, &t.V7, &t.V8, &t.V9, &t.V10, &t.V11, &t.V12, &t.V13, &t.V14, &t.V15)
}
