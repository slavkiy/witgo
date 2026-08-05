package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/slavkiy/witgo/parser/ir"
)

const guestBindgenVersion = "v0.7.0"

func (g *Generator) generateGuest(model *ir.File, packageName string) error {
	world, err := selectGuestWorld(model, g.config.World)
	if err != nil {
		return err
	}
	packageRoot, err := guestPackageRoot(g.config.Output, g.config.PackageRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(g.config.Output, 0o755); err != nil {
		return fmt.Errorf("create guest output directory: %w", err)
	}
	source, err := renderGuestFacade(model, world, packageName, packageRoot)
	if err != nil {
		return fmt.Errorf("render guest facade: %w", err)
	}
	source, err = format.Source(source)
	if err != nil {
		return fmt.Errorf("format guest facade: %w\n%s", err, source)
	}
	if err := g.runGuestBindgen(world.Name, packageRoot); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(g.config.Output, g.config.Filename), source)
}

func selectGuestWorld(model *ir.File, selected string) (*ir.World, error) {
	var worlds []*ir.World
	for _, decl := range model.Decls {
		if world, ok := decl.(*ir.World); ok {
			worlds = append(worlds, world)
		}
	}
	selected = strings.TrimSpace(selected)
	if selected != "" {
		for _, world := range worlds {
			if world.Name == selected {
				return world, nil
			}
		}
		return nil, fmt.Errorf("WIT world %q was not found", selected)
	}
	if len(worlds) != 1 {
		return nil, fmt.Errorf("guest generation requires -world when the input contains %d worlds", len(worlds))
	}
	return worlds[0], nil
}

func guestPackageRoot(output, configured string) (string, error) {
	if root := strings.Trim(strings.TrimSpace(configured), "/"); root != "" {
		return root, nil
	}
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("resolve guest output: %w", err)
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
		if readErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 2 && fields[0] == "module" {
					rel, relErr := filepath.Rel(dir, abs)
					if relErr != nil {
						return "", relErr
					}
					if rel == "." {
						return fields[1], nil
					}
					return strings.TrimSuffix(fields[1], "/") + "/" + filepath.ToSlash(rel), nil
				}
			}
			return "", fmt.Errorf("go.mod at %q has no module directive", dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("cannot derive guest PackageRoot for %q; set Config.PackageRoot", output)
}

