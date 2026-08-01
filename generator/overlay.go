package generator

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"os"
	"path"
	"strings"

	"github.com/slavkiy/witgo/parser/ir"
	"gopkg.in/yaml.v3"
)

// GoOverlay describes optional public Go representations over canonical WIT
// wire types. It is resolved and validated before rendering.
type GoOverlay struct {
	Version int                       `yaml:"version"`
	Types   map[string]GoTypeMapping  `yaml:"types"`
	Errors  map[string]GoErrorMapping `yaml:"errors"`
	path    string
	types   map[string]resolvedTypeMapping
	errors  map[string]resolvedErrorMapping
	decls   map[string]any
}

// GoTypeMapping maps one canonical WIT alias to a public Go type.
type GoTypeMapping struct {
	GoType string `yaml:"go_type"`
	Import string `yaml:"import"`
	Codec  string `yaml:"codec"`
}

// GoErrorMapping maps a WIT result return to idiomatic Go error returns.
type GoErrorMapping struct {
	ResultError bool `yaml:"result_error"`
}

type resolvedTypeMapping struct {
	ID        string
	Interface string
	Name      string
	Wire      ir.Type
	Config    GoTypeMapping
}

type resolvedErrorMapping struct {
	ID        string
	Interface string
	Name      string
	Function  *ir.Func
}

// LoadGoOverlay reads and validates a version 1 overlay. An empty path disables
// overlays and preserves historical generation exactly.
func LoadGoOverlay(filename string, file *ir.File) (*GoOverlay, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read Go overlay %q: %w", filename, err)
	}
	var overlay GoOverlay
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&overlay); err != nil {
		return nil, fmt.Errorf("parse Go overlay %q: %w", filename, err)
	}
	overlay.path = filename
	if overlay.Version != 1 {
		return nil, fmt.Errorf("Go overlay %q: unsupported schema version %d, expected 1", filename, overlay.Version)
	}
	if err := overlay.resolve(file); err != nil {
		return nil, err
	}
	return &overlay, nil
}

func (o *GoOverlay) resolve(file *ir.File) error {
	if file == nil || file.Package == nil {
		return fmt.Errorf("Go overlay %q: WIT package declaration is required", o.path)
	}
	o.types = make(map[string]resolvedTypeMapping)
	o.errors = make(map[string]resolvedErrorMapping)
	o.decls = make(map[string]any)
	typeIndex := make(map[string]resolvedTypeMapping)
	functionIndex := make(map[string]resolvedErrorMapping)
	for _, decl := range file.Decls {
		iface, ok := decl.(*ir.Interface)
		if !ok {
			continue
		}
		for _, item := range iface.Items {
			switch named := item.(type) {
			case *ir.TypeDef:
				o.decls[named.Name] = named
			case *ir.Record:
				o.decls[named.Name] = named
			case *ir.Variant:
				o.decls[named.Name] = named
			}
			switch item := item.(type) {
			case *ir.TypeDef:
				id := canonicalOverlayID(file.Package, iface.Name, item.Name)
				typeIndex[id] = resolvedTypeMapping{ID: id, Interface: iface.Name, Name: item.Name, Wire: item.Type}
			case *ir.Func:
				id := canonicalOverlayID(file.Package, iface.Name, item.Name)
				functionIndex[id] = resolvedErrorMapping{ID: id, Interface: iface.Name, Name: item.Name, Function: item}
			}
		}
	}
	for id, config := range o.Types {
		resolved, ok := typeIndex[id]
		if !ok {
			return o.errorf(id, "canonical WIT alias was not found")
		}
		if err := validateGoType(config); err != nil {
			return o.errorf(id, "%v", err)
		}
		if err := validateCodec(config.Codec, resolved.Wire, config.GoType); err != nil {
			return o.errorf(id, "%v", err)
		}
		resolved.Config = config
		o.types[resolved.Name] = resolved
	}
	for id, config := range o.Errors {
		if !config.ResultError {
			continue
		}
		resolved, ok := functionIndex[id]
		if !ok {
			return o.errorf(id, "canonical WIT function was not found")
		}
		generic, ok := resolved.Function.Result.(*ir.GenericType)
		if !ok || generic.Name != "result" {
			return o.errorf(id, "result_error requires a function returning result<T, E>")
		}
		o.errors[resolved.Interface+"#"+resolved.Name] = resolved
	}
	if err := o.validateSupportedShapes(file); err != nil {
		return err
	}
	return nil
}

