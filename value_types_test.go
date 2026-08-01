package witgo

import (
	"encoding/json"
	"testing"
)

func TestOptionJSONAndHelpers(t *testing.T) {
	value := Some("ready")
	if got, ok := value.Get(); !ok || got != "ready" || value.Or("fallback") != "ready" {
		t.Fatalf("unexpected some value: %#v", value)
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) != `{"some":"ready"}` {
		t.Fatalf("marshal some = %s, %v", data, err)
	}
	var none Option[string]
	if err := json.Unmarshal([]byte(`{"none":true}`), &none); err != nil || !none.IsNone() || none.Or("fallback") != "fallback" {
		t.Fatalf("decode none = %#v, %v", none, err)
	}
	mapped := MapOption(value, func(value string) int { return len(value) })
	if mapped.Value != 5 || mapped.Pointer() == nil {
		t.Fatalf("mapped option = %#v", mapped)
	}
	nestedData, err := json.Marshal(Some(None[string]()))
	if err != nil || string(nestedData) != `{"some":{"none":true}}` {
		t.Fatalf("nested option = %s, %v", nestedData, err)
	}
}

func TestResultJSONAndMatch(t *testing.T) {
	value := Ok[int, string](42)
	if got, ok := value.GetOK(); !ok || got != 42 || value.IsErr() {
		t.Fatalf("unexpected ok result: %#v", value)
	}
	if got := MatchResult(value, func(number int) string { return "ok" }, func(message string) string { return message }); got != "ok" {
		t.Fatalf("match = %q", got)
	}
	mapped := MapResult(value, func(number int) int64 { return int64(number * 2) })
	if got, ok := mapped.GetOK(); !ok || got != 84 {
		t.Fatalf("mapped result = %#v", mapped)
	}
	data, err := json.Marshal(Err[int]("failed"))
	if err != nil || string(data) != `{"err":"failed"}` {
		t.Fatalf("marshal err = %s, %v", data, err)
	}
	var decoded Result[int, string]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got, ok := decoded.GetErr(); !ok || got != "failed" {
		t.Fatalf("decoded err = %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`{"ok":1,"err":"bad"}`), &decoded); err == nil {
		t.Fatal("ambiguous result was accepted")
	}
}

func TestCharJSON(t *testing.T) {
	value, err := NewChar('Ж')
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) != `"Ж"` {
		t.Fatalf("marshal char = %s, %v", data, err)
	}
	var decoded Char
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Rune() != 'Ж' {
		t.Fatalf("decode char = %q, %v", decoded, err)
	}
	if err := json.Unmarshal([]byte(`"ab"`), &decoded); err == nil {
		t.Fatal("multi-rune char was accepted")
	}
}

func TestTupleJSON(t *testing.T) {
	value := NewTuple3("item", uint32(7), Some(Char('x')))
	data, err := json.Marshal(value)
	if err != nil || string(data) != `["item",7,{"some":"x"}]` {
		t.Fatalf("marshal tuple = %s, %v", data, err)
	}
	var decoded Tuple3[string, uint32, Option[Char]]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.V0 != "item" || decoded.V1 != 7 || decoded.V2.Value != 'x' || !decoded.V2.Some {
		t.Fatalf("decoded tuple = %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`[1,2]`), &decoded); err == nil {
		t.Fatal("wrong tuple arity was accepted")
	}
}

func TestUnitJSON(t *testing.T) {
	data, err := json.Marshal(UnitValue())
	if err != nil || string(data) != "null" {
		t.Fatalf("marshal unit = %s, %v", data, err)
	}
	var value Unit
	if err := json.Unmarshal([]byte("null"), &value); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{}`), &value); err == nil {
		t.Fatal("non-null unit was accepted")
	}
}

func TestMapJSONAndHelpers(t *testing.T) {
	value := NewMap[string, Option[uint32]]().Put("answer", Some(uint32(42)))
	data, err := json.Marshal(value)
	if err != nil || string(data) != `[["answer",{"some":42}]]` {
		t.Fatalf("marshal map = %s, %v", data, err)
	}
	var decoded Map[string, Option[uint32]]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got, ok := decoded.Get("answer"); !ok || got.Value != 42 {
		t.Fatalf("decoded map = %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`[["x",1],["x",2]]`), &decoded); err == nil {
		t.Fatal("duplicate map key was accepted")
	}
}

func TestDynamicTupleHelpers(t *testing.T) {
	tuple := NewTuple("value", float64(7))
	if !tuple.Set(1, uint32(9)) {
		t.Fatal("tuple set failed")
	}
	value, present, err := TupleValue[uint32](tuple, 1)
	if err != nil || !present || value != 9 {
		t.Fatalf("tuple value = %d, %t, %v", value, present, err)
	}
}
