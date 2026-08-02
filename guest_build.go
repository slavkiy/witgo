package witgo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GuestBuildConfig describes a TinyGo WASI Preview 2 component build. The WIT
// package is embedded by TinyGo as Component Type metadata and used to turn the
// Canonical ABI core module into a WebAssembly Component.
type GuestBuildConfig struct {
	// Main is the plugin main package or .go file.
	Main string
	// WITPackage is a WIT package accepted by TinyGo --wit-package. TinyGo
	// accepts a WIT package directory; a packaged .wasm may also be used.
	WITPackage string
	// World is the implemented WIT world.
	World string
	// Output is the resulting component .wasm path.
	Output string
	// TinyGo optionally overrides the tinygo executable path.
	TinyGo string
	// NoDebug removes debug data from the component.
	NoDebug bool
	// Manifest is embedded after a successful build. A nil manifest omits the
	// witgo custom section; an empty non-nil manifest is still embedded.
	Manifest *PluginManifest
}

// BuildGuestComponent builds a registered Go guest as a WASI Preview 2
// WebAssembly Component. Canonical ABI glue must have been generated in guest
// mode before this function is called.
func BuildGuestComponent(config GuestBuildConfig) error {
	if strings.TrimSpace(config.Main) == "" {
		return fmt.Errorf("guest main package is required")
	}
	if strings.TrimSpace(config.WITPackage) == "" {
		return fmt.Errorf("guest WIT package is required")
	}
	if strings.TrimSpace(config.World) == "" {
		return fmt.Errorf("guest WIT world is required")
	}
	if strings.TrimSpace(config.Output) == "" {
		return fmt.Errorf("guest component output is required")
	}
	tool := strings.TrimSpace(config.TinyGo)
	if tool == "" {
		var err error
		tool, err = exec.LookPath("tinygo")
		if err != nil {
			return fmt.Errorf("find tinygo: %w", err)
		}
	}
	args := []string{"build", "-target=wasip2", "-o", config.Output, "--wit-package", config.WITPackage, "--wit-world", config.World}
	if config.NoDebug {
		args = append(args, "-no-debug")
	}
	args = append(args, config.Main)
	command := exec.Command(tool, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("build TinyGo guest component: %w\n%s", err, strings.TrimSpace(output.String()))
	}
	if config.Manifest != nil {
		component, err := os.ReadFile(config.Output)
		if err != nil {
			return fmt.Errorf("read built guest component: %w", err)
		}
		component, err = EmbedPluginManifest(component, *config.Manifest)
		if err != nil {
			return fmt.Errorf("embed plugin manifest: %w", err)
		}
		if err := os.WriteFile(config.Output, component, 0o644); err != nil {
			return fmt.Errorf("write guest component with manifest: %w", err)
		}
	}
	return nil
}

// PluginBuildConfig describes the complete generated-contract plugin build.
type PluginBuildConfig struct {
	Generate Config
	Build    GuestBuildConfig
}

// BuildPlugin generates guest bindings and builds the loadable component.
func BuildPlugin(config PluginBuildConfig) error {
	config.Generate.Mode = GenerateGuest
	if config.Generate.World == "" {
		config.Generate.World = config.Build.World
	}
	if config.Build.World == "" {
		config.Build.World = config.Generate.World
	}
	if config.Build.WITPackage == "" {
		config.Build.WITPackage = config.Generate.WIT
	}
	if err := Generate(config.Generate); err != nil {
		return fmt.Errorf("generate plugin contract: %w", err)
	}
	return BuildGuestComponent(config.Build)
}
