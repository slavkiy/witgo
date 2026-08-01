package witgo

type Config struct {
	// WIT is a .wit file or directory. WITMode controls directory traversal.
	WIT string
	// WITFiles is an explicit, ordered-independent set of .wit files. It cannot
	// be combined with WIT and is primarily used by GenerateFiles.
	WITFiles []string
	// WITMode defaults to WITInputAuto for backwards compatibility.
	WITMode WITInputMode
	// GoOverlay is an optional versioned YAML file that changes the public Go
	// representation while preserving the canonical WIT wire types.
	GoOverlay string
	// Output is the directory where generated bindings are written.
	Output string
	// Package is the Go package name. When empty, it is derived from WIT package.
	Package string
	// Filename defaults to bindings.gen.go.
	Filename string
	// EnableRuntimeAPI emits the opt-in witgo:runtime@1.0.0 guest facade and
	// enables its host binding in generated Open functions.
	EnableRuntimeAPI bool
	// Mode selects which side of the component boundary is generated. The zero
	// value is GenerateHost and preserves the historical host SDK.
	Mode GenerationMode
	// World selects the WIT world in guest mode. It may be omitted when the
	// input contains exactly one world.
	World string
	// PackageRoot is the Go import path of Output. Guest bindings use it for
	// the packages generated below Output. When empty it is derived from the
	// nearest go.mod.
	PackageRoot string
	// GuestBindgen is an optional path to wit-bindgen-go. The default searches
	// PATH. It is used only in guest mode.
	GuestBindgen string
}

// GenerationMode selects the API surface emitted from a WIT world.
type GenerationMode uint8

const (
	// GenerateHost emits loaders, validation and composition clients.
	GenerateHost GenerationMode = iota
	// GenerateGuest emits Go/TinyGo Component Model guest bindings. Host-only
	// constructors such as OpenPlugin are deliberately absent.
	GenerateGuest
)

// WITInputMode controls how a WIT input path is expanded.
type WITInputMode uint8

const (
	// WITInputAuto accepts a file or recursively scans a directory. This keeps
	// the behaviour of Config.WIT from earlier releases.
	WITInputAuto WITInputMode = iota
	// WITInputFile requires exactly one .wit file.
	WITInputFile
	// WITInputPackage loads .wit files directly inside one package directory.
	// Nested directories such as deps are package boundaries and are ignored.
	WITInputPackage
	// WITInputTree recursively loads every .wit file and requires them all to
	// declare the same WIT package.
	WITInputTree
)
