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

// Generate creates or replaces the configured generated Go file.
func (g *Generator) Generate() error {
	if g == nil || g.inner == nil {
		return fmt.Errorf("generator is nil")
	}
	return g.inner.Generate()
}

func generatorConfig(config Config) generator.Config {
	return generator.Config{
		WIT:      config.WIT,
		Output:   config.Output,
		Package:  config.Package,
		Filename: config.Filename,
	}
}
