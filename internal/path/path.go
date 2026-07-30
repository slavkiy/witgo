package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func NormalizePath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not find home folder: %w", err)
		}

		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not process path: %w", err)
	}

	return filepath.Clean(absolutePath), nil
}
