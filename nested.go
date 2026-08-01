package witgo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	defaultNestedPluginMaxCandidates = 256
)

// NestedPluginOptions controls automatic plugin-to-plugin dependency wiring.
// A component requests dependencies through ordinary WIT imports. When a
// manual HostImport is absent, witgo searches for a component exporting the
// exact same function name and structural signature.
type NestedPluginOptions struct {
	Disabled bool
	// SearchPaths is only a fallback for components without PluginManifest.
	SearchPaths []string
	// AllowedRoots restricts relative dependency paths declared by plugins.
	// The zero value allows only the parent component directory tree.
	AllowedRoots  []string
	MaxCandidates int
	Resolver      NestedPluginResolver
}

// PluginHost owns immutable discovery settings. Each load performed through a
// host creates a separate box, so plugin instances and their state are never
// shared between top-level loads.
type PluginHost struct {
	options  NestedPluginOptions
	mu       sync.Mutex
	runtimes map[*Runtime]struct{}
	closed   bool
}

// NewPluginHost creates a host-router for automatic nested dependencies.
func NewPluginHost(options NestedPluginOptions) (*PluginHost, error) {
	if err := RequireHostBuild(); err != nil {
		return nil, err
	}
	if options.MaxCandidates < 0 {
		return nil, errors.New("NestedPluginOptions.MaxCandidates cannot be negative")
	}
	return newPluginHost(options), nil
}

func newPluginHost(options NestedPluginOptions) *PluginHost {
	options.SearchPaths = append([]string(nil), options.SearchPaths...)
	options.AllowedRoots = append([]string(nil), options.AllowedRoots...)
	// Host-owned roots are fixed when the router is created. Otherwise a
	// descendant could reinterpret a relative root from its own directory and
	// gradually escape the original box policy.
	for index, root := range options.SearchPaths {
		if absolute, err := filepath.Abs(root); err == nil {
			options.SearchPaths[index] = filepath.Clean(absolute)
		}
	}
	for index, root := range options.AllowedRoots {
		if absolute, err := filepath.Abs(root); err == nil {
			options.AllowedRoots[index] = filepath.Clean(absolute)
		}
	}
	return &PluginHost{options: options, runtimes: make(map[*Runtime]struct{})}
}

// PluginBox is an isolated root runtime and all automatically loaded children.
type PluginBox struct{ runtime *Runtime }

// Runtime returns the root component runtime.
func (b *PluginBox) Runtime() *Runtime {
	if b == nil {
		return nil
	}
	return b.runtime
}

// Close closes the root and every nested runtime in the box.
func (b *PluginBox) Close() error {
	if b == nil {
		return nil
	}
	return b.runtime.Close()
}

// PluginPaths returns direct nested providers selected for the root runtime.
func (b *PluginBox) PluginPaths() []string {
	if b == nil {
		return nil
	}
	return b.runtime.AllPluginPaths()
}

// Close closes every independent box created through this host. It is safe to
// call more than once; a closed host rejects new boxes.
func (h *PluginHost) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	runtimes := make([]*Runtime, 0, len(h.runtimes))
	for runtime := range h.runtimes {
		runtimes = append(runtimes, runtime)
	}
	h.runtimes = make(map[*Runtime]struct{})
	h.mu.Unlock()
	var first error
	for _, runtime := range runtimes {
		if err := runtime.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// BoxCount returns the number of live independent root boxes.
func (h *PluginHost) BoxCount() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.runtimes)
}

func (h *PluginHost) register(runtime *Runtime) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("plugin host is closed")
	}
	h.runtimes[runtime] = struct{}{}
	return nil
}

func (h *PluginHost) unregister(runtime *Runtime) {
	h.mu.Lock()
	delete(h.runtimes, runtime)
	h.mu.Unlock()
}

// OpenBoxContext creates a completely independent component box using this
// host's discovery policy.
func (h *PluginHost) OpenBoxContext(ctx context.Context, filename string, options RuntimeOptions, imports []HostImport) (*PluginBox, error) {
	if h == nil {
		return nil, errors.New("plugin host is nil")
	}
	options.PluginHost = h
	runtime, err := LoadRuntimeWithImportsContext(ctx, filename, options, imports)
	if err != nil {
		return nil, err
	}
	return &PluginBox{runtime: runtime}, nil
}

