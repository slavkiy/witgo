package analysis

import (
	"fmt"

	"github.com/slavkiy/witgo/internal/parser/ast"
)

type BuiltinKind uint8

const (
	BuiltinPrimitive BuiltinKind = iota
	BuiltinGeneric
)

type TypeInfo struct {
	Name  string
	Kind  BuiltinKind
	Arity int
}

type SymbolKind uint8

const (
	SymbolType SymbolKind = iota
	SymbolFunction
	SymbolInterface
	SymbolWorld
	SymbolImport
	SymbolExport
)

type Symbol struct {
	Name string
	Kind SymbolKind
	Node ast.Node
}

type Scope struct {
	parent  *Scope
	symbols map[string]Symbol
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, symbols: map[string]Symbol{}}
}

func (s *Scope) Define(sym Symbol) error {
	if _, exists := s.symbols[sym.Name]; exists {
		return fmt.Errorf("duplicate symbol %q", sym.Name)
	}
	s.symbols[sym.Name] = sym
	return nil
}

func (s *Scope) Lookup(name string) (Symbol, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if sym, ok := cur.symbols[name]; ok {
			return sym, true
		}
	}
	return Symbol{}, false
}

type Result struct {
	Globals  *Scope
	Builtins map[string]TypeInfo
}

func Analyze(file *ast.File) (*Result, error) {
	res := &Result{Globals: NewScope(nil), Builtins: builtins()}
	for _, decl := range file.Decls {
		if err := defineTopLevel(res, decl); err != nil {
			return nil, err
		}
	}
	for _, useDecl := range file.Uses {
		_ = useDecl
	}
	for _, decl := range file.Decls {
		if err := analyzeDecl(res, decl); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func defineTopLevel(res *Result, decl ast.Decl) error {
	switch d := decl.(type) {
	case *ast.InterfaceDecl:
		return res.Globals.Define(Symbol{Name: d.Name.Text, Kind: SymbolInterface, Node: d})
	case *ast.WorldDecl:
		return res.Globals.Define(Symbol{Name: d.Name.Text, Kind: SymbolWorld, Node: d})
	case interface{ typeDef(); Span() ast.Span }:
		return res.Globals.Define(Symbol{Name: extractTypeName(decl), Kind: SymbolType, Node: decl})
	default:
		return nil
	}
}

func analyzeDecl(res *Result, decl ast.Decl) error {
	switch d := decl.(type) {
	case *ast.InterfaceDecl:
		return analyzeInterface(res, d)
	case *ast.WorldDecl:
		return analyzeWorld(res, d)
	case *ast.TypeDecl:
		return validateType(res, res.Globals, d.Type)
	case *ast.RecordDecl, *ast.FlagsDecl, *ast.EnumDecl:
		return nil
	case *ast.VariantDecl:
		for _, c := range d.Cases {
			if c.Type != nil {
				if err := validateType(res, res.Globals, c.Type); err != nil {
					return err
				}
			}
		}
	case *ast.ResourceDecl:
		return analyzeResource(res, res.Globals, d)
	}
	return nil
}

func analyzeInterface(res *Result, decl *ast.InterfaceDecl) error {
	scope := NewScope(res.Globals)
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.TypeDecl, *ast.RecordDecl, *ast.FlagsDecl, *ast.EnumDecl, *ast.VariantDecl, *ast.ResourceDecl:
			if err := scope.Define(Symbol{Name: extractTypeName(it), Kind: SymbolType, Node: it}); err != nil {
				return err
			}
		case *ast.FuncDecl:
			if err := scope.Define(Symbol{Name: it.Name.Text, Kind: SymbolFunction, Node: it}); err != nil {
				return err
			}
		}
	}
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.UseDecl:
		case *ast.TypeDecl:
			if err := validateType(res, scope, it.Type); err != nil {
				return err
			}
		case *ast.RecordDecl, *ast.FlagsDecl, *ast.EnumDecl:
		case *ast.VariantDecl:
			for _, c := range it.Cases {
				if c.Type != nil {
					if err := validateType(res, scope, c.Type); err != nil {
						return err
					}
				}
			}
		case *ast.ResourceDecl:
			if err := analyzeResource(res, scope, it); err != nil {
				return err
			}
		case *ast.FuncDecl:
			if err := validateFunc(res, scope, it); err != nil {
				return err
			}
		}
	}
	return nil
}

func analyzeWorld(res *Result, decl *ast.WorldDecl) error {
	scope := NewScope(res.Globals)
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.TypeDecl, *ast.RecordDecl, *ast.FlagsDecl, *ast.EnumDecl, *ast.VariantDecl, *ast.ResourceDecl:
			if err := scope.Define(Symbol{Name: extractTypeName(it), Kind: SymbolType, Node: it}); err != nil {
				return err
			}
		case *ast.FuncDecl:
			if err := scope.Define(Symbol{Name: it.Name.Text, Kind: SymbolFunction, Node: it}); err != nil {
				return err
			}
		case *ast.ImportDecl:
			if err := scope.Define(Symbol{Name: it.Name.Text, Kind: SymbolImport, Node: it}); err != nil {
				return err
			}
		case *ast.ExportDecl:
			if err := scope.Define(Symbol{Name: it.Name.Text, Kind: SymbolExport, Node: it}); err != nil {
				return err
			}
		}
	}
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.UseDecl:
		case *ast.TypeDecl:
			if err := validateType(res, scope, it.Type); err != nil {
				return err
			}
		case *ast.RecordDecl, *ast.FlagsDecl, *ast.EnumDecl:
		case *ast.VariantDecl:
			for _, c := range it.Cases {
				if c.Type != nil {
					if err := validateType(res, scope, c.Type); err != nil {
						return err
					}
				}
			}
		case *ast.ResourceDecl:
			if err := analyzeResource(res, scope, it); err != nil {
				return err
			}
		case *ast.FuncDecl:
			if err := validateFunc(res, scope, it); err != nil {
				return err
			}
		case *ast.ImportDecl:
			if err := validateExternType(res, scope, it.Extern); err != nil {
				return err
			}
		case *ast.ExportDecl:
			if err := validateExternType(res, scope, it.Extern); err != nil {
				return err
			}
		case *ast.IncludeDecl:
		}
	}
	return nil
}

