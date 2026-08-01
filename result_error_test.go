package witgo

import (
	"errors"
	"fmt"
	"testing"
)

func TestWITErrorPreservesPayloadThroughWrapping(t *testing.T) {
	err := fmt.Errorf("save user: %w", NewWITError(struct{ Code string }{Code: "conflict"}))
	var target WITError[struct{ Code string }]
	if !errors.As(err, &target) || target.Value.Code != "conflict" {
		t.Fatalf("errors.As did not preserve WIT payload: %#v", target)
	}
}
