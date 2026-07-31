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
		"type pluginSSOClient struct",
		"func OpenPlugin",
		"func OpenPluginWithOptions",
		"binding userBinding",
		"func (value User) Save() (bool, error)",
		`WITPackageID        = "simple:model@1.0.0"`,
		`runtime.Call("simple:model/sso@1.0.0#get")`,
		`witgo.LoadRuntimeWithContract`,
		`type PluginImports struct`,
		`func PluginPing() witgo.Contract`,
		`func ValidatePlugin(filename string) (witgo.ValidationReport, error)`,
		`witgo.ValidateComponent(filename, PluginPing())`,
		`func CheckPlugin(filename string) error`,
		`return report.Err()`,
		`"simple:model/host@1.0.0#current-user"`,
		`"simple:model/sso@1.0.0#get"`,
		`"simple:model/sso@1.0.0#save"`,
		`"()->(record{name:string,age:s64})"`,
		`Interface: "simple:model/host@1.0.0"`,
		`Function: "current-user"`,
		"func (c *Plugin) Close() error",
		"func (c *pluginSSOClient) Get() (User, error)",
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	if normalized := strings.Join(strings.Fields(string(source)), " "); !strings.Contains(normalized, "SSO SSO") {
		t.Error("generated Plugin does not expose its SSO interface client")
	}
	for _, unwanted := range []string{
		"type Caller interface",
		"type PluginClient struct",
		"func NewPlugin",
		"func LowerUser",
		"func LiftUser",
		"map[string]any",
		"ReadMemory(",
		"readRecord[",
		"func (c *Plugin) Get()",
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

func TestGenerateGroupsMethodsByExportedInterface(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	contract := `package test:metadata@1.0.0;

interface metadata {
    record info {
        name: string,
        version: string,
        description: string,
        author: string,
        license: string,
    }

    get: func() -> info;
}

interface host {
    process-string: func(value: string) -> string;
}

world plugin {
    import host;
    export metadata;
}`
	if err := os.WriteFile(filepath.Join(witDir, "metadata.wit"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(Config{WIT: witDir, Output: outputDir}); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(outputDir, DefaultFilename)
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.Join(strings.Fields(string(source)), " ")
	for _, expected := range []string{
		"Metadata Metadata",
		"func (c *pluginMetadataClient) Get() (Info, error)",
		`c.runtime.Call("test:metadata/metadata@1.0.0#get")`,
		`Interface: "test:metadata/host@1.0.0"`,
		`Function: "process-string"`,
		`imports.Host.ProcessString(_witgoArg0)`,
	} {
		if !strings.Contains(normalized, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	if strings.Contains(normalized, "func (c *Plugin) Get()") {
		t.Error("interface method was flattened onto Plugin")
	}
	parseGenerated(t, filename, source)
}

func TestGenerateUsesStableImportsStructForLargeWorld(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	contract := `package test:large@1.0.0;

interface cache { get: func(key: string) -> string; }
interface logger { log: func(message: string); }
interface clock { now: func() -> u64; }
interface api { run: func(); }

world plugin {
    import cache;
    import logger;
    import clock;
    export api;
}`
	if err := os.WriteFile(filepath.Join(witDir, "large.wit"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(Config{WIT: witDir, Output: outputDir}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(outputDir, DefaultFilename))
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.Join(strings.Fields(string(source)), " ")
	for _, expected := range []string{
		"type PluginImports struct { Cache Cache Logger Logger Clock Clock }",
		"func OpenPlugin(filename string, imports PluginImports) (*Plugin, error)",
		"func OpenPluginWithOptions(filename string, _witgoOptions witgo.RuntimeOptions, imports PluginImports) (*Plugin, error)",
	} {
		if !strings.Contains(normalized, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	parseGenerated(t, filepath.Join(outputDir, DefaultFilename), source)
}

func parseGenerated(t *testing.T, filename string, source []byte) {
	t.Helper()
	fileSet := token.NewFileSet()
	_, err := goparser.ParseFile(fileSet, filename, source, goparser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated Go: %v", err)
	}
}
