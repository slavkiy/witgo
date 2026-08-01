package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/slavkiy/witgo"
)

type options struct {
	wit          string
	output       string
	packageName  string
	filename     string
	language     string
	witMode      string
	goOverlay    string
	mode         string
	world        string
	packageRoot  string
	guestBindgen string
	buildMain    string
	componentOut string
	witPackage   string
	tinygo       string
	noDebug      bool
}

func main() {
	opts := parseFlags()
	mode, err := parseWITMode(opts.witMode)
	if err != nil {
		renderGenerationError(err, normalizeLanguage(opts.language))
		os.Exit(2)
	}
	generationMode, err := parseGenerationMode(opts.mode)
	if err != nil {
		renderGenerationError(err, normalizeLanguage(opts.language))
		os.Exit(2)
	}
	err = witgo.Generate(witgo.Config{
		WIT:          opts.wit,
		WITMode:      mode,
		GoOverlay:    opts.goOverlay,
		Output:       opts.output,
		Package:      opts.packageName,
		Filename:     opts.filename,
		Mode:         generationMode,
		World:        opts.world,
		PackageRoot:  opts.packageRoot,
		GuestBindgen: opts.guestBindgen,
	})
	if err != nil {
		renderGenerationError(err, normalizeLanguage(opts.language))
		os.Exit(1)
	}
	if generationMode == witgo.GenerateGuest && strings.TrimSpace(opts.buildMain) != "" {
		witPackage := opts.witPackage
		if witPackage == "" {
			witPackage = opts.wit
		}
		componentOut := opts.componentOut
		if componentOut == "" {
			componentOut = "plugin.component.wasm"
		}
		if err := witgo.BuildGuestComponent(witgo.GuestBuildConfig{
			Main: opts.buildMain, WITPackage: witPackage, World: opts.world,
			Output: componentOut, TinyGo: opts.tinygo, NoDebug: opts.noDebug,
		}); err != nil {
			renderGenerationError(err, normalizeLanguage(opts.language))
			os.Exit(1)
		}
	}

	output := opts.filename
	if output == "" {
		output = "bindings.gen.go"
	}
	renderSuccess(filepath.Join(opts.output, output), normalizeLanguage(opts.language))
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.wit, "wit", "./wit", "WIT file or directory")
	flag.StringVar(&opts.witMode, "wit-mode", "auto", "WIT input mode: auto, file, package, or tree")
	flag.StringVar(&opts.goOverlay, "go-overlay", "", "optional Go type overlay YAML file")
	flag.StringVar(&opts.mode, "mode", "host", "generation mode: host or guest")
	flag.StringVar(&opts.world, "world", "", "WIT world to generate in guest mode")
	flag.StringVar(&opts.packageRoot, "package-root", "", "Go import path of the guest output directory")
	flag.StringVar(&opts.guestBindgen, "guest-bindgen", "", "path to wit-bindgen-go (guest mode)")
	flag.StringVar(&opts.buildMain, "build-main", "", "plugin main package/file to build after guest generation")
	flag.StringVar(&opts.componentOut, "component-out", "", "TinyGo component output; defaults to plugin.component.wasm")
	flag.StringVar(&opts.witPackage, "wit-package", "", "packaged WIT .wasm passed to TinyGo; defaults to -wit")
	flag.StringVar(&opts.tinygo, "tinygo", "", "path to tinygo")
	flag.BoolVar(&opts.noDebug, "no-debug", false, "strip TinyGo debug data")
	flag.StringVar(&opts.output, "out", "./internal/contract", "generated package directory")
	flag.StringVar(&opts.packageName, "package", "", "Go package name; defaults to WIT package name")
	flag.StringVar(&opts.filename, "filename", "", "generated filename; defaults to bindings.gen.go")
	flag.StringVar(&opts.language, "lang", os.Getenv("LANG"), "diagnostic language, for example ru or en")
	flag.Parse()
	return opts
}

func parseGenerationMode(value string) (witgo.GenerationMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "host", "":
		return witgo.GenerateHost, nil
	case "guest", "plugin":
		return witgo.GenerateGuest, nil
	default:
		return witgo.GenerateHost, fmt.Errorf("unknown generation mode %q", value)
	}
}

func parseWITMode(value string) (witgo.WITInputMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "":
		return witgo.WITInputAuto, nil
	case "file":
		return witgo.WITInputFile, nil
	case "package", "pkg":
		return witgo.WITInputPackage, nil
	case "tree", "recursive":
		return witgo.WITInputTree, nil
	default:
		return witgo.WITInputAuto, fmt.Errorf("unknown WIT input mode %q", value)
	}
}

func renderGenerationError(generationErr error, language string) {
	message := "WIT generation failed"
	if language == "rus" {
		message = "не удалось сгенерировать Go-код из WIT"
	}
	fmt.Fprintf(os.Stderr, "ERROR GenerationFailed: %s\n%s\n", message, generationErr)
}

func renderSuccess(filename, language string) {
	message := "Go bindings generated"
	if language == "rus" {
		message = "Go bindings сгенерированы"
	}
	fmt.Printf("OK GenerationComplete: %s\n%s\n", message, filename)
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if separator := strings.IndexAny(language, "_.-"); separator >= 0 {
		language = language[:separator]
	}
	switch language {
	case "en":
		return "eng"
	case "ru":
		return "rus"
	case "uk":
		return "ukr"
	case "", "c", "posix":
		return "eng"
	default:
		return language
	}
}
