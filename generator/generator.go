// Package generator turns WIT contracts into Go bindings.
package generator

import (
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/slavkiy/witgo/parser/ir"
	"github.com/slavkiy/witgo/parser/parser"
)

const DefaultFilename = "bindings.gen.go"

type Config struct {
	WIT              string
	Output           string
	Package          string
	Filename         string
	EnableRuntimeAPI bool
}

type Generator struct {
	config Config
}

func New(config Config) (*Generator, error) {
	if strings.TrimSpace(config.WIT) == "" {
		return nil, fmt.Errorf("wit path is required")
	}
	if strings.TrimSpace(config.Output) == "" {
		config.Output = "."
	}
	if strings.TrimSpace(config.Filename) == "" {
		config.Filename = DefaultFilename
	}
	if filepath.Base(config.Filename) != config.Filename || filepath.Ext(config.Filename) != ".go" {
		return nil, fmt.Errorf("filename must be a Go filename without directories")
	}
	return &Generator{config: config}, nil
}

func Generate(config Config) error {
	generator, err := New(config)
	if err != nil {
		return err
	}
	return generator.Generate()
}

func (g *Generator) Generate() error {
	files, err := loadWIT(g.config.WIT)
	if err != nil {
		return err
	}

	model, err := lowerFiles(files)
	if err != nil {
		return err
	}

	packageName := strings.TrimSpace(g.config.Package)
	if packageName == "" && model.Package != nil {
		packageName = goPackageName(model.Package.Name)
	}
	if packageName == "" {
		return fmt.Errorf("go package name is required when WIT package is not declared")
	}

	source, err := RenderWithOptions(model, packageName, RenderOptions{EnableRuntimeAPI: g.config.EnableRuntimeAPI})
	if err != nil {
		return fmt.Errorf("render Go bindings: %w", err)
	}
	source, err = format.Source(source)
	if err != nil {
		return fmt.Errorf("format Go bindings: %w\n%s", err, source)
	}

	if err := os.MkdirAll(g.config.Output, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return writeAtomic(filepath.Join(g.config.Output, g.config.Filename), source)
}

type sourceFile struct {
	path    string
	content []byte
}

func loadWIT(root string) ([]sourceFile, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat WIT path: %w", err)
	}

	var paths []string
	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(root), ".wit") {
			paths = append(paths, root)
		}
	} else {
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".wit") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk WIT directory: %w", err)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .wit files found in %q", root)
	}
	sort.Strings(paths)

	files := make([]sourceFile, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		files = append(files, sourceFile{path: path, content: content})
	}
	return files, nil
}

func lowerFiles(files []sourceFile) (*ir.File, error) {
	merged := &ir.File{}
	for _, source := range files {
		parsed, err := parser.Parse(string(source.content))
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", source.path, err)
		}
		file, err := ir.Lower(parsed)
		if err != nil {
			return nil, fmt.Errorf("lower %q: %w", source.path, err)
		}
		if err := mergeFile(merged, file, source.path); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func mergeFile(target, source *ir.File, path string) error {
	if source.Package != nil {
		if target.Package == nil {
			copy := *source.Package
			target.Package = &copy
		} else if *target.Package != *source.Package {
			return fmt.Errorf(
				"WIT package in %q is %s:%s@%s, expected %s:%s@%s",
				path,
				source.Package.Namespace, source.Package.Name, source.Package.Version,
				target.Package.Namespace, target.Package.Name, target.Package.Version,
			)
		}
	}
	target.Uses = append(target.Uses, source.Uses...)
	target.Decls = append(target.Decls, source.Decls...)
	return nil
}

func writeAtomic(filename string, content []byte) (err error) {
	dir := filepath.Dir(filename)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(filename)+".*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if _, err = temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err = temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set output permissions: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err = os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("replace output %q: %w", filename, err)
	}
	return nil
}
