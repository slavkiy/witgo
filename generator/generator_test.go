package generator

import (
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
		"func (value User) Save(ctx context.Context) (bool, error)",
		`WITPackageID        = "simple:model@1.0.0"`,
		`runtime.CallContext(ctx, "simple:model/sso@1.0.0#get")`,
		`witgo.LoadRuntimeWithContract`,
		`type PluginImports struct`,
		`func PluginPing() witgo.Contract`,
		`func ValidatePlugin(filename string) (witgo.ValidationReport, error)`,
		`witgo.ValidateComponentContext(ctx, filename, PluginPing())`,
		`func CheckPlugin(filename string) error`,
		`return report.Err()`,
		`"simple:model/host@1.0.0#current-user"`,
		`"simple:model/sso@1.0.0#get"`,
		`"simple:model/sso@1.0.0#save"`,
		`"()->(record{name:string,age:s64})"`,
		`Interface: "simple:model/host@1.0.0"`,
		`Function: "current-user"`,
		`if imports.Host != nil`,
		`_witgoImports = append(_witgoImports, witgo.HostImport{`,
		"func (c *Plugin) Close() error",
		"func (c *Plugin) Restart() error",
		"reopen func(context.Context) (runtimeCaller, error)",
		"client.SSO = client.ssoClient",
		"func (c *pluginSSOClient) Get(ctx context.Context) (User, error)",
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
		"plugin import host is nil",
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

func TestGeneratePackageIgnoresNestedDependencies(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(witDir, "types.wit"), []byte("package app:plugin; record item { value: string }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(witDir, "deps", "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(witDir, "deps", "vendor", "types.wit"), []byte("package vendor:types;"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(Config{WIT: witDir, WITMode: InputPackage, Output: outputDir}); err != nil {
		t.Fatalf("Generate package returned error: %v", err)
	}
}

func TestGenerateExplicitFiles(t *testing.T) {
	witDir := t.TempDir()
	outputDir := t.TempDir()
	first := filepath.Join(witDir, "types.wit")
	second := filepath.Join(witDir, "world.wit")
	if err := os.WriteFile(first, []byte("package app:plugin; record item { value: string }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package app:plugin; world plugin {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(Config{WITFiles: []string{second, first}, Output: outputDir}); err != nil {
		t.Fatalf("Generate explicit files returned error: %v", err)
	}
	source, err := os.ReadFile(filepath.Join(outputDir, DefaultFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "type Item struct") || !strings.Contains(string(source), "type Plugin struct") {
		t.Fatal("generated bindings do not contain declarations from both files")
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
		"func (c *pluginMetadataClient) Get(ctx context.Context) (Info, error)",
		`c.runtime.CallContext(ctx, "test:metadata/metadata@1.0.0#get")`,
		`Interface: "test:metadata/host@1.0.0"`,
		`Function: "process-string"`,
		`imports.Host.ProcessString(ctx, _witgoArg0)`,
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
		"func OpenPlugin(filename string, imports ...PluginImports) (*Plugin, error)",
		"func OpenPluginWithOptions(filename string, _witgoOptions witgo.RuntimeOptions, imports ...PluginImports) (*Plugin, error)",
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

func TestGenerateTransparentPluginCompositionAPI(t *testing.T) {
	dir := t.TempDir()
	wit := `package example:pipeline@1.0.0;

interface image-codec {
  decode: func(data: list<u8>) -> result<string, string>;
}

world codec-plugin {
  export image-codec;
}

world processor-plugin {
  import image-codec;
}
`
	if err := os.WriteFile(filepath.Join(dir, "pipeline.wit"), []byte(wit), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")
	if err := Generate(Config{WIT: dir, Output: output, Package: "contract"}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(output, DefaultFilename))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"type ImageCodec interface {",
		"Decode(ctx context.Context, data []uint8) (witgo.Result[string, string], error)",
		"var ImageCodecDescriptor = witgo.InterfaceDescriptor{",
		`ID: "example:pipeline/image-codec@1.0.0"`,
		"func RegisterImageCodec(host *witgo.Host, name string, provider ImageCodec, options ...witgo.RegisterOption) error",
		"func ResolveImageCodec(host *witgo.Host, name string) (ImageCodec, error)",
		"func AutoResolveImageCodec(host *witgo.Host) (ImageCodec, error)",
		"var _ ImageCodec = (*ImageCodecProviderClient)(nil)",
		"var _ ImageCodec = (*codecPluginImageCodecClient)(nil)",
		"type ProcessorPluginBindings = ProcessorPluginImports",
		"func AutoBindProcessorPlugin(host *witgo.Host) (ProcessorPluginBindings, error)",
		"func OpenProcessorPluginWithHost(host *witgo.Host",
		"func (c *ImageCodecProviderClient) witgoCompositionPlug() (witgo.CompositionPlug, bool)",
		"options = append(options, witgo.ComponentProvider(composition))",
		"_witgoOptions.CompositionPlugs = append(_witgoOptions.CompositionPlugs, _witgoPlug)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated source does not contain %q", want)
		}
	}
	if count := strings.Count(text, "type ImageCodec interface {"); count != 1 {
		t.Fatalf("ImageCodec interface generated %d times", count)
	}
	parseGenerated(t, filepath.Join(output, DefaultFilename), source)
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
		"Open(ctx context.Context) (File, error)",
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

func TestRuntimeSystemFacadeIsExplicitOptIn(t *testing.T) {
	contract := `package test:runtime@1.0.0;
interface api { run: func(); }
world plugin { export api; }`
	generate := func(enabled bool) string {
		t.Helper()
		witDir, outputDir := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(witDir, "runtime.wit"), []byte(contract), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Generate(Config{WIT: witDir, Output: outputDir, EnableRuntimeAPI: enabled}); err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join(outputDir, DefaultFilename))
		if err != nil {
			t.Fatal(err)
		}
		return string(source)
	}
	plain := generate(false)
	if strings.Contains(plain, "type RuntimeSystem interface") || strings.Contains(plain, "UnsafeRequestAdditionalFuel") {
		t.Fatal("runtime system facade appeared without opt-in")
	}
	enabled := generate(true)
	for _, expected := range []string{
		"type RuntimeSystem interface",
		"CallInfo() (witgo.RuntimeCallInfo, error)",
		"UnsafeRequestAdditionalFuel(amount uint64, reason string) (witgo.FuelGrant, error)",
		"type System struct",
		"_witgoOptions.EnableRuntimeAPI = true",
		"vendor capability witgo:runtime/runtime@1.0.0",
	} {
		if !strings.Contains(enabled, expected) {
			t.Errorf("opt-in source does not contain %q", expected)
		}
	}
}

func TestGenerateGoOverlayUsesPublicAndWireTypes(t *testing.T) {
	dir, output := t.TempDir(), t.TempDir()
	wit := `package example:users@1.0.0;
interface users {
  type timestamp = s64;
  record user { id: string, created-at: timestamp }
  save: func(value: user) -> result<_, string>;
}
world plugin { export users; }`
	overlay := `version: 1
types:
  example:users/users@1.0.0#timestamp:
    go_type: time.Time
    import: time
    codec: unix-seconds
errors:
  example:users/users@1.0.0#save:
    result_error: true
`
	witPath := filepath.Join(dir, "plugin.wit")
	overlayPath := filepath.Join(dir, "plugin.witgo.yaml")
	if err := os.WriteFile(witPath, []byte(wit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlayPath, []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(Config{WIT: witPath, GoOverlay: overlayPath, Output: output}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(output, DefaultFilename))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"type Timestamp = time.Time",
		"type userWire struct",
		"CreatedAt int64",
		"func lowerUser(value User) userWire",
		"wire.CreatedAt = lowerTimestamp(value.CreatedAt)",
		"func (c *pluginUsersClient) Save(ctx context.Context, value User) error",
		"return witgo.NewWITError(value)",
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("generated overlay source does not contain %q", expected)
		}
	}
	parseGenerated(t, filepath.Join(output, DefaultFilename), source)
	compileGeneratedOverlay(t, output)
}

func TestGenerateGoOverlayValidation(t *testing.T) {
	dir := t.TempDir()
	witPath := filepath.Join(dir, "plugin.wit")
	if err := os.WriteFile(witPath, []byte(`package example:users@1.0.0; interface users { type timestamp = s64; test: func(values: list<timestamp>); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, overlay, want string }{
		{"version", "version: 2\n", "unsupported schema version"},
		{"unknown codec", "version: 1\ntypes:\n  example:users/users@1.0.0#timestamp:\n    go_type: time.Time\n    import: time\n    codec: magic\n", `unknown codec "magic"`},
		{"nested list", "version: 1\ntypes:\n  example:users/users@1.0.0#timestamp:\n    go_type: time.Time\n    import: time\n    codec: unix-seconds\n", "mapping inside list is not supported yet"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overlayPath := filepath.Join(dir, strings.ReplaceAll(test.name, " ", "-")+".yaml")
			if err := os.WriteFile(overlayPath, []byte(test.overlay), 0o644); err != nil {
				t.Fatal(err)
			}
			err := Generate(Config{WIT: witPath, GoOverlay: overlayPath, Output: t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), strconv.Quote(overlayPath)) {
				t.Fatalf("Generate error = %v, want path and %q", err, test.want)
			}
		})
	}
}

func compileGeneratedOverlay(t *testing.T, output string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	module := "module overlaytest\n\ngo 1.18\n\nrequire github.com/slavkiy/witgo v0.0.0\nreplace github.com/slavkiy/witgo => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(output, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = output
	if result, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("resolve overlay package: %v\n%s", err, result)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = output
	if result, err := command.CombinedOutput(); err != nil {
		source, _ := os.ReadFile(filepath.Join(output, DefaultFilename))
		t.Fatalf("compile overlay package: %v\n%s\n%s", err, result, source)
	}
}
