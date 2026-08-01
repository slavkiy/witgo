package witgo

import (
	"fmt"

	"github.com/slavkiy/witgo/generator"
)

// Generator creates Go bindings from WIT contracts.
type Generator struct {
	inner *generator.Generator
}

// NewGenerator validates the configuration and creates a reusable generator.
func NewGenerator(config Config) (*Generator, error) {
	inner, err := generator.New(generatorConfig(config))
	if err != nil {
		return nil, err
	}
	return &Generator{inner: inner}, nil
}

// Generate creates Go bindings in config.Output.
func Generate(config Config) error {
	instance, err := NewGenerator(config)
	if err != nil {
		return err
	}
	return instance.Generate()
}

// GenerateFile creates bindings from exactly one .wit file.
func GenerateFile(config Config, filename string) error {
	config.WIT = filename
	config.WITFiles = nil
	config.WITMode = WITInputFile
	return Generate(config)
}

// GenerateFiles creates one binding package from an explicit set of .wit
// files. Every file must belong to the same WIT package.
func GenerateFiles(config Config, filenames ...string) error {
	config.WIT = ""
	config.WITFiles = append([]string(nil), filenames...)
	config.WITMode = WITInputFile
	return Generate(config)
}

// GeneratePackage creates bindings from all .wit files directly inside a
// package directory. Nested package and deps directories are not traversed.
func GeneratePackage(config Config, directory string) error {
	config.WIT = directory
	config.WITFiles = nil
	config.WITMode = WITInputPackage
	return Generate(config)
}

// GenerateTree recursively loads every .wit file below directory. It is useful
// for split source trees that contain one WIT package and no dependency packages.
func GenerateTree(config Config, directory string) error {
	config.WIT = directory
	config.WITFiles = nil
	config.WITMode = WITInputTree
	return Generate(config)
}

// Generate creates or replaces the configured generated Go file.
func (g *Generator) Generate() error {
	if g == nil || g.inner == nil {
		return fmt.Errorf("generator is nil")
	}
	return g.inner.Generate()
}

func generatorConfig(config Config) generator.Config {
	return generator.Config{
		WIT:              config.WIT,
		WITFiles:         append([]string(nil), config.WITFiles...),
		WITMode:          generator.InputMode(config.WITMode),
		GoOverlay:        config.GoOverlay,
		Output:           config.Output,
		Package:          config.Package,
		Filename:         config.Filename,
		EnableRuntimeAPI: config.EnableRuntimeAPI,
	}
}