// OpenBox is OpenBoxContext with context.Background().
func (h *PluginHost) OpenBox(filename string, options RuntimeOptions, imports []HostImport) (*PluginBox, error) {
	return h.OpenBoxContext(context.Background(), filename, options, imports)
}

// NestedPluginRequest describes one unresolved WIT import.
type NestedPluginRequest struct {
	Parent    string
	Import    string
	Signature string
}

// NestedPluginResolver returns a component path for an unresolved import.
// Returning an empty path means that no provider is available.
type NestedPluginResolver func(ctx context.Context, request NestedPluginRequest) (string, error)

func (o NestedPluginOptions) maxCandidates() int {
	if o.MaxCandidates == 0 {
		return defaultNestedPluginMaxCandidates
	}
	return o.MaxCandidates
}

type nestedLoadState struct {
	stack  []string
	active map[string]bool
	host   *PluginHost
}

func newNestedLoadState(host *PluginHost) *nestedLoadState {
	return &nestedLoadState{active: make(map[string]bool), host: host}
}

func (s *nestedLoadState) enter(path string) error {
	if s.active[path] {
		return fmt.Errorf("%w: %s -> %s", ErrNestedPluginCycle, strings.Join(s.stack, " -> "), path)
	}
	s.stack = append(s.stack, path)
	s.active[path] = true
	return nil
}

func (s *nestedLoadState) leave(path string) {
	delete(s.active, path)
	if len(s.stack) > 0 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

func resolveNestedImports(ctx context.Context, parent string, options RuntimeOptions, manual []HostImport, state *nestedLoadState) ([]HostImport, []*Runtime, []string, RuntimeOptions, error) {
	if options.NestedPlugins.Disabled {
		return manual, nil, nil, options, nil
	}
	memo := make(map[string][]CompositionPlug)
	plugs, childPaths, err := planNestedComposition(ctx, parent, options, manual, state, memo)
	if err != nil {
		return nil, nil, nil, options, err
	}
	options.CompositionPlugs = append(options.CompositionPlugs, plugs...)
	options.CompositionPlugs, err = normalizeComposition(parent, options.CompositionPlugs)
	if err != nil {
		return nil, nil, nil, options, err
	}
	return manual, nil, childPaths, options, nil
}

func planNestedComposition(ctx context.Context, parent string, options RuntimeOptions, manual []HostImport, state *nestedLoadState, memo map[string][]CompositionPlug) ([]CompositionPlug, []string, error) {
	manifest, err := inspectComponentBridge(ctx, parent, options)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect nested plugin parent %q: %w", parent, err)
	}
	manualNames := make(map[string]bool, len(manual))
	manualInterfaces := make(map[string]bool, len(manual))
	for _, item := range manual {
		manualNames[item.Interface+"#"+item.Function] = true
		manualInterfaces[item.Interface] = true
	}
	var unresolved []string
	for _, name := range manifest.Imports {
		if !manualNames[name] {
			interfaceName, _, ok := strings.Cut(name, "#")
			if ok && manualInterfaces[interfaceName] {
				return nil, nil, fmt.Errorf("%w: interface %q is only partially supplied by host callbacks", ErrPluginDependencyMismatch, interfaceName)
			}
			unresolved = append(unresolved, name)
		}
	}
	if len(unresolved) == 0 {
		return nil, nil, nil
	}

	providers, err := resolveNestedProviders(ctx, parent, unresolved, manifest.Signatures, options)
	if err != nil {
		return nil, nil, err
	}
	byInterface := make(map[string]string)
	for _, name := range unresolved {
		path := providers[name]
		if path == "" {
			return nil, nil, fmt.Errorf("%w: import=%q parent=%q", ErrNestedPluginNotFound, name, parent)
		}
		interfaceName, _, ok := strings.Cut(name, "#")
		if !ok || interfaceName == "" {
			return nil, nil, fmt.Errorf("invalid nested import name %q", name)
		}
		if previous := byInterface[interfaceName]; previous != "" && previous != path {
			return nil, nil, fmt.Errorf("%w: interface=%q providers=[%s %s]", ErrNestedPluginAmbiguous, interfaceName, previous, path)
		}
		byInterface[interfaceName] = path
	}
	interfaces := make([]string, 0, len(byInterface))
	for interfaceName := range byInterface {
		interfaces = append(interfaces, interfaceName)
	}
	sort.Strings(interfaces)
	plugs := make([]CompositionPlug, 0, len(interfaces))
	pathSet := make(map[string]bool)
	var childPaths []string
	for _, interfaceName := range interfaces {
		path := byInterface[interfaceName]
		dependencies, cached := memo[path]
		if !cached {
			if err := state.enter(path); err != nil {
				return nil, nil, err
			}
			childOptions := options
			childOptions.CompositionPlugs = nil
			dependencies, _, err = planNestedComposition(ctx, path, childOptions, nil, state, memo)
			state.leave(path)
			if err != nil {
				return nil, nil, fmt.Errorf("plan nested plugin %q for interface %q: %w", path, interfaceName, err)
			}
			memo[path] = cloneCompositionPlugs(dependencies)
		}
		plugs = append(plugs, CompositionPlug{Interface: interfaceName, Component: path, Dependencies: cloneCompositionPlugs(dependencies)})
		if !pathSet[path] {
			pathSet[path] = true
			childPaths = append(childPaths, path)
		}
	}
	sort.Strings(childPaths)
	return plugs, childPaths, nil
}

