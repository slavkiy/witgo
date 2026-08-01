package witgo

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

// CompositionPlug describes one exact Component Model import edge. Component
// provides Interface; Dependencies satisfy imports of that provider itself.
// Paths are normalized by the runtime before they reach the native bridge.
type CompositionPlug struct {
	Interface    string            `json:"interface"`
	Component    string            `json:"component"`
	Dependencies []CompositionPlug `json:"dependencies,omitempty"`
}

// ComponentComposition is a complete same-Store composition rooted at a
// consumer component.
type ComponentComposition struct {
	Component    string            `json:"component"`
	Dependencies []CompositionPlug `json:"dependencies,omitempty"`
}

func normalizeComposition(root string, plugs []CompositionPlug) ([]CompositionPlug, error) {
	rootPath, err := componentPath(root)
	if err != nil {
		return nil, err
	}
	active := map[string]bool{rootPath: true}
	var normalize func(string, []CompositionPlug) ([]CompositionPlug, error)
	normalize = func(parent string, values []CompositionPlug) ([]CompositionPlug, error) {
		seenInterfaces := make(map[string]bool, len(values))
		result := make([]CompositionPlug, 0, len(values))
		for _, value := range values {
			if value.Interface == "" || value.Component == "" {
				return nil, errors.New("composition interface and component path are required")
			}
			if seenInterfaces[value.Interface] {
				return nil, fmt.Errorf("%w: multiple providers selected for %s", ErrPluginAlreadyRegistered, value.Interface)
			}
			seenInterfaces[value.Interface] = true
			path, err := componentPath(resolveNestedPath(filepath.Dir(parent), value.Component))
			if err != nil {
				return nil, fmt.Errorf("composition provider for %s: %w", value.Interface, err)
			}
			if active[path] {
				return nil, fmt.Errorf("%w: interface=%s component=%s", ErrNestedPluginCycle, value.Interface, path)
			}
			active[path] = true
			dependencies, err := normalize(path, value.Dependencies)
			delete(active, path)
			if err != nil {
				return nil, err
			}
			result = append(result, CompositionPlug{Interface: value.Interface, Component: path, Dependencies: dependencies})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Interface < result[j].Interface })
		return result, nil
	}
	return normalize(root, plugs)
}

func compositionContract(contract Contract, plugs []CompositionPlug) Contract {
	if len(plugs) == 0 {
		return contract
	}
	provided := make(map[string]bool, len(plugs))
	for _, plug := range plugs {
		provided[plug.Interface] = true
	}
	result := Contract{Exports: append([]string(nil), contract.Exports...)}
	for _, name := range contract.Imports {
		interfaceName := name
		if value, _, ok := stringsCutFunction(name); ok {
			interfaceName = value
		}
		if !provided[interfaceName] {
			result.Imports = append(result.Imports, name)
		}
	}
	allowed := make(map[string]bool, len(result.Imports)+len(result.Exports))
	for _, name := range append(append([]string(nil), result.Imports...), result.Exports...) {
		allowed[name] = true
	}
	for name, signature := range contract.Signatures {
		if allowed[name] {
			if result.Signatures == nil {
				result.Signatures = make(map[string]string)
			}
			result.Signatures[name] = signature
		}
	}
	return result
}

func stringsCutFunction(name string) (string, string, bool) {
	for index := len(name) - 1; index >= 0; index-- {
		if name[index] == '#' {
			return name[:index], name[index+1:], true
		}
	}
	return name, "", false
}
