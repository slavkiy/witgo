package generator

import (
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateBindings(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	contract := `package simple:model@1.0.0;

interface types {
    record user {
        name: string,
        age: s64,
    }
}

interface host {
    use types.{user};
    current-user: func() -> user;
}

interface sso {
    use types.{user};
    get: func() -> user;
    save: func(user: user) -> bool;
}

world plugin {
    import host;
    export sso;
}`
	if err := os.WriteFile(filepath.Join(witDir, "user.wit"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Generate(Config{
		WIT:    witDir,
		Output: outputDir,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	filename := filepath.Join(outputDir, DefaultFilename)
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"type User struct",
		"type Host interface",
		"type SSO interface",
		"type Plugin struct",
		"func OpenPlugin",
		"binding userBinding",
		"func (value User) Save() (bool, error)",
		`WITPackageID        = "simple:model@1.0.0"`,
		`runtime.Call("simple:model/sso@1.0.0#get")`,
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	for _, unwanted := range []string{
		"type Caller interface",
		"type PluginImports struct",
		"type PluginClient struct",
		"func LowerUser",
		"func LiftUser",
		"map[string]any",
	} {
		if strings.Contains(string(source), unwanted) {
			t.Errorf("generated source unexpectedly contains %q", unwanted)
		}
	}
	parseGenerated(t, filename, source)
}

func TestGenerateRejectsDifferentPackages(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(witDir, "a.wit"), []byte("package one:a;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(witDir, "b.wit"), []byte("package two:b;"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Generate(Config{WIT: witDir, Output: outputDir, Package: "bindings"})
	if err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("Generate error = %v, want package mismatch", err)
	}
}

func parseGenerated(t *testing.T, filename string, source []byte) {
	t.Helper()
	fileSet := token.NewFileSet()
	_, err := goparser.ParseFile(fileSet, filename, source, goparser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated Go: %v", err)
	}
}
