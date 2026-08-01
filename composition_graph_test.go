package witgo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeEmptyComponent(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("(component)"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNormalizeCompositionUsesExactInterfaceIDs(t *testing.T) {
	root := writeEmptyComponent(t, "root.wasm")
	first := writeEmptyComponent(t, "first.wasm")
	second := writeEmptyComponent(t, "second.wasm")

	plugs, err := normalizeComposition(root, []CompositionPlug{
		{Interface: "alpha:cache/api@1.0.0", Component: first},
		{Interface: "beta:cache/api@1.0.0", Component: second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plugs) != 2 || plugs[0].Interface == plugs[1].Interface {
		t.Fatalf("exact interface identities were collapsed: %#v", plugs)
	}
}

func TestNormalizeCompositionRejectsDuplicateExactInterface(t *testing.T) {
	root := writeEmptyComponent(t, "root.wasm")
	provider := writeEmptyComponent(t, "provider.wasm")
	_, err := normalizeComposition(root, []CompositionPlug{
		{Interface: "test:cache/api@1.0.0", Component: provider},
		{Interface: "test:cache/api@1.0.0", Component: provider},
	})
	if !errors.Is(err, ErrPluginAlreadyRegistered) {
		t.Fatalf("expected duplicate provider error, got %v", err)
	}
}

func TestNormalizeCompositionRejectsPathCycle(t *testing.T) {
	root := writeEmptyComponent(t, "root.wasm")
	provider := writeEmptyComponent(t, "provider.wasm")
	_, err := normalizeComposition(root, []CompositionPlug{{
		Interface: "test:graph/provider@1.0.0",
		Component: provider,
		Dependencies: []CompositionPlug{{
			Interface: "test:graph/root@1.0.0",
			Component: root,
		}},
	}})
	if !errors.Is(err, ErrNestedPluginCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestCompositionContractRemovesOnlySatisfiedImports(t *testing.T) {
	contract := Contract{
		Imports: []string{"alpha:cache/api@1.0.0#get", "beta:cache/api@1.0.0#get"},
		Exports: []string{"test:root/api@1.0.0#run"},
		Signatures: map[string]string{
			"alpha:cache/api@1.0.0#get": "func()->u32",
			"beta:cache/api@1.0.0#get":  "func()->u32",
			"test:root/api@1.0.0#run":   "func()->u32",
		},
	}
	effective := compositionContract(contract, []CompositionPlug{{Interface: "alpha:cache/api@1.0.0", Component: "provider.wasm"}})
	if len(effective.Imports) != 1 || effective.Imports[0] != "beta:cache/api@1.0.0#get" {
		t.Fatalf("wrong remaining imports: %#v", effective.Imports)
	}
	if _, exists := effective.Signatures["alpha:cache/api@1.0.0#get"]; exists {
		t.Fatal("signature of a composed import must not be compared with host imports")
	}
}
