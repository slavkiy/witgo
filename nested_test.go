package witgo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNestedLoadStateHasNoDepthLimitAndRejectsCycles(t *testing.T) {
	state := newNestedLoadState(newPluginHost(NestedPluginOptions{}))
	for index := 0; index < 1024; index++ {
		if err := state.enter(fmt.Sprintf("plugin-%d.wasm", index)); err != nil {
			t.Fatalf("enter depth %d: %v", index, err)
		}
	}
	if err := state.enter("plugin-0.wasm"); !errors.Is(err, ErrNestedPluginCycle) {
		t.Fatalf("cycle error = %v, want ErrNestedPluginCycle", err)
	}
}

func TestNestedCandidatesAreRecursiveAndBoxesAreIndependent(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "root.wasm")
	child := filepath.Join(dir, "children", "child.wasm")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("root"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}

	candidates, err := nestedCandidates(parent, NestedPluginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0] != child {
		t.Fatalf("candidates = %v, want [%s]", candidates, child)
	}

	host, err := NewPluginHost(NestedPluginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	left, right := newNestedLoadState(host), newNestedLoadState(host)
	if err := left.enter(parent); err != nil {
		t.Fatal(err)
	}
	if err := right.enter(parent); err != nil {
		t.Fatalf("independent box treated another box as a cycle: %v", err)
	}
}

func TestNestedCandidateLimit(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "root.wasm")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.wasm", "b.wasm"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := nestedCandidates(parent, NestedPluginOptions{MaxCandidates: 1})
	if err == nil {
		t.Fatal("nestedCandidates unexpectedly accepted too many candidates")
	}
}

func TestPluginHostControlsIndependentBoxes(t *testing.T) {
	host, err := NewPluginHost(NestedPluginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	left := &Runtime{pluginHost: host}
	right := &Runtime{pluginHost: host}
	if err := host.register(left); err != nil {
		t.Fatal(err)
	}
	if err := host.register(right); err != nil {
		t.Fatal(err)
	}
	if got := host.BoxCount(); got != 2 {
		t.Fatalf("BoxCount() = %d, want 2", got)
	}
	if err := left.Close(); err != nil {
		t.Fatal(err)
	}
	if got := host.BoxCount(); got != 1 {
		t.Fatalf("BoxCount() after close = %d, want 1", got)
	}
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}
	if got := host.BoxCount(); got != 0 {
		t.Fatalf("BoxCount() after host close = %d, want 0", got)
	}
	if err := host.register(&Runtime{}); err == nil {
		t.Fatal("closed host accepted a new box")
	}
}

func TestPluginHostFreezesPolicyRoots(t *testing.T) {
	host, err := NewPluginHost(NestedPluginOptions{
		AllowedRoots: []string{"plugins"},
		SearchPaths:  []string{"legacy"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range append(host.options.AllowedRoots, host.options.SearchPaths...) {
		if !filepath.IsAbs(root) {
			t.Fatalf("policy root was not frozen as an absolute path: %q", root)
		}
	}
}

func TestSplitNestedBudgetNeverAmplifiesResources(t *testing.T) {
	options := RuntimeOptions{
		Fuel:             100,
		MemoryLimitBytes: 1000,
		InstanceLimit:    10,
		MaxResultBytes:   4096,
	}
	parent, child, err := splitNestedBudget(options, 3)
	if err != nil {
		t.Fatal(err)
	}
	if parent.Fuel != 25 || child.Fuel != 25 {
		t.Fatalf("fuel shares = parent %d, child %d", parent.Fuel, child.Fuel)
	}
	if parent.MemoryLimitBytes != 250 || child.MemoryLimitBytes != 250 {
		t.Fatalf("memory shares = parent %d, child %d", parent.MemoryLimitBytes, child.MemoryLimitBytes)
	}
	if parent.InstanceLimit != 2 || child.InstanceLimit != 2 {
		t.Fatalf("instance shares = parent %d, child %d", parent.InstanceLimit, child.InstanceLimit)
	}
	if parent.MaxResultBytes != options.MaxResultBytes || child.MaxResultBytes != options.MaxResultBytes {
		t.Fatal("non-budget safety limit unexpectedly changed")
	}

	_, grandchild, err := splitNestedBudget(child, 1)
	if err != nil {
		t.Fatal(err)
	}
	if grandchild.Fuel > child.Fuel || grandchild.MemoryLimitBytes > child.MemoryLimitBytes || grandchild.InstanceLimit > child.InstanceLimit {
		t.Fatalf("descendant budget grew: child=%#v descendant=%#v", child, grandchild)
	}
}

func TestSplitNestedBudgetRejectsUnrepresentableShare(t *testing.T) {
	_, _, err := splitNestedBudget(RuntimeOptions{FuelPerCall: 1}, 1)
	if !errors.Is(err, ErrNestedPluginBudget) {
		t.Fatalf("error = %v, want ErrNestedPluginBudget", err)
	}
}
