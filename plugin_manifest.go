package witgo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	iwasm "github.com/slavkiy/witgo/internal/wasm"
)

const pluginManifestSection = "witgo:plugin-manifest"

// PluginManifest is metadata owned by a plugin. Dependencies map a complete
// WIT import or an interface name to a component path relative to the plugin.
// Resource limits intentionally cannot be declared here; only the host owns
// the box budget.
type PluginManifest struct {
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// EmbedPluginManifest appends a witgo custom section to a binary Component
// Model component. It is intended for plugin build pipelines.
func EmbedPluginManifest(component []byte, manifest PluginManifest) ([]byte, error) {
	if iwasm.DetectKind(component) != iwasm.KindComponent {
		return nil, errors.New("plugin manifest can only be embedded into a binary WebAssembly component")
	}
	if _, found, err := decodePluginManifestSection(component); err != nil {
		return nil, err
	} else if found {
		return nil, errors.New("component already contains a witgo plugin manifest")
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode plugin manifest: %w", err)
	}
	payload := appendULEB(nil, uint64(len(pluginManifestSection)))
	payload = append(payload, pluginManifestSection...)
	payload = append(payload, data...)
	result := append([]byte(nil), component...)
	result = append(result, 0)
	result = appendULEB(result, uint64(len(payload)))
	result = append(result, payload...)
	return result, nil
}

// ReadPluginManifest reads an embedded manifest or the sidecar
// <component>.witgo.json used for textual components and development builds.
func ReadPluginManifest(filename string) (PluginManifest, bool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return PluginManifest{}, false, err
	}
	if iwasm.IsWasm(data) {
		manifest, found, err := decodePluginManifestSection(data)
		if err != nil || found {
			return manifest, found, err
		}
	}
	sidecar, err := os.ReadFile(filename + ".witgo.json")
	if errors.Is(err, os.ErrNotExist) {
		return PluginManifest{}, false, nil
	}
	if err != nil {
		return PluginManifest{}, false, fmt.Errorf("read plugin manifest sidecar: %w", err)
	}
	var manifest PluginManifest
	if err := json.Unmarshal(sidecar, &manifest); err != nil {
		return PluginManifest{}, false, fmt.Errorf("decode plugin manifest sidecar: %w", err)
	}
	return manifest, true, nil
}

func decodePluginManifestSection(component []byte) (PluginManifest, bool, error) {
	if len(component) < 8 {
		return PluginManifest{}, false, nil
	}
	for offset := 8; offset < len(component); {
		sectionID := component[offset]
		offset++
		size, next, ok := readULEB(component, offset)
		if !ok || size > uint64(len(component)-next) {
			return PluginManifest{}, false, errors.New("malformed WebAssembly section while reading plugin manifest")
		}
		end := next + int(size)
		if sectionID == 0 {
			nameLength, content, ok := readULEB(component, next)
			if !ok || nameLength > uint64(end-content) {
				return PluginManifest{}, false, errors.New("malformed WebAssembly custom section")
			}
			nameEnd := content + int(nameLength)
			if bytes.Equal(component[content:nameEnd], []byte(pluginManifestSection)) {
				var manifest PluginManifest
				if err := json.Unmarshal(component[nameEnd:end], &manifest); err != nil {
					return PluginManifest{}, false, fmt.Errorf("decode embedded plugin manifest: %w", err)
				}
				return manifest, true, nil
			}
		}
		offset = end
	}
	return PluginManifest{}, false, nil
}

func appendULEB(out []byte, value uint64) []byte {
	for {
		current := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			current |= 0x80
		}
		out = append(out, current)
		if value == 0 {
			return out
		}
	}
}

func readULEB(data []byte, offset int) (uint64, int, bool) {
	var value uint64
	for shift := uint(0); shift < 64 && offset < len(data); shift += 7 {
		current := data[offset]
		offset++
		value |= uint64(current&0x7f) << shift
		if current&0x80 == 0 {
			return value, offset, true
		}
	}
	return 0, offset, false
}
