// Package bridgebin contains platform-specific Wasmtime shared libraries.
package bridgebin

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

var ErrUnavailable = errors.New("embedded component bridge is unavailable for this platform")
var ErrIntegrity = errors.New("embedded component bridge failed integrity verification")

// Library materializes the embedded shared library in the user's cache directory.
// The filename includes a content hash, so upgrades and concurrent processes
// do not overwrite a loaded library.
func Library() (string, error) {
	compressed := compressedBridge
	if len(compressed) == 0 {
		return "", fmt.Errorf("%w: %s/%s", ErrUnavailable, runtime.GOOS, runtime.GOARCH)
	}
	fields := bytes.Fields([]byte(expectedBridgeSHA256))
	if len(fields) == 0 {
		return "", fmt.Errorf("%w: missing library checksum", ErrIntegrity)
	}
	return Install(compressed, string(fields[0]))
}

// Install verifies and atomically materializes a packaged bridge. It is also
// used for versioned release assets on platforms not embedded in module source.
func Install(compressed []byte, expectedSHA256 string) (string, error) {
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
	if len(data) == 0 {
		return "", fmt.Errorf("%w: %s/%s library placeholder", ErrUnavailable, runtime.GOOS, runtime.GOARCH)
	}
	sum := sha256.Sum256(data)
	expected, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(expected) != sha256.Size {
		return "", fmt.Errorf("%w: invalid compiled-in SHA-256", ErrIntegrity)
	}
	if !equalDigest(sum[:], expected) {
		return "", fmt.Errorf("%w: packaged binary has %x, expected %s", ErrIntegrity, sum, expectedSHA256)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve bridge cache: %w", err)
	}
	directory := filepath.Join(cache, "witgo", "bridge")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create bridge cache: %w", err)
	}
	filename := fmt.Sprintf("witgo-bridge-%x", sum[:8])
	if runtime.GOOS == "windows" {
		filename += ".dll"
	} else if runtime.GOOS == "darwin" {
		filename += ".dylib"
	} else {
		filename += ".so"
	}
	path := filepath.Join(directory, filename)
	if _, err := os.Stat(path); err == nil {
		if err := verifyFile(path, expected); err == nil {
			return path, nil
		}
		if err := os.Remove(path); err != nil {
			return "", fmt.Errorf("replace invalid cached bridge: %w", err)
		}
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
		if verifyFile(path, expected) == nil {
			return path, nil
		}
		return "", fmt.Errorf("install embedded bridge: %w", err)
	}
	if err := verifyFile(path, expected); err != nil {
		return "", err
	}
	return path, nil
}

// AssetName returns the release asset name for the current Go platform.
func AssetName() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64", "linux/arm64":
		return runtime.GOOS + "_" + runtime.GOARCH + ".so.gz", nil
	case "darwin/amd64", "darwin/arm64":
		return runtime.GOOS + "_" + runtime.GOARCH + ".dylib.gz", nil
	case "windows/amd64", "windows/arm64":
		return runtime.GOOS + "_" + runtime.GOARCH + ".dll.gz", nil
	default:
		return "", fmt.Errorf("%w: %s/%s", ErrUnavailable, runtime.GOOS, runtime.GOARCH)
	}
}

func verifyFile(path string, expected []byte) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !equalDigest(hash.Sum(nil), expected) {
		return fmt.Errorf("%w: cached binary has %x, expected %x", ErrIntegrity, hash.Sum(nil), expected)
	}
	return nil
}

func equalDigest(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for i := range left {
		different |= left[i] ^ right[i]
	}
	return different == 0
}
