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
	wit         string
	output      string
	packageName string
	filename    string
	language    string
	witMode     string
}

func main() {
	opts := parseFlags()
	mode, err := parseWITMode(opts.witMode)
	if err != nil {
		renderGenerationError(err, normalizeLanguage(opts.language))
		os.Exit(2)
	}

	err = witgo.Generate(witgo.Config{
		WIT:      opts.wit,
		WITMode:  mode,
		Output:   opts.output,
		Package:  opts.packageName,
		Filename: opts.filename,
	})
	if err != nil {
		renderGenerationError(err, normalizeLanguage(opts.language))
		os.Exit(1)
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
	flag.StringVar(&opts.output, "out", "./internal/contract", "generated package directory")
	flag.StringVar(&opts.packageName, "package", "", "Go package name; defaults to WIT package name")
	flag.StringVar(&opts.filename, "filename", "", "generated filename; defaults to bindings.gen.go")
	flag.StringVar(&opts.language, "lang", os.Getenv("LANG"), "diagnostic language, for example ru or en")
	flag.Parse()
	return opts
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
