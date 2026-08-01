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
}

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
