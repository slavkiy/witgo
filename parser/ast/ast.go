package ast

import "github.com/slavkiy/witgo/parser/token"

type Node interface {
	Span() Span
	node()
}

type Decl interface {
	Node
	decl()
}

type InterfaceItem interface {
	Node
	interfaceItem()
}

type WorldItem interface {
	Node
	worldItem()
}

type TypeDef interface {
	Node
	typeDef()
}

type TypeExpr interface {
	Node
	typeExpr()
}

type Value interface {
	Node
	value()
}

type Span struct {
	Start token.Position
	End   token.Position
}

type File struct {
	Package *PackageDecl
	Uses    []*UseDecl
	Decls   []Decl
}

func (n *File) Span() Span {
	if n.Package != nil && len(n.Decls) == 0 {
		return n.Package.Span()
	}
	if n.Package != nil && len(n.Decls) > 0 {
		return Span{Start: n.Package.Span().Start, End: n.Decls[len(n.Decls)-1].Span().End}
	}
	if len(n.Decls) == 0 {
		return Span{}
	}
	return Span{Start: n.Decls[0].Span().Start, End: n.Decls[len(n.Decls)-1].Span().End}
}

func (*File) node() {}

type Attribute struct {
	Name      string
	Arguments []AttributeArg
	SpanRange Span
}

func (n *Attribute) Span() Span { return n.SpanRange }
func (*Attribute) node()        {}

type AttributeArg struct {
	Name  string
	Value Value
}

type StringValue struct {
	Value     string
	Raw       string
	SpanRange Span
}

func (n *StringValue) Span() Span { return n.SpanRange }
func (*StringValue) node()        {}
func (*StringValue) value()       {}

type IntValue struct {
	Value     string
	SpanRange Span
}

func (n *IntValue) Span() Span { return n.SpanRange }
func (*IntValue) node()        {}
func (*IntValue) value()       {}

type Name struct {
	Text      string
	Explicit  bool
	SpanRange Span
}

func (n Name) Span() Span { return n.SpanRange }

type Path struct {
	Package   *PackagePath
	Segments  []Name
	SpanRange Span
}

func (n *Path) Span() Span { return n.SpanRange }
func (*Path) node()        {}

type PackagePath struct {
	Namespace string
	Name      string
	SpanRange Span
}

func (n *PackagePath) Span() Span { return n.SpanRange }
func (*PackagePath) node()        {}

type PackageDecl struct {
	Namespace string
	Name      string
	Version   string
	Attrs     []*Attribute
	SpanRange Span
}

func (n *PackageDecl) Span() Span { return n.SpanRange }
func (*PackageDecl) node()        {}
func (*PackageDecl) decl()        {}

type InterfaceDecl struct {
	Attrs     []*Attribute
	Name      Name
	Items     []InterfaceItem
	SpanRange Span
}

func (n *InterfaceDecl) Span() Span { return n.SpanRange }
func (*InterfaceDecl) node()        {}
func (*InterfaceDecl) decl()        {}

type WorldDecl struct {
	Attrs     []*Attribute
	Name      Name
	Items     []WorldItem
	SpanRange Span
}

func (n *WorldDecl) Span() Span { return n.SpanRange }
func (*WorldDecl) node()        {}
func (*WorldDecl) decl()        {}

type UseDecl struct {
	From      Path
	Names     []UseName
	SpanRange Span
}

func (n *UseDecl) Span() Span   { return n.SpanRange }
func (*UseDecl) node()          {}
func (*UseDecl) decl()          {}
func (*UseDecl) interfaceItem() {}
func (*UseDecl) worldItem()     {}

type UseName struct {
	Name  Name
	Alias *Name
}

type TypeDecl struct {
	Attrs     []*Attribute
	Name      Name
	Type      TypeExpr
	SpanRange Span
}

func (n *TypeDecl) Span() Span   { return n.SpanRange }
func (*TypeDecl) node()          {}
func (*TypeDecl) decl()          {}
func (*TypeDecl) interfaceItem() {}
func (*TypeDecl) worldItem()     {}
func (*TypeDecl) typeDef()       {}

type RecordDecl struct {
	Attrs     []*Attribute
	Name      Name
	Fields    []Field
	SpanRange Span
}

func (n *RecordDecl) Span() Span   { return n.SpanRange }
func (*RecordDecl) node()          {}
func (*RecordDecl) decl()          {}
func (*RecordDecl) interfaceItem() {}
func (*RecordDecl) worldItem()     {}
func (*RecordDecl) typeDef()       {}

type Field struct {
	Name      Name
	Type      TypeExpr
	SpanRange Span
}

type FlagsDecl struct {
	Attrs     []*Attribute
	Name      Name
	Flags     []Name
	SpanRange Span
}

func (n *FlagsDecl) Span() Span   { return n.SpanRange }
func (*FlagsDecl) node()          {}
func (*FlagsDecl) decl()          {}
func (*FlagsDecl) interfaceItem() {}
func (*FlagsDecl) worldItem()     {}
func (*FlagsDecl) typeDef()       {}

