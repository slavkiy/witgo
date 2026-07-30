package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rp1s/digreyt"
	"github.com/rp1s/digreyt/translate"
	"github.com/slavkiy/witgo"
)

type options struct {
	wit           string
	output        string
	packageName   string
	filename      string
	language      string
	autoTranslate bool
}

func main() {
	opts := parseFlags()
	translate.SetLanguage(normalizeLanguage(opts.language))

	err := witgo.Generate(witgo.Config{
		WIT:      opts.wit,
		Output:   opts.output,
		Package:  opts.packageName,
		Filename: opts.filename,
	})
	if err != nil {
		renderGenerationError(err, opts.autoTranslate)
		os.Exit(1)
	}

	output := opts.filename
	if output == "" {
		output = "bindings.gen.go"
	}
	renderSuccess(filepath.Join(opts.output, output))
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.wit, "wit", "./wit", "WIT file or directory")
	flag.StringVar(&opts.output, "out", "./internal/contract", "generated package directory")
	flag.StringVar(&opts.packageName, "package", "", "Go package name; defaults to WIT package name")
	flag.StringVar(&opts.filename, "filename", "", "generated filename; defaults to bindings.gen.go")
	flag.StringVar(&opts.language, "lang", os.Getenv("LANG"), "diagnostic language, for example ru or en")
	flag.BoolVar(&opts.autoTranslate, "auto-translate", true, "translate missing diagnostic text automatically")
	flag.Parse()
	return opts
}

func renderGenerationError(generationErr error, autoTranslate bool) {
	diagnostic := digreyt.Error{
		CodeName: "GenerationFailed",
		Severity: digreyt.SeverityError,
		MessageTranslations: translate.Translations{
			{Language: "eng", Text: "WIT generation failed"},
			{Language: "rus", Text: "не удалось сгенерировать Go-код из WIT"},
		},
		DescriptionTranslations: []translate.Translations{
			{{Language: "eng", Text: generationErr.Error()}},
		},
	}

	if autoTranslate && translate.Language() != translate.DefaultLanguage {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		localized, err := diagnostic.LocalizeAuto(ctx)
		if err == nil {
			diagnostic = localized
		} else {
			diagnostic = diagnostic.Localize()
			diagnostic.Description = []string{
				generationErr.Error(),
				fmt.Sprintf("automatic translation unavailable: %v", err),
			}
		}
	} else {
		diagnostic = diagnostic.Localize()
	}

	arena := digreyt.New("")
	arena.Add(diagnostic)
	_ = arena.Render(os.Stderr)
}

func renderSuccess(filename string) {
	arena := digreyt.New("")
	arena.Add(digreyt.Error{
		CodeName: "GenerationComplete",
		Severity: digreyt.SeveritySuccess,
		MessageTranslations: translate.Translations{
			{Language: "eng", Text: "Go bindings generated"},
			{Language: "rus", Text: "Go bindings сгенерированы"},
		},
		Description: []string{filename},
	})
	_ = arena.Render(os.Stdout)
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
		return translate.DefaultLanguage
	default:
		return language
	}
}
