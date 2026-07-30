package path

import (
	"fmt"
	"os"
	"path/filepath"
)

type DirectoryTree struct {
	Name string
	Path string

	Directories []*Directory
	Files       []*SourceFile
}

type Directory struct {
	Name string
	Path string

	DepthFromRoot int

	Directories []*Directory
	Files       []*SourceFile
}

type SourceFile struct {
	Name string
	Path string

	Content []byte
}

func BuildDirectoryTree(dirname string) (*DirectoryTree, error) {
	rootPath, err := filepath.Abs(dirname)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("stat root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", rootPath)
	}

	root := &DirectoryTree{
		Name: info.Name(),
		Path: rootPath,
	}

	directories, files, err := loadDirectory(rootPath, 1)
	if err != nil {
		return nil, err
	}

	root.Directories = directories
	root.Files = files

	return root, nil
}

func loadDirectory(dirname string, depthFromRoot int) ([]*Directory, []*SourceFile, error) {
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return nil, nil, fmt.Errorf("read directory %q: %w", dirname, err)
	}

	directories := make([]*Directory, 0)
	files := make([]*SourceFile, 0)

	for _, entry := range entries {
		entryPath := filepath.Join(dirname, entry.Name())

		if entry.IsDir() {
			nestedDirectories, nestedFiles, err := loadDirectory(entryPath, depthFromRoot+1)
			if err != nil {
				return nil, nil, err
			}

			directories = append(directories, &Directory{
				Name:          entry.Name(),
				Path:          entryPath,
				DepthFromRoot: depthFromRoot,
				Directories:   nestedDirectories,
				Files:         nestedFiles,
			})
			continue
		}

		content, err := os.ReadFile(entryPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read file %q: %w", entryPath, err)
		}

		files = append(files, &SourceFile{
			Name:    entry.Name(),
			Path:    entryPath,
			Content: content,
		})
	}

	return directories, files, nil
}