type EnumDecl struct {
	Attrs     []*Attribute
	Name      Name
	Cases     []Name
	SpanRange Span
}

func (n *EnumDecl) Span() Span   { return n.SpanRange }
func (*EnumDecl) node()          {}
func (*EnumDecl) decl()          {}
func (*EnumDecl) interfaceItem() {}
func (*EnumDecl) worldItem()     {}
func (*EnumDecl) typeDef()       {}

type VariantDecl struct {
	Attrs     []*Attribute
	Name      Name
	Cases     []VariantCase
	SpanRange Span
}

func (n *VariantDecl) Span() Span   { return n.SpanRange }
func (*VariantDecl) node()          {}
func (*VariantDecl) decl()          {}
func (*VariantDecl) interfaceItem() {}
func (*VariantDecl) worldItem()     {}
func (*VariantDecl) typeDef()       {}

type VariantCase struct {
	Name      Name
	Type      TypeExpr
	SpanRange Span
}

type ResourceDecl struct {
	Attrs     []*Attribute
	Name      Name
	Items     []ResourceItem
	SpanRange Span
}

func (n *ResourceDecl) Span() Span   { return n.SpanRange }
func (*ResourceDecl) node()          {}
func (*ResourceDecl) decl()          {}
func (*ResourceDecl) interfaceItem() {}
func (*ResourceDecl) worldItem()     {}
func (*ResourceDecl) typeDef()       {}

type ResourceItem interface {
	Node
	resourceItem()
}

type ConstructorDecl struct {
	Attrs     []*Attribute
	Params    []Param
	Result    TypeExpr
	SpanRange Span
}

func (n *ConstructorDecl) Span() Span  { return n.SpanRange }
func (*ConstructorDecl) node()         {}
func (*ConstructorDecl) resourceItem() {}

type FuncDecl struct {
	Attrs     []*Attribute
	Name      Name
	Params    []Param
	Result    TypeExpr
	Async     bool
	Static    bool
	SpanRange Span
}

func (n *FuncDecl) Span() Span   { return n.SpanRange }
func (*FuncDecl) node()          {}
func (*FuncDecl) interfaceItem() {}
func (*FuncDecl) worldItem()     {}
func (*FuncDecl) resourceItem()  {}

type Param struct {
	Name string
	Type TypeExpr
	Span Span
}

type ImportDecl struct {
	Attrs      []*Attribute
	ExternalID *Name
	Name       Name
	Extern     ExternType
	SpanRange  Span
}

func (n *ImportDecl) Span() Span { return n.SpanRange }
func (*ImportDecl) node()        {}
func (*ImportDecl) worldItem()   {}

type ExportDecl struct {
	Attrs      []*Attribute
	ExternalID *Name
	Name       Name
	Extern     ExternType
	SpanRange  Span
}

func (n *ExportDecl) Span() Span { return n.SpanRange }
func (*ExportDecl) node()        {}
func (*ExportDecl) worldItem()   {}

type IncludeDecl struct {
	Attrs     []*Attribute
	Path      Path
	Renames   []IncludeName
	SpanRange Span
}

func (n *IncludeDecl) Span() Span { return n.SpanRange }
func (*IncludeDecl) node()        {}
func (*IncludeDecl) worldItem()   {}

type IncludeName struct {
	From Name
	To   Name
}

type ExternType interface {
	Node
	externType()
}

type ExternPath struct {
	Path      Path
	SpanRange Span
}

func (n *ExternPath) Span() Span { return n.SpanRange }
func (*ExternPath) node()        {}
func (*ExternPath) externType()  {}

type InlineInterface struct {
	Items     []InterfaceItem
	SpanRange Span
}

func (n *InlineInterface) Span() Span { return n.SpanRange }
func (*InlineInterface) node()        {}
func (*InlineInterface) externType()  {}

type FuncType struct {
	Func      *FuncDecl
	SpanRange Span
}

func (n *FuncType) Span() Span { return n.SpanRange }
func (*FuncType) node()        {}
func (*FuncType) externType()  {}

type PrimitiveType struct {
	Name      string
	SpanRange Span
}

func (n *PrimitiveType) Span() Span { return n.SpanRange }
func (*PrimitiveType) node()        {}
func (*PrimitiveType) typeExpr()    {}

type NamedType struct {
	Name      Name
	Wildcard  bool
	SpanRange Span
}

func (n *NamedType) Span() Span { return n.SpanRange }
func (*NamedType) node()        {}
func (*NamedType) typeExpr()    {}

type GenericType struct {
	Name      string
	Args      []TypeExpr
	SpanRange Span
}

func (n *GenericType) Span() Span { return n.SpanRange }
func (*GenericType) node()        {}
func (*GenericType) typeExpr()    {}

type TupleType struct {
	Items     []TypeExpr
	SpanRange Span
}

func (n *TupleType) Span() Span { return n.SpanRange }
func (*TupleType) node()        {}
func (*TupleType) typeExpr()    {}
