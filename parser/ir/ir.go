package ir

import (
	"fmt"

	"github.com/slavkiy/witgo/parser/ast"
)

type File struct {
	Package *Package
	Uses    []Use
	Decls   []Decl
}

type Package struct {
	Namespace string
	Name      string
	Version   string
}

type Use struct {
	From  Path
	Names []UseName
}

type UseName struct {
	Name  string
	Alias string
}

type Path struct {
	Package  *PackageRef
	Segments []string
}

type PackageRef struct {
	Namespace string
	Name      string
}

type Decl interface {
	decl()
}

type Interface struct {
	Name  string
	Items []InterfaceItem
}

func (*Interface) decl() {}

type World struct {
	Name  string
	Items []WorldItem
}

func (*World) decl() {}

type TypeDef struct {
	Name string
	Type Type
}

func (*TypeDef) decl()          {}
func (*TypeDef) interfaceItem() {}
func (*TypeDef) worldItem()     {}

type Record struct {
	Name   string
	Fields []Field
}

func (*Record) decl()          {}
func (*Record) interfaceItem() {}
func (*Record) worldItem()     {}

type Field struct {
	Name string
	Type Type
}

type Flags struct {
	Name  string
	Flags []string
}

func (*Flags) decl()          {}
func (*Flags) interfaceItem() {}
func (*Flags) worldItem()     {}

type Enum struct {
	Name  string
	Cases []string
}

func (*Enum) decl()          {}
func (*Enum) interfaceItem() {}
func (*Enum) worldItem()     {}

type Variant struct {
	Name  string
	Cases []VariantCase
}

func (*Variant) decl()          {}
func (*Variant) interfaceItem() {}
func (*Variant) worldItem()     {}

type VariantCase struct {
	Name string
	Type Type
}

type Resource struct {
	Name  string
	Items []ResourceItem
}

func (*Resource) decl()          {}
func (*Resource) interfaceItem() {}
func (*Resource) worldItem()     {}

type InterfaceItem interface {
	interfaceItem()
}

type WorldItem interface {
	worldItem()
}

type ResourceItem interface {
	resourceItem()
}

type Func struct {
	Name   string
	Params []Param
	Result Type
	Async  bool
	Static bool
}

func (*Func) interfaceItem() {}
func (*Func) worldItem()     {}
func (*Func) resourceItem()  {}

type Param struct {
	Name string
	Type Type
}

type Constructor struct {
	Params []Param
	Result Type
}

func (*Constructor) resourceItem() {}

type Import struct {
	ExternalID string
	Name       string
	Extern     ExternType
}

func (*Import) worldItem() {}

type Export struct {
	ExternalID string
	Name       string
	Extern     ExternType
}

func (*Export) worldItem() {}

type Include struct {
	Path    Path
	Renames []IncludeName
}

func (*Include) worldItem() {}

type IncludeName struct {
	From string
	To   string
}

type ExternType interface {
	externType()
}

type ExternPath struct {
	Path Path
}

func (*ExternPath) externType() {}

type InlineInterface struct {
	Items []InterfaceItem
}

func (*InlineInterface) externType() {}

type FuncType struct {
	Func *Func
}

func (*FuncType) externType() {}

type Type interface {
	typeNode()
}

type PrimitiveType struct {
	Name string
}

func (*PrimitiveType) typeNode() {}

type NamedType struct {
	Name     string
	Wildcard bool
}

func (*NamedType) typeNode() {}

type GenericType struct {
	Name string
	Args []Type
}

func (*GenericType) typeNode() {}

type TupleType struct {
	Items []Type
}

func (*TupleType) typeNode() {}

func Lower(file *ast.File) (*File, error) {
	if file == nil {
		return nil, nil
	}

	out := &File{}
	if file.Package != nil {
		out.Package = &Package{
			Namespace: file.Package.Namespace,
			Name:      file.Package.Name,
			Version:   file.Package.Version,
		}
	}

	out.Uses = make([]Use, 0, len(file.Uses))
	for _, useDecl := range file.Uses {
		useNode, err := lowerUseDecl(useDecl)
		if err != nil {
			return nil, err
		}
		out.Uses = append(out.Uses, *useNode)
	}

	out.Decls = make([]Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		lowered, err := lowerDecl(decl)
		if err != nil {
			return nil, err
		}
		out.Decls = append(out.Decls, lowered)
	}

	return out, nil
}

