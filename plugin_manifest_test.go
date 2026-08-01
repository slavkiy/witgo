package witgo

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmbedAndReadPluginManifest(t *testing.T) {
	component := []byte{'\x00', 'a', 's', 'm', '\x0d', '\x00', '\x01', '\x00'}
	want := PluginManifest{Dependencies: map[string]string{
		"example:app/storage@1.0.0": "plugins/storage.wasm",
	}}
	embedded, err := EmbedPluginManifest(component, want)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EmbedPluginManifest(embedded, want); err == nil {
		t.Fatal("duplicate embedded manifest was accepted")
	}
	filename := filepath.Join(t.TempDir(), "plugin.wasm")
	if err := os.WriteFile(filename, embedded, 0o600); err != nil {
		t.Fatal(err)
	}
	got, found, err := ReadPluginManifest(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadPluginManifest() = (%#v, %v), want (%#v, true)", got, found, want)
	}
}

func TestReadPluginManifestSidecar(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "plugin.wat")
	if err := os.WriteFile(filename, []byte("(component)"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename+".witgo.json", []byte(`{"dependencies":{"example:app/log@1.0.0":"log.wasm"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, found, err := ReadPluginManifest(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !found || manifest.Dependencies["example:app/log@1.0.0"] != "log.wasm" {
		t.Fatalf("unexpected sidecar manifest: %#v, found=%v", manifest, found)
	}
}

func TestEmbedPluginManifestRejectsCoreModule(t *testing.T) {
	core := []byte{'\x00', 'a', 's', 'm', '\x01', '\x00', '\x00', '\x00'}
	if _, err := EmbedPluginManifest(core, PluginManifest{}); err == nil {
		t.Fatal("core module was accepted as a component")
	}
}

func TestResolveManifestDependencyPolicy(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "plugin.wasm")
	child := filepath.Join(dir, "children", "child.wasm")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("(component)"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveManifestDependency(parent, filepath.Join("children", "child.wasm"), nil)
	if err != nil || got != child {
		t.Fatalf("allowed child = %q, %v; want %q", got, err, child)
	}
	if _, err := resolveManifestDependency(parent, filepath.Join("..", "outside.wasm"), nil); !errors.Is(err, ErrNestedPluginPathDenied) {
		t.Fatalf("outside path error = %v, want ErrNestedPluginPathDenied", err)
	}
	if _, err := resolveManifestDependency(parent, child, nil); !errors.Is(err, ErrNestedPluginPathDenied) {
		t.Fatalf("absolute path error = %v, want ErrNestedPluginPathDenied", err)
	}
}

func TestPluginManifestDependencyPrefersFunction(t *testing.T) {
	manifest := PluginManifest{Dependencies: map[string]string{
		"example:app/api@1.0.0":      "interface.wasm",
		"example:app/api@1.0.0#read": "function.wasm",
	}}
	if got := pluginManifestDependency(manifest, "example:app/api@1.0.0#read"); got != "function.wasm" {
		t.Fatalf("function dependency = %q", got)
	}
	if got := pluginManifestDependency(manifest, "example:app/api@1.0.0#write"); got != "interface.wasm" {
		t.Fatalf("interface dependency = %q", got)
	}
}
