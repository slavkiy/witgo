package witgo

import (
	"fmt"

	ipath "github.com/slavkiy/witgo/internal/path"
)

func Open(config *Config) (*WitgoCtx, error) {
	pkg, err := LoadPackage(config)
	if err != nil {
		return nil, err
	}

	_ = pkg

	return nil, nil
}

type Package struct {
	DirectoryTree *ipath.DirectoryTree
	Config        *Config
}

func LoadPackage(config *Config) (*Package, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	directoryTree, err := ipath.BuildDirectoryTree(config.WIT)
	if err != nil {
		return nil, fmt.Errorf("build package tree: %w", err)
	}

	return &Package{
		DirectoryTree: directoryTree,
		Config:        config,
	}, nil
}
