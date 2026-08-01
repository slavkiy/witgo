package generator

import (
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
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

func TestGenerateRejectsGoNameCollision(t *testing.T) {
	dir := t.TempDir()
	contract := `package test:collision;
record foo-bar { value: string }
record foo--bar { value: string }
`
	if err := os.WriteFile(filepath.Join(dir, "collision.wit"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Generate(Config{WIT: dir, Output: filepath.Join(dir, "out"), Package: "contract"})
	if err == nil || !strings.Contains(err.Error(), "Go name collision") || !strings.Contains(err.Error(), "FooBar") {
		t.Fatalf("Generate error = %v, want clear Go name collision", err)
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
interface api {
    enum color { red, green }
    flags permissions { read, write }
    variant choice { none, number(u32) }
    run: func(value: choice) -> list<list<u32>>;
}

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
		"type Color string",
		`ColorGreen Color = "green"`,
		"func (value Permissions) MarshalJSON() ([]byte, error)",
		"func (value Choice) MarshalJSON() ([]byte, error)",
		"func (value *Choice) UnmarshalJSON(data []byte) error",
	} {
		if !strings.Contains(normalized, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	parseGenerated(t, filepath.Join(outputDir, DefaultFilename), source)
}

func TestGenerateComponentHandles(t *testing.T) {
	dir := t.TempDir()
	wit := `package test:handles@1.0.0;

interface api {
    resource file;
    type completion = future<string>;
    type chunks = stream<list<u8>>;
    type failure = error-context;
    open: func() -> file;
    wait: func(value: completion) -> failure;
    consume: func(value: chunks);
}

world plugin { export api; }
`
	if err := os.WriteFile(filepath.Join(dir, "handles.wit"), []byte(wit), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")
	if err := Generate(Config{WIT: dir, Output: output, Package: "contract"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, DefaultFilename))
	if err != nil {
		t.Fatal(err)
	}
	generated := strings.Join(strings.Fields(string(data)), " ")
	for _, expected := range []string{
		"type File = witgo.Handle",
		"type Completion = witgo.Handle",
		"type Chunks = witgo.Handle",
		"type Failure = witgo.Handle",
		"Open() (File, error)",
		`"test:handles/api@1.0.0#wait": "(future<string>)->(error-context)"`,
		`"test:handles/api@1.0.0#consume": "(stream<list<u8>>)->()"`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}

func TestGenerateValueTypeHelpers(t *testing.T) {
	dir := t.TempDir()
	wit := `package test:values@1.0.0;

interface api {
    type maybe-name = option<string>;
    type pair = tuple<char, u32>;
    type outcome = result<pair, string>;
    type simple-result = result<string>;
    type empty-result = result<>;
    type failure-only = result<_, string>;
    type dictionary = map<string, maybe-name>;
    type large-tuple = tuple<u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8, u8>;
    enum color { red, green }
    flags permissions { read, write }
    variant choice { none, text(string), pair(pair) }
    transform: func(value: maybe-name, pair: pair) -> outcome;
    collide: func(err: string, output: string) -> string;
}

world plugin { export api; }
`
	if err := os.WriteFile(filepath.Join(dir, "values.wit"), []byte(wit), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")
	if err := Generate(Config{WIT: dir, Output: output, Package: "contract"}); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(output, DefaultFilename)
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	generated := strings.Join(strings.Fields(string(data)), " ")
	for _, expected := range []string{
		"type MaybeName = witgo.Option[string]",
		"type Pair = witgo.Tuple2[witgo.Char, uint32]",
		"type Outcome = witgo.Result[Pair, string]",
		"type SimpleResult = witgo.Result[string, witgo.Unit]",
		"type EmptyResult = witgo.Result[witgo.Unit, witgo.Unit]",
		"type FailureOnly = witgo.Result[witgo.Unit, string]",
		"type Dictionary = witgo.Map[string, MaybeName]",
		"type LargeTuple = witgo.Tuple",
		"func ParseColor(value string) (Color, error)",
		"func ColorValues() []Color",
		"func (value Permissions) Has(flags Permissions) bool",
		"func ParsePermissions(names ...string) (Permissions, error)",
		"func NewChoiceNone() Choice",
		"func NewChoiceText(payload string) Choice",
		"func (value Choice) GetText() (string, bool)",
		"func (value Choice) IsPair() bool",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	parseGenerated(t, filename, data)

	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	module := "module generated.test/contract\n\ngo 1.18\n\nrequire github.com/slavkiy/witgo v0.0.0\nreplace github.com/slavkiy/witgo => " + filepath.ToSlash(repository) + "\n"
	if err := os.WriteFile(filepath.Join(output, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	helperTest := `package contract

import (
    "encoding/json"
    "testing"
)

func TestGeneratedHelpers(t *testing.T) {
    color, err := ParseColor("green")
    if err != nil || !color.Valid() || color.String() != "green" { t.Fatalf("color = %q, %v", color, err) }
    permissions, err := ParsePermissions("read", "write")
    if err != nil || !permissions.Has(PermissionsRead) { t.Fatalf("permissions = %v, %v", permissions, err) }
    permissions.Remove(PermissionsRead)
    if permissions.Has(PermissionsRead) { t.Fatal("removed flag is still set") }
    empty, err := json.Marshal(Permissions(0))
    if err != nil || string(empty) != "[]" { t.Fatalf("empty flags = %s, %v", empty, err) }
    choice := NewChoiceText("hello")
    if payload, ok := choice.GetText(); !ok || payload != "hello" || !choice.IsText() { t.Fatalf("choice = %#v", choice) }
}
`
	if err := os.WriteFile(filepath.Join(output, "bindings_test.go"), []byte(helperTest), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = output
	if result, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("resolve generated package: %v\n%s", err, result)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = output
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile generated package: %v\n%s", err, result)
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