func lowerDecl(decl ast.Decl) (Decl, error) {
	switch d := decl.(type) {
	case *ast.InterfaceDecl:
		items := make([]InterfaceItem, 0, len(d.Items))
		for _, item := range d.Items {
			lowered, err := lowerInterfaceItem(item)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		}
		return &Interface{Name: d.Name.Text, Items: items}, nil
	case *ast.WorldDecl:
		items := make([]WorldItem, 0, len(d.Items))
		for _, item := range d.Items {
			lowered, err := lowerWorldItem(item)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		}
		return &World{Name: d.Name.Text, Items: items}, nil
	case *ast.TypeDecl:
		return lowerTypeDecl(d)
	case *ast.RecordDecl:
		return lowerRecordDecl(d)
	case *ast.FlagsDecl:
		return lowerFlagsDecl(d), nil
	case *ast.EnumDecl:
		return lowerEnumDecl(d), nil
	case *ast.VariantDecl:
		return lowerVariantDecl(d)
	case *ast.ResourceDecl:
		return lowerResourceDecl(d)
	default:
		return nil, fmt.Errorf("unsupported decl type %T", decl)
	}
}

func lowerInterfaceItem(item ast.InterfaceItem) (InterfaceItem, error) {
	switch it := item.(type) {
	case *ast.UseDecl:
		return lowerUseDecl(it)
	case *ast.TypeDecl:
		return lowerTypeDecl(it)
	case *ast.RecordDecl:
		return lowerRecordDecl(it)
	case *ast.FlagsDecl:
		return lowerFlagsDecl(it), nil
	case *ast.EnumDecl:
		return lowerEnumDecl(it), nil
	case *ast.VariantDecl:
		return lowerVariantDecl(it)
	case *ast.ResourceDecl:
		return lowerResourceDecl(it)
	case *ast.FuncDecl:
		return lowerFuncDecl(it)
	default:
		return nil, fmt.Errorf("unsupported interface item %T", item)
	}
}

func lowerWorldItem(item ast.WorldItem) (WorldItem, error) {
	switch it := item.(type) {
	case *ast.UseDecl:
		return lowerUseDecl(it)
	case *ast.TypeDecl:
		return lowerTypeDecl(it)
	case *ast.RecordDecl:
		return lowerRecordDecl(it)
	case *ast.FlagsDecl:
		return lowerFlagsDecl(it), nil
	case *ast.EnumDecl:
		return lowerEnumDecl(it), nil
	case *ast.VariantDecl:
		return lowerVariantDecl(it)
	case *ast.ResourceDecl:
		return lowerResourceDecl(it)
	case *ast.FuncDecl:
		return lowerFuncDecl(it)
	case *ast.ImportDecl:
		return lowerImportDecl(it)
	case *ast.ExportDecl:
		return lowerExportDecl(it)
	case *ast.IncludeDecl:
		return lowerIncludeDecl(it)
	default:
		return nil, fmt.Errorf("unsupported world item %T", item)
	}
}

func lowerUseDecl(decl *ast.UseDecl) (*Use, error) {
	path, err := lowerPath(decl.From)
	if err != nil {
		return nil, err
	}
	out := &Use{
		From:  *path,
		Names: make([]UseName, 0, len(decl.Names)),
	}
	for _, name := range decl.Names {
		var alias string
		if name.Alias != nil {
			alias = name.Alias.Text
		}
		out.Names = append(out.Names, UseName{Name: name.Name.Text, Alias: alias})
	}
	return out, nil
}

func (*Use) interfaceItem() {}
func (*Use) worldItem()     {}

func lowerTypeDecl(decl *ast.TypeDecl) (*TypeDef, error) {
	typ, err := lowerType(decl.Type)
	if err != nil {
		return nil, err
	}
	return &TypeDef{Name: decl.Name.Text, Type: typ}, nil
}

func lowerRecordDecl(decl *ast.RecordDecl) (*Record, error) {
	fields := make([]Field, 0, len(decl.Fields))
	for _, field := range decl.Fields {
		typ, err := lowerType(field.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, Field{Name: field.Name.Text, Type: typ})
	}
	return &Record{Name: decl.Name.Text, Fields: fields}, nil
}

func lowerFlagsDecl(decl *ast.FlagsDecl) *Flags {
	flags := make([]string, 0, len(decl.Flags))
	for _, flag := range decl.Flags {
		flags = append(flags, flag.Text)
	}
	return &Flags{Name: decl.Name.Text, Flags: flags}
}

func lowerEnumDecl(decl *ast.EnumDecl) *Enum {
	cases := make([]string, 0, len(decl.Cases))
	for _, c := range decl.Cases {
		cases = append(cases, c.Text)
	}
	return &Enum{Name: decl.Name.Text, Cases: cases}
}

func lowerVariantDecl(decl *ast.VariantDecl) (*Variant, error) {
	cases := make([]VariantCase, 0, len(decl.Cases))
	for _, c := range decl.Cases {
		typ, err := lowerType(c.Type)
		if err != nil {
			return nil, err
		}
		cases = append(cases, VariantCase{Name: c.Name.Text, Type: typ})
	}
	return &Variant{Name: decl.Name.Text, Cases: cases}, nil
}

