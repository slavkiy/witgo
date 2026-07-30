package witgo

import (
	"fmt"

	ipath "github.com/slavkiy/witgo/internal/path"
)

func Open(config *Config) (*WitgoCtx, error) {
	pkg, err := loadPackage(config)
	if err != nil {
		return nil, err
	}

	_ = pkg

	return nil, nil
}

type dir_package struct {
	DirectoryTree *ipath.DirectoryTree
	Config        *Config
}

func loadPackage(config *Config) (*dir_package, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	directoryTree, err := ipath.BuildDirectoryTree(config.WIT)
	if err != nil {
		return nil, fmt.Errorf("build package tree: %w", err)
	}

	return &dir_package{
		DirectoryTree: directoryTree,
		Config:        config,
	}, nil
}