func analyzeResource(res *Result, scope *Scope, decl *ast.ResourceDecl) error {
	resourceScope := NewScope(scope)
	if err := resourceScope.Define(Symbol{Name: decl.Name.Text, Kind: SymbolType, Node: decl}); err != nil {
		return err
	}
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.ConstructorDecl:
			for _, param := range it.Params {
				if err := validateType(res, resourceScope, param.Type); err != nil {
					return err
				}
			}
			if it.Result != nil {
				if err := validateType(res, resourceScope, it.Result); err != nil {
					return err
				}
			}
		case *ast.FuncDecl:
			if err := validateFunc(res, resourceScope, it); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateExternType(res *Result, scope *Scope, expr ast.ExternType) error {
	switch e := expr.(type) {
	case *ast.ExternPath:
		return nil
	case *ast.InlineInterface:
		tmp := &ast.InterfaceDecl{Items: e.Items}
		return analyzeInterface(res, tmp)
	case *ast.FuncType:
		return validateFunc(res, scope, e.Func)
	default:
		return nil
	}
}

func validateFunc(res *Result, scope *Scope, fn *ast.FuncDecl) error {
	for _, param := range fn.Params {
		if err := validateType(res, scope, param.Type); err != nil {
			return err
		}
	}
	if fn.Result != nil {
		return validateType(res, scope, fn.Result)
	}
	return nil
}

func validateType(res *Result, scope *Scope, expr ast.TypeExpr) error {
	switch t := expr.(type) {
	case *ast.PrimitiveType:
		if info, ok := res.Builtins[t.Name]; !ok || info.Kind != BuiltinPrimitive {
			return fmt.Errorf("unknown primitive type %q", t.Name)
		}
	case *ast.NamedType:
		if t.Wildcard {
			return nil
		}
		if _, ok := scope.Lookup(t.Name.Text); ok {
			return nil
		}
		if _, ok := res.Builtins[t.Name.Text]; ok {
			return nil
		}
		return fmt.Errorf("unknown type %q", t.Name.Text)
	case *ast.GenericType:
		info, ok := res.Builtins[t.Name]
		if !ok {
			return fmt.Errorf("unknown generic type %q", t.Name)
		}
		if info.Kind != BuiltinGeneric {
			return fmt.Errorf("%q is not generic", t.Name)
		}
		if info.Arity >= 0 && len(t.Args) != info.Arity {
			return fmt.Errorf("generic type %q expects %d args, got %d", t.Name, info.Arity, len(t.Args))
		}
		for _, arg := range t.Args {
			if err := validateType(res, scope, arg); err != nil {
				return err
			}
		}
	case *ast.TupleType:
		for _, item := range t.Items {
			if err := validateType(res, scope, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractTypeName(node any) string {
	switch n := node.(type) {
	case *ast.TypeDecl:
		return n.Name.Text
	case *ast.RecordDecl:
		return n.Name.Text
	case *ast.FlagsDecl:
		return n.Name.Text
	case *ast.EnumDecl:
		return n.Name.Text
	case *ast.VariantDecl:
		return n.Name.Text
	case *ast.ResourceDecl:
		return n.Name.Text
	default:
		return ""
	}
}

func builtins() map[string]TypeInfo {
	return map[string]TypeInfo{
		"bool":   {Name: "bool", Kind: BuiltinPrimitive},
		"char":   {Name: "char", Kind: BuiltinPrimitive},
		"f32":    {Name: "f32", Kind: BuiltinPrimitive},
		"f64":    {Name: "f64", Kind: BuiltinPrimitive},
		"s8":     {Name: "s8", Kind: BuiltinPrimitive},
		"s16":    {Name: "s16", Kind: BuiltinPrimitive},
		"s32":    {Name: "s32", Kind: BuiltinPrimitive},
		"s64":    {Name: "s64", Kind: BuiltinPrimitive},
		"string": {Name: "string", Kind: BuiltinPrimitive},
		"u8":     {Name: "u8", Kind: BuiltinPrimitive},
		"u16":    {Name: "u16", Kind: BuiltinPrimitive},
		"u32":    {Name: "u32", Kind: BuiltinPrimitive},
		"u64":    {Name: "u64", Kind: BuiltinPrimitive},
		"borrow": {Name: "borrow", Kind: BuiltinGeneric, Arity: 1},
		"future": {Name: "future", Kind: BuiltinGeneric, Arity: 1},
		"list":   {Name: "list", Kind: BuiltinGeneric, Arity: 1},
		"map":    {Name: "map", Kind: BuiltinGeneric, Arity: 2},
		"option": {Name: "option", Kind: BuiltinGeneric, Arity: 1},
		"own":    {Name: "own", Kind: BuiltinGeneric, Arity: 1},
		"result": {Name: "result", Kind: BuiltinGeneric, Arity: -1},
		"stream": {Name: "stream", Kind: BuiltinGeneric, Arity: 1},
		"tuple":  {Name: "tuple", Kind: BuiltinGeneric, Arity: -1},
	}
}