func splitNestedBudget(options RuntimeOptions, childCount int) (RuntimeOptions, RuntimeOptions, error) {
	if childCount == 0 {
		return options, options, nil
	}
	parts := uint64(childCount + 1)
	parent, child := options, options
	splitUint := func(name string, value uint64) (uint64, error) {
		if value == 0 {
			return 0, nil
		}
		share := value / parts
		if share == 0 {
			return 0, fmt.Errorf("%w: %s=%d cannot cover parent and %d direct children", ErrNestedPluginBudget, name, value, childCount)
		}
		return share, nil
	}
	splitInt := func(name string, value int64) (int64, error) {
		if value == 0 {
			return 0, nil
		}
		share := value / int64(parts)
		if share == 0 {
			return 0, fmt.Errorf("%w: %s=%d cannot cover parent and %d direct children", ErrNestedPluginBudget, name, value, childCount)
		}
		return share, nil
	}
	var err error
	if parent.Fuel, err = splitUint("Fuel", options.Fuel); err != nil {
		return options, options, err
	}
	child.Fuel = parent.Fuel
	if parent.FuelPerCall, err = splitUint("FuelPerCall", options.FuelPerCall); err != nil {
		return options, options, err
	}
	child.FuelPerCall = parent.FuelPerCall
	if parent.MemoryLimitBytes, err = splitInt("MemoryLimitBytes", options.MemoryLimitBytes); err != nil {
		return options, options, err
	}
	child.MemoryLimitBytes = parent.MemoryLimitBytes
	if parent.InstanceLimit, err = splitInt("InstanceLimit", options.InstanceLimit); err != nil {
		return options, options, err
	}
	child.InstanceLimit = parent.InstanceLimit
	return parent, child, nil
}