func lowerResourceDecl(decl *ast.ResourceDecl) (*Resource, error) {
	items := make([]ResourceItem, 0, len(decl.Items))
	for _, item := range decl.Items {
		switch it := item.(type) {
		case *ast.ConstructorDecl:
			lowered, err := lowerConstructorDecl(it)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		case *ast.FuncDecl:
			lowered, err := lowerFuncDecl(it)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		default:
			return nil, fmt.Errorf("unsupported resource item %T", item)
		}
	}
	return &Resource{Name: decl.Name.Text, Items: items}, nil
}

func lowerConstructorDecl(decl *ast.ConstructorDecl) (*Constructor, error) {
	params, err := lowerParams(decl.Params)
	if err != nil {
		return nil, err
	}
	result, err := lowerType(decl.Result)
	if err != nil {
		return nil, err
	}
	return &Constructor{Params: params, Result: result}, nil
}

func lowerFuncDecl(decl *ast.FuncDecl) (*Func, error) {
	params, err := lowerParams(decl.Params)
	if err != nil {
		return nil, err
	}
	result, err := lowerType(decl.Result)
	if err != nil {
		return nil, err
	}
	return &Func{
		Name:   decl.Name.Text,
		Params: params,
		Result: result,
		Async:  decl.Async,
		Static: decl.Static,
	}, nil
}

func lowerParams(params []ast.Param) ([]Param, error) {
	out := make([]Param, 0, len(params))
	for _, param := range params {
		typ, err := lowerType(param.Type)
		if err != nil {
			return nil, err
		}
		out = append(out, Param{Name: param.Name, Type: typ})
	}
	return out, nil
}

func lowerImportDecl(decl *ast.ImportDecl) (*Import, error) {
	extern, err := lowerExternType(decl.Extern)
	if err != nil {
		return nil, err
	}
	return &Import{
		ExternalID: lowerOptionalName(decl.ExternalID),
		Name:       decl.Name.Text,
		Extern:     extern,
	}, nil
}

func lowerExportDecl(decl *ast.ExportDecl) (*Export, error) {
	extern, err := lowerExternType(decl.Extern)
	if err != nil {
		return nil, err
	}
	return &Export{
		ExternalID: lowerOptionalName(decl.ExternalID),
		Name:       decl.Name.Text,
		Extern:     extern,
	}, nil
}

func lowerIncludeDecl(decl *ast.IncludeDecl) (*Include, error) {
	path, err := lowerPath(decl.Path)
	if err != nil {
		return nil, err
	}
	renames := make([]IncludeName, 0, len(decl.Renames))
	for _, rename := range decl.Renames {
		renames = append(renames, IncludeName{
			From: rename.From.Text,
			To:   rename.To.Text,
		})
	}
	return &Include{Path: *path, Renames: renames}, nil
}

func lowerExternType(extern ast.ExternType) (ExternType, error) {
	switch e := extern.(type) {
	case *ast.ExternPath:
		path, err := lowerPath(e.Path)
		if err != nil {
			return nil, err
		}
		return &ExternPath{Path: *path}, nil
	case *ast.InlineInterface:
		items := make([]InterfaceItem, 0, len(e.Items))
		for _, item := range e.Items {
			lowered, err := lowerInterfaceItem(item)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		}
		return &InlineInterface{Items: items}, nil
	case *ast.FuncType:
		fn, err := lowerFuncDecl(e.Func)
		if err != nil {
			return nil, err
		}
		return &FuncType{Func: fn}, nil
	default:
		return nil, fmt.Errorf("unsupported extern type %T", extern)
	}
}

func lowerType(expr ast.TypeExpr) (Type, error) {
	if expr == nil {
		return nil, nil
	}
	switch t := expr.(type) {
	case *ast.PrimitiveType:
		return &PrimitiveType{Name: t.Name}, nil
	case *ast.NamedType:
		return &NamedType{Name: t.Name.Text, Wildcard: t.Wildcard}, nil
	case *ast.GenericType:
		args := make([]Type, 0, len(t.Args))
		for _, arg := range t.Args {
			lowered, err := lowerType(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, lowered)
		}
		return &GenericType{Name: t.Name, Args: args}, nil
	case *ast.TupleType:
		items := make([]Type, 0, len(t.Items))
		for _, item := range t.Items {
			lowered, err := lowerType(item)
			if err != nil {
				return nil, err
			}
			items = append(items, lowered)
		}
		return &TupleType{Items: items}, nil
	default:
		return nil, fmt.Errorf("unsupported type expr %T", expr)
	}
}

func lowerPath(path ast.Path) (*Path, error) {
	out := &Path{
		Segments: make([]string, 0, len(path.Segments)),
	}
	if path.Package != nil {
		out.Package = &PackageRef{
			Namespace: path.Package.Namespace,
			Name:      path.Package.Name,
		}
	}
	for _, segment := range path.Segments {
		out.Segments = append(out.Segments, segment.Text)
	}
	return out, nil
}

func lowerOptionalName(name *ast.Name) string {
	if name == nil {
		return ""
	}
	return name.Text
}