func (o *GoOverlay) validateSupportedShapes(file *ir.File) error {
	for _, decl := range file.Decls {
		iface, ok := decl.(*ir.Interface)
		if !ok {
			continue
		}
		for _, item := range iface.Items {
			switch item := item.(type) {
			case *ir.TypeDef:
				if _, mapped := o.types[item.Name]; !mapped {
					if id, reason := o.unsupportedNested(item.Type, false, map[string]bool{}); id != "" {
						return o.errorf(id, "used by %s#%s: %s", iface.Name, item.Name, reason)
					}
				}
			case *ir.Record:
				for _, field := range item.Fields {
					if id, reason := o.unsupportedNested(field.Type, false, map[string]bool{}); id != "" {
						return o.errorf(id, "used by record %s.%s: %s", item.Name, field.Name, reason)
					}
				}
			case *ir.Variant:
				for _, variantCase := range item.Cases {
					if id := o.mappingInType(variantCase.Type, map[string]bool{}); id != "" {
						return o.errorf(id, "mapping inside variant %s case %s is not supported yet", item.Name, variantCase.Name)
					}
				}
			case *ir.Func:
				for _, param := range item.Params {
					if id, reason := o.unsupportedNested(param.Type, false, map[string]bool{}); id != "" {
						return o.errorf(id, "used by function %s parameter %s: %s", item.Name, param.Name, reason)
					}
				}
				if id, reason := o.unsupportedNested(item.Result, false, map[string]bool{}); id != "" {
					return o.errorf(id, "used by function %s result: %s", item.Name, reason)
				}
			}
		}
	}
	return nil
}

func (o *GoOverlay) unsupportedNested(typ ir.Type, nested bool, seen map[string]bool) (string, string) {
	switch typ := typ.(type) {
	case *ir.NamedType:
		if mapping, ok := o.types[typ.Name]; ok {
			if nested {
				return mapping.ID, "mapping inside a generic or tuple is not supported yet"
			}
			return "", ""
		}
	case *ir.GenericType:
		for _, arg := range typ.Args {
			if id := o.mappingInType(arg, seen); id != "" {
				return id, "mapping inside " + typ.Name + " is not supported yet"
			}
		}
	case *ir.TupleType:
		for _, item := range typ.Items {
			if id := o.mappingInType(item, seen); id != "" {
				return id, "mapping inside tuple is not supported yet"
			}
		}
	}
	return "", ""
}

func (o *GoOverlay) mappingInType(typ ir.Type, seen map[string]bool) string {
	switch typ := typ.(type) {
	case *ir.NamedType:
		if mapping, ok := o.types[typ.Name]; ok {
			return mapping.ID
		}
		if seen[typ.Name] {
			return ""
		}
		seen[typ.Name] = true
		switch decl := o.decls[typ.Name].(type) {
		case *ir.TypeDef:
			return o.mappingInType(decl.Type, seen)
		case *ir.Record:
			for _, field := range decl.Fields {
				if id := o.mappingInType(field.Type, seen); id != "" {
					return id
				}
			}
		case *ir.Variant:
			for _, variantCase := range decl.Cases {
				if id := o.mappingInType(variantCase.Type, seen); id != "" {
					return id
				}
			}
		}
	case *ir.GenericType:
		for _, arg := range typ.Args {
			if id := o.mappingInType(arg, seen); id != "" {
				return id
			}
		}
	case *ir.TupleType:
		for _, item := range typ.Items {
			if id := o.mappingInType(item, seen); id != "" {
				return id
			}
		}
	}
	return ""
}

func validateGoType(config GoTypeMapping) error {
	if strings.TrimSpace(config.GoType) == "" {
		return fmt.Errorf("go_type is required")
	}
	expr, err := goparser.ParseExpr(config.GoType)
	if err != nil {
		return fmt.Errorf("invalid go_type %q: %w", config.GoType, err)
	}
	valid := true
	ast.Inspect(expr, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.StarExpr, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.MapType:
			valid = false
		}
		return valid
	})
	if !valid || strings.Contains(config.GoType, "unsafe.Pointer") || config.GoType == "context.Context" {
		return fmt.Errorf("go_type %q is not allowed in overlays", config.GoType)
	}
	if config.Import != "" {
		qualifier := strings.Split(config.GoType, ".")[0]
		if !strings.Contains(config.GoType, ".") || qualifier != path.Base(config.Import) {
			return fmt.Errorf("go_type %q must use import package name %q", config.GoType, path.Base(config.Import))
		}
	} else if strings.Contains(config.GoType, ".") {
		return fmt.Errorf("import is required for qualified go_type %q", config.GoType)
	}
	if strings.TrimSpace(config.Codec) == "" {
		return fmt.Errorf("codec is required")
	}
	return nil
}

func validateCodec(codec string, wire ir.Type, goType string) error {
	primitive, ok := wire.(*ir.PrimitiveType)
	if !ok {
		return fmt.Errorf("codec %q currently requires a primitive alias wire type", codec)
	}
	switch codec {
	case "unix-seconds", "unix-milliseconds", "unix-nanoseconds":
		if primitive.Name != "s64" || goType != "time.Time" {
			return fmt.Errorf("codec %q requires WIT s64 and go_type time.Time", codec)
		}
	case "duration-nanoseconds":
		if primitive.Name != "s64" || goType != "time.Duration" {
			return fmt.Errorf("codec %q requires WIT s64 and go_type time.Duration", codec)
		}
	default:
		return fmt.Errorf("unknown codec %q", codec)
	}
	return nil
}

func canonicalOverlayID(pkg *ir.Package, iface, member string) string {
	id := pkg.Namespace + ":" + pkg.Name + "/" + iface
	if pkg.Version != "" {
		id += "@" + pkg.Version
	}
	return id + "#" + member
}

func (o *GoOverlay) errorf(id, format string, args ...any) error {
	return fmt.Errorf("Go overlay %q mapping %q: %s", o.path, id, fmt.Sprintf(format, args...))
}