func resolveNestedProviders(ctx context.Context, parent string, imports []string, signatures map[string]string, options RuntimeOptions) (map[string]string, error) {
	result := make(map[string]string, len(imports))
	pluginManifest, manifestFound, err := ReadPluginManifest(parent)
	if err != nil {
		return nil, fmt.Errorf("read dependency manifest for %q: %w", parent, err)
	}
	if manifestFound {
		for _, name := range imports {
			path := pluginManifestDependency(pluginManifest, name)
			if path == "" {
				continue
			}
			normalized, err := resolveManifestDependency(parent, path, options.NestedPlugins.AllowedRoots)
			if err != nil {
				return nil, fmt.Errorf("resolve manifest dependency for import %q: %w", name, err)
			}
			contract, err := inspectComponentBridge(ctx, normalized, options)
			if err != nil {
				return nil, fmt.Errorf("inspect manifest dependency %q: %w", normalized, err)
			}
			if err := validateNestedProvider(name, signatures[name], contract); err != nil {
				return nil, fmt.Errorf("manifest dependency %q: %w", normalized, err)
			}
			result[name] = normalized
		}
		return result, nil
	}
	if resolver := options.NestedPlugins.Resolver; resolver != nil {
		for _, name := range imports {
			path, err := resolver(ctx, NestedPluginRequest{Parent: parent, Import: name, Signature: signatures[name]})
			if err != nil {
				return nil, fmt.Errorf("resolve nested import %q: %w", name, err)
			}
			if path == "" {
				continue
			}
			normalized, err := componentPath(resolveNestedPath(filepath.Dir(parent), path))
			if err != nil {
				return nil, err
			}
			contract, err := inspectComponentBridge(ctx, normalized, options)
			if err != nil {
				return nil, err
			}
			if err := validateNestedProvider(name, signatures[name], contract); err != nil {
				return nil, fmt.Errorf("nested resolver returned %q: %w", normalized, err)
			}
			result[name] = normalized
		}
		return result, nil
	}

	candidates, err := nestedCandidates(parent, options.NestedPlugins)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		contract, err := inspectComponentBridge(ctx, candidate, options)
		if err != nil {
			continue
		}
		for _, name := range imports {
			if !contract.Provides(name) || (signatures[name] != "" && contract.Signatures[name] != signatures[name]) {
				continue
			}
			if previous := result[name]; previous != "" && previous != candidate {
				return nil, fmt.Errorf("%w: import=%q providers=[%s %s]", ErrNestedPluginAmbiguous, name, previous, candidate)
			}
			result[name] = candidate
		}
	}
	return result, nil
}

func pluginManifestDependency(manifest PluginManifest, importName string) string {
	if path := manifest.Dependencies[importName]; path != "" {
		return path
	}
	if interfaceName, _, ok := strings.Cut(importName, "#"); ok {
		return manifest.Dependencies[interfaceName]
	}
	return ""
}

func resolveManifestDependency(parent, dependency string, allowedRoots []string) (string, error) {
	base := filepath.Dir(parent)
	if filepath.IsAbs(dependency) {
		return "", fmt.Errorf("%w: plugin manifests must use relative paths", ErrNestedPluginPathDenied)
	}
	path, err := filepath.Abs(filepath.Join(base, dependency))
	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)
	roots := allowedRoots
	if len(roots) == 0 {
		roots = []string{base}
	}
	allowed := false
	for _, root := range roots {
		root, err := filepath.Abs(resolveNestedPath(base, root))
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(filepath.Clean(root), path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("%w: %s", ErrNestedPluginPathDenied, path)
	}
	return componentPath(path)
}

func validateNestedProvider(name, signature string, contract Contract) error {
	if !contract.Provides(name) {
		return fmt.Errorf("component does not export %q", name)
	}
	if signature != "" && contract.Signatures[name] != signature {
		return fmt.Errorf("signature for %q differs: import=%q export=%q", name, signature, contract.Signatures[name])
	}
	return nil
}

func nestedCandidates(parent string, options NestedPluginOptions) ([]string, error) {
	base := filepath.Dir(parent)
	roots := options.SearchPaths
	if len(roots) == 0 {
		roots = []string{base}
	}
	seen := map[string]bool{parent: true}
	var result []string
	limit := options.maxCandidates()
	var visit func(string) error
	visit = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read nested plugin directory %q: %w", dir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if err := visit(path); err != nil {
					return err
				}
				continue
			}
			if !strings.EqualFold(filepath.Ext(entry.Name()), ".wasm") {
				continue
			}
			normalized, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			normalized = filepath.Clean(normalized)
			if seen[normalized] {
				continue
			}
			seen[normalized] = true
			result = append(result, normalized)
			if len(result) > limit {
				return fmt.Errorf("nested plugin candidate limit exceeded: maximum=%d", limit)
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := visit(resolveNestedPath(base, root)); err != nil {
			return nil, err
		}
	}
	sort.Strings(result)
	return result, nil
}

func resolveNestedPath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

func containsRuntimeHandle(value any) bool {
	switch value := value.(type) {
	case Handle:
		return true
	case []any:
		for _, item := range value {
			if containsRuntimeHandle(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range value {
			if containsRuntimeHandle(item) {
				return true
			}
		}
	}
	return false
}

func closeRuntimes(runtimes []*Runtime) {
	for index := len(runtimes) - 1; index >= 0; index-- {
		_ = runtimes[index].Close()
	}
}