func (g *Generator) runGuestBindgen(world, packageRoot string) error {
	tool := strings.TrimSpace(g.config.GuestBindgen)
	var command *exec.Cmd
	if tool != "" {
		command = exec.Command(tool)
	} else if path, err := exec.LookPath("wit-bindgen-go"); err == nil {
		command = exec.Command(path)
	} else {
		command = exec.Command("go", "run", "go.bytecodealliance.org/cmd/wit-bindgen-go@"+guestBindgenVersion)
	}
	outputPath, err := filepath.Abs(g.config.Output)
	if err != nil {
		return fmt.Errorf("resolve guest output: %w", err)
	}
	args := []string{"generate", "--world", world, "--out", outputPath, "--package-root", packageRoot}
	input := g.config.WIT
	if len(g.config.WITFiles) != 0 {
		return fmt.Errorf("guest generation currently requires a WIT file or package directory; WITFiles is not supported")
	}
	if info, err := os.Stat(input); err == nil {
		if info.IsDir() {
			command.Dir = input
			input = "."
		} else {
			command.Dir = filepath.Dir(input)
			input = filepath.Base(input)
		}
	}
	command.Args = append(command.Args, args...)
	command.Args = append(command.Args, input)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("generate Canonical ABI guest bindings with wit-bindgen-go: %w\n%s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

func renderGuestFacade(model *ir.File, world *ir.World, packageName, packageRoot string) ([]byte, error) {
	r := &renderer{interfaces: map[string]*ir.Interface{}, records: map[string]*ir.Record{}, namedTypes: map[string]any{}, modelFuncs: map[string][]*ir.Func{}, exports: map[string]bool{}, emitted: map[string]string{}, witPackage: model.Package, goImports: map[string]bool{}}
	r.collect(model)
	imports, exports, direct, err := r.worldParts(world)
	if err != nil {
		return nil, err
	}
	if len(direct) != 0 {
		return nil, fmt.Errorf("guest facade does not yet support direct world functions; put them in an interface")
	}
	if len(exports) == 0 {
		return nil, fmt.Errorf("guest world %q has no exports", world.Name)
	}
	if model.Package == nil {
		return nil, fmt.Errorf("guest generation requires a WIT package declaration")
	}
	base := packageRoot + "/" + model.Package.Namespace + "/" + model.Package.Name
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by witgo guest mode. DO NOT EDIT.")
	fmt.Fprintf(&out, "\npackage %s\n\n", packageName)
	fmt.Fprintln(&out, "import (")
	fmt.Fprintln(&out, "\t\"fmt\"")
	all := append(append([]worldInterface(nil), imports...), exports...)
	sort.Slice(all, func(i, j int) bool { return all[i].name < all[j].name })
	aliases := map[string]string{}
	for _, iface := range all {
		if _, exists := aliases[iface.name]; exists {
			continue
		}
		alias := goLocalName(iface.name)
		aliases[iface.name] = alias
		fmt.Fprintf(&out, "\t%s %s\n", alias, strconv.Quote(base+"/"+iface.name))
	}
	fmt.Fprintln(&out, ")")
	fmt.Fprintln(&out)

	worldName := goName(world.Name)
	for _, exported := range exports {
		alias := aliases[exported.name]
		fmt.Fprintf(&out, "// %sGuest is implemented by the Go plugin export.\n", exported.interfaceName)
		fmt.Fprintf(&out, "type %sGuest interface {\n", exported.interfaceName)
		for _, fn := range exported.functions {
			fmt.Fprintf(&out, "\t%s%s\n", goName(fn.Name), guestFunctionSignature(fn, alias))
		}
		fmt.Fprintln(&out, "}")
		fmt.Fprintln(&out)
	}
	fmt.Fprintf(&out, "// %sGuest contains every export implemented by the plugin.\n", worldName)
	fmt.Fprintf(&out, "type %sGuest struct {\n", worldName)
	for _, exported := range exports {
		fmt.Fprintf(&out, "\t%s %sGuest\n", goName(exported.name), exported.interfaceName)
	}
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "// Export%s registers all guest exports before the component starts.\n", worldName)
	fmt.Fprintf(&out, "func Export%s(implementation %sGuest) error {\n", worldName, worldName)
	for _, exported := range exports {
		field := goName(exported.name)
		alias := aliases[exported.name]
		fmt.Fprintf(&out, "\tif implementation.%s == nil { return fmt.Errorf(%q) }\n", field, "guest export "+exported.name+" is nil")
		for _, fn := range exported.functions {
			fmt.Fprintf(&out, "\t%s.Exports.%s = implementation.%s.%s\n", alias, goName(fn.Name), field, goName(fn.Name))
		}
	}
	fmt.Fprintln(&out, "\treturn nil")
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out)

	for _, imported := range imports {
		client := imported.interfaceName + "GuestImport"
		alias := aliases[imported.name]
		fmt.Fprintf(&out, "type %s struct{}\n\n", client)
		for _, fn := range imported.functions {
			fmt.Fprintf(&out, "func (%s) %s%s {\n", client, goName(fn.Name), guestFunctionSignature(fn, alias))
			args := make([]string, len(fn.Params))
			for i, param := range fn.Params {
				args[i] = paramName(param, i)
			}
			if fn.Result != nil {
				fmt.Fprint(&out, "\treturn ")
			} else {
				fmt.Fprint(&out, "\t")
			}
			fmt.Fprintf(&out, "%s.%s(%s)\n}\n\n", alias, goName(fn.Name), strings.Join(args, ", "))
		}
	}
	fmt.Fprintln(&out, "// Imports exposes host functions to guest code.")
	fmt.Fprintln(&out, "var Imports = struct {")
	for _, imported := range imports {
		fmt.Fprintf(&out, "\t%s %sGuestImport\n", goName(imported.name), imported.interfaceName)
	}
	fmt.Fprintln(&out, "}{}")
	return out.Bytes(), nil
}

func guestFunctionSignature(fn *ir.Func, packageAlias string) string {
	params := make([]string, 0, len(fn.Params))
	for i, param := range fn.Params {
		params = append(params, paramName(param, i)+" "+guestType(param.Type, packageAlias))
	}
	result := ""
	if fn.Result != nil {
		result = " " + guestType(fn.Result, packageAlias)
	}
	return "(" + strings.Join(params, ", ") + ")" + result
}

func guestType(typ ir.Type, packageAlias string) string {
	switch typ := typ.(type) {
	case *ir.PrimitiveType:
		if value, ok := primitiveType(typ.Name); ok {
			return value
		}
	case *ir.NamedType:
		if value, ok := primitiveType(typ.Name); ok {
			return value
		}
		return packageAlias + "." + goName(typ.Name)
	}
	// Complex anonymous Component Model types are intentionally rejected by
	// go/format later with a useful source location rather than silently using
	// the host-side witgo representations, which have a different ABI layout.
	return "any"
}
