package witgo

type Config struct {
	// WIT is a .wit file or a directory scanned recursively for .wit files.
	WIT string
	// Output is the directory where generated bindings are written.
	Output string
	// Package is the Go package name. When empty, it is derived from WIT package.
	Package string
	// Filename defaults to bindings.gen.go.
	Filename string
}
