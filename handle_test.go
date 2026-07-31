package witgo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleWireBinding(t *testing.T) {
	bridge := &componentBridge{}
	bound := bridge.bindHandles(map[string]any{
		"nested": []any{map[string]any{
			"$witgo_handle": json.Number("42"),
			"kind":          "resource",
			"owned":         true,
		}},
	}).(map[string]any)
	handle := bound["nested"].([]any)[0].(Handle)
	if handle.ID() != 42 || handle.Kind() != HandleResource || !handle.Owned() || handle.IsClosed() {
		t.Fatalf("bound handle = id:%d kind:%q owned:%v closed:%v", handle.ID(), handle.Kind(), handle.Owned(), handle.IsClosed())
	}
	data, err := json.Marshal(handle)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"$witgo_handle":42,"kind":"resource","owned":true}` {
		t.Fatalf("handle JSON = %s", data)
	}
	bridge.markHandlesConsumed([]uint64{42})
	if !handle.IsClosed() {
		t.Fatal("consumed handle is reported open")
	}
}

func TestDetachedHandleIsRejected(t *testing.T) {
	var handle Handle
	err := json.Unmarshal([]byte(`{"$witgo_handle":7,"kind":"future"}`), &handle)
	if err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("Unmarshal error = %v", err)
	}
}
