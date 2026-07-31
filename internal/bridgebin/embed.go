// Package bridgebin contains platform-specific Wasmtime bridge executables.
package bridgebin

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

var ErrUnavailable = errors.New("embedded component bridge is unavailable for this platform")

// Executable materializes the embedded binary in the user's cache directory.
// The filename includes a content hash, so upgrades and concurrent processes
// do not overwrite a running executable.
func Executable() (string, error) {
	compressed := compressedBridge
	if len(compressed) == 0 {
		return "", fmt.Errorf("%w: %s/%s", ErrUnavailable, runtime.GOOS, runtime.GOARCH)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", fmt.Errorf("open embedded bridge: %w", err)
	}
	data, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return "", fmt.Errorf("decompress embedded bridge: %w", err)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close embedded bridge: %w", closeErr)
	}
	sum := sha256.Sum256(data)
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve bridge cache: %w", err)
	}
	directory := filepath.Join(cache, "witgo", "bridge")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create bridge cache: %w", err)
	}
	filename := fmt.Sprintf("witgo-component-host-%x", sum[:8])
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	path := filepath.Join(directory, filename)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	temporary, err := os.CreateTemp(directory, "bridge-*")
	if err != nil {
		return "", fmt.Errorf("create embedded bridge: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Chmod(0o755)
	}
	if close := temporary.Close(); err == nil {
		err = close
	}
	if err != nil {
		return "", fmt.Errorf("write embedded bridge: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
		return "", fmt.Errorf("install embedded bridge: %w", err)
	}
	return path, nil
}
