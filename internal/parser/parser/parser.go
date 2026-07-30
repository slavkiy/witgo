package parser

import (
	"fmt"

	"github.com/slavkiy/witgo/internal/parser/ast"
	"github.com/slavkiy/witgo/internal/parser/lexer"
	"github.com/slavkiy/witgo/internal/parser/token"
)

type Parser struct {
	src    string
	tokens []token.Token
	pos    int
}

func Parse(src string) (_ *ast.File, err error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	p := &Parser{src: src, tokens: toks}
	defer func() {
		if recovered := recover(); recovered != nil {
			if errValue, ok := recovered.(error); ok {
				err = errValue
				return
			}
			err = fmt.Errorf("parser: %v", recovered)
		}
	}()
	return p.parseFile()
}

func (p *Parser) parseFile() (*ast.File, error) {
	file := &ast.File{}
	attrs, err := p.parseAttributes()
	if err != nil {
		return nil, err
	}
	if p.match(token.TokenKeywordPackage) {
		file.Package, err = p.parsePackageDecl(attrs)
		if err != nil {
			return nil, err
		}
	}
	for !p.match(token.TokenEOF) {
		attrs, err = p.parseAttributes()
		if err != nil {
			return nil, err
		}
		if p.match(token.TokenKeywordUse) {
			useDecl, err := p.parseUseDecl()
			if err != nil {
				return nil, err
			}
			file.Uses = append(file.Uses, useDecl)
			continue
		}
		decl, err := p.parseDecl(attrs)
		if err != nil {
			return nil, err
		}
		file.Decls = append(file.Decls, decl)
	}
	return file, nil
}

func (p *Parser) parseDecl(attrs []*ast.Attribute) (ast.Decl, error) {
	switch p.peek().Type {
	case token.TokenKeywordUse:
		return p.parseUseDecl()
	case token.TokenKeywordInterface:
		return p.parseInterfaceDecl(attrs)
	case token.TokenKeywordWorld:
		return p.parseWorldDecl(attrs)
	case token.TokenKeywordType:
		return p.parseTypeDecl(attrs)
	case token.TokenKeywordRecord:
		return p.parseRecordDecl(attrs)
	case token.TokenKeywordFlags:
		return p.parseFlagsDecl(attrs)
	case token.TokenKeywordEnum:
		return p.parseEnumDecl(attrs)
	case token.TokenKeywordVariant:
		return p.parseVariantDecl(attrs)
	case token.TokenKeywordResource:
		return p.parseResourceDecl(attrs)
	default:
		return nil, p.errorf(p.peek(), "expected declaration")
	}
}

func (p *Parser) parsePackageDecl(attrs []*ast.Attribute) (*ast.PackageDecl, error) {
	start := p.expect(token.TokenKeywordPackage)
	ns, err := p.expectName(false)
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenColon)
	name, err := p.expectName(false)
	if err != nil {
		return nil, err
	}
	version := ""
	if p.match(token.TokenAt) {
		p.next()
		version, err = p.parseVersion()
		if err != nil {
			return nil, err
		}
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.PackageDecl{Namespace: ns.Text, Name: name.Text, Version: version, Attrs: attrs, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseInterfaceDecl(attrs []*ast.Attribute) (*ast.InterfaceDecl, error) {
	start := p.expect(token.TokenKeywordInterface)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	var items []ast.InterfaceItem
	for !p.match(token.TokenRBrace) {
		itemAttrs, err := p.parseAttributes()
		if err != nil {
			return nil, err
		}
		item, err := p.parseInterfaceItem(itemAttrs)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	end := p.expect(token.TokenRBrace)
	return &ast.InterfaceDecl{Attrs: attrs, Name: name, Items: items, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseWorldDecl(attrs []*ast.Attribute) (*ast.WorldDecl, error) {
	start := p.expect(token.TokenKeywordWorld)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	var items []ast.WorldItem
	for !p.match(token.TokenRBrace) {
		itemAttrs, err := p.parseAttributes()
		if err != nil {
			return nil, err
		}
		item, err := p.parseWorldItem(itemAttrs)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	end := p.expect(token.TokenRBrace)
	return &ast.WorldDecl{Attrs: attrs, Name: name, Items: items, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseInterfaceItem(attrs []*ast.Attribute) (ast.InterfaceItem, error) {
	switch p.peek().Type {
	case token.TokenKeywordUse:
		return p.parseUseDecl()
	case token.TokenKeywordType:
		return p.parseTypeDecl(attrs)
	case token.TokenKeywordRecord:
		return p.parseRecordDecl(attrs)
	case token.TokenKeywordFlags:
		return p.parseFlagsDecl(attrs)
	case token.TokenKeywordEnum:
		return p.parseEnumDecl(attrs)
	case token.TokenKeywordVariant:
		return p.parseVariantDecl(attrs)
	case token.TokenKeywordResource:
		return p.parseResourceDecl(attrs)
	case token.TokenIdentifier, token.TokenExplicitID:
		return p.parseFuncDecl(attrs, false)
	default:
		return nil, p.errorf(p.peek(), "expected interface item")
	}
}

func (p *Parser) parseWorldItem(attrs []*ast.Attribute) (ast.WorldItem, error) {
	switch p.peek().Type {
	case token.TokenKeywordUse:
		return p.parseUseDecl()
	case token.TokenKeywordType:
		return p.parseTypeDecl(attrs)
	case token.TokenKeywordRecord:
		return p.parseRecordDecl(attrs)
	case token.TokenKeywordFlags:
		return p.parseFlagsDecl(attrs)
	case token.TokenKeywordEnum:
		return p.parseEnumDecl(attrs)
	case token.TokenKeywordVariant:
		return p.parseVariantDecl(attrs)
	case token.TokenKeywordResource:
		return p.parseResourceDecl(attrs)
	case token.TokenKeywordImport:
		return p.parseImportDecl(attrs)
	case token.TokenKeywordExport:
		return p.parseExportDecl(attrs)
	case token.TokenKeywordInclude:
		return p.parseIncludeDecl(attrs)
	case token.TokenIdentifier, token.TokenExplicitID:
		return p.parseFuncDecl(attrs, false)
	default:
		return nil, p.errorf(p.peek(), "expected world item")
	}
}

func (p *Parser) parseUseDecl() (*ast.UseDecl, error) {
	start := p.expect(token.TokenKeywordUse)
	path, err := p.parseUsePath()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenDot)
	p.expect(token.TokenLBrace)
	var names []ast.UseName
	for {
		name, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		item := ast.UseName{Name: name}
		if p.match(token.TokenKeywordAs) {
			p.next()
			alias, err := p.expectAnyName()
			if err != nil {
				return nil, err
			}
			item.Alias = &alias
		}
		names = append(names, item)
		if !p.match(token.TokenComma) {
			break
		}
		p.next()
	}
	p.expect(token.TokenRBrace)
	end := p.expect(token.TokenSemicolon)
	return &ast.UseDecl{From: *path, Names: names, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseTypeDecl(attrs []*ast.Attribute) (*ast.TypeDecl, error) {
	start := p.expect(token.TokenKeywordType)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenEquals)
	typ, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.TypeDecl{Attrs: attrs, Name: name, Type: typ, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseRecordDecl(attrs []*ast.Attribute) (*ast.RecordDecl, error) {
	start := p.expect(token.TokenKeywordRecord)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	fields, end, err := p.parseFields()
	if err != nil {
		return nil, err
	}
	return &ast.RecordDecl{Attrs: attrs, Name: name, Fields: fields, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseFields() ([]ast.Field, token.Token, error) {
	var fields []ast.Field
	for !p.match(token.TokenRBrace) {
		name, err := p.expectAnyName()
		if err != nil {
			return nil, token.Token{}, err
		}
		colon := p.expect(token.TokenColon)
		typ, err := p.parseTypeExpr()
		if err != nil {
			return nil, token.Token{}, err
		}
		fields = append(fields, ast.Field{Name: name, Type: typ, SpanRange: ast.Span{Start: name.SpanRange.Start, End: typ.Span().End}})
		if p.match(token.TokenComma) {
			p.next()
		} else if !p.match(token.TokenRBrace) {
			return nil, token.Token{}, p.errorf(colon, "expected ',' or '}'")
		}
	}
	end := p.expect(token.TokenRBrace)
	return fields, end, nil
}

func (p *Parser) parseFlagsDecl(attrs []*ast.Attribute) (*ast.FlagsDecl, error) {
	start := p.expect(token.TokenKeywordFlags)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	var flags []ast.Name
	for !p.match(token.TokenRBrace) {
		flag, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		flags = append(flags, flag)
		if p.match(token.TokenComma) {
			p.next()
		}
	}
	end := p.expect(token.TokenRBrace)
	return &ast.FlagsDecl{Attrs: attrs, Name: name, Flags: flags, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseEnumDecl(attrs []*ast.Attribute) (*ast.EnumDecl, error) {
	start := p.expect(token.TokenKeywordEnum)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	var cases []ast.Name
	for !p.match(token.TokenRBrace) {
		c, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
		if p.match(token.TokenComma) {
			p.next()
		}
	}
	end := p.expect(token.TokenRBrace)
	return &ast.EnumDecl{Attrs: attrs, Name: name, Cases: cases, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseVariantDecl(attrs []*ast.Attribute) (*ast.VariantDecl, error) {
	start := p.expect(token.TokenKeywordVariant)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenLBrace)
	var cases []ast.VariantCase
	for !p.match(token.TokenRBrace) {
		cname, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		item := ast.VariantCase{Name: cname, SpanRange: cname.SpanRange}
		if p.match(token.TokenLParen) {
			p.next()
			typ, err := p.parseTypeExpr()
			if err != nil {
				return nil, err
			}
			p.expect(token.TokenRParen)
			item.Type = typ
			item.SpanRange.End = typ.Span().End
		}
		cases = append(cases, item)
		if p.match(token.TokenComma) {
			p.next()
		}
	}
	end := p.expect(token.TokenRBrace)
	return &ast.VariantDecl{Attrs: attrs, Name: name, Cases: cases, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseResourceDecl(attrs []*ast.Attribute) (*ast.ResourceDecl, error) {
	start := p.expect(token.TokenKeywordResource)
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	if p.match(token.TokenSemicolon) {
		end := p.next()
		return &ast.ResourceDecl{Attrs: attrs, Name: name, SpanRange: span(start, end)}, nil
	}
	p.expect(token.TokenLBrace)
	var items []ast.ResourceItem
	for !p.match(token.TokenRBrace) {
		itemAttrs, err := p.parseAttributes()
		if err != nil {
			return nil, err
		}
		item, err := p.parseResourceItem(itemAttrs)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	end := p.expect(token.TokenRBrace)
	return &ast.ResourceDecl{Attrs: attrs, Name: name, Items: items, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseResourceItem(attrs []*ast.Attribute) (ast.ResourceItem, error) {
	switch p.peek().Type {
	case token.TokenKeywordConstructor:
		return p.parseConstructorDecl(attrs)
	case token.TokenIdentifier, token.TokenExplicitID:
		if p.peekAt(1).Type == token.TokenColon && p.peekAt(2).Type == token.TokenKeywordStatic {
			return p.parseStaticFuncDecl(attrs)
		}
		return p.parseFuncDecl(attrs, false)
	default:
		return nil, p.errorf(p.peek(), "expected resource item")
	}
}

func (p *Parser) parseConstructorDecl(attrs []*ast.Attribute) (*ast.ConstructorDecl, error) {
	start := p.expect(token.TokenKeywordConstructor)
	p.expect(token.TokenLParen)
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenRParen)
	var result ast.TypeExpr
	if p.match(token.TokenArrow) {
		p.next()
		result, err = p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.ConstructorDecl{Attrs: attrs, Params: params, Result: result, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseStaticFuncDecl(attrs []*ast.Attribute) (*ast.FuncDecl, error) {
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	start := name.SpanRange.Start
	p.expect(token.TokenColon)
	p.expect(token.TokenKeywordStatic)
	async := false
	if p.match(token.TokenKeywordAsync) {
		p.next()
		async = true
	}
	p.expect(token.TokenKeywordFunc)
	p.expect(token.TokenLParen)
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenRParen)
	var result ast.TypeExpr
	if p.match(token.TokenArrow) {
		p.next()
		result, err = p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.FuncDecl{Attrs: attrs, Name: name, Params: params, Result: result, Async: async, Static: true, SpanRange: ast.Span{Start: start, End: end.End}}, nil
}

func (p *Parser) parseFuncDecl(attrs []*ast.Attribute, static bool) (*ast.FuncDecl, error) {
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	start := name.SpanRange.Start
	p.expect(token.TokenColon)
	async := false
	if p.match(token.TokenKeywordAsync) {
		p.next()
		async = true
	}
	p.expect(token.TokenKeywordFunc)
	p.expect(token.TokenLParen)
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenRParen)
	var result ast.TypeExpr
	if p.match(token.TokenArrow) {
		p.next()
		result, err = p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.FuncDecl{Attrs: attrs, Name: name, Params: params, Result: result, Async: async, Static: static, SpanRange: ast.Span{Start: start, End: end.End}}, nil
}

func (p *Parser) parseImportDecl(attrs []*ast.Attribute) (*ast.ImportDecl, error) {
	start := p.expect(token.TokenKeywordImport)
	return p.parseWorldExternDecl(start, attrs, true)
}

func (p *Parser) parseExportDecl(attrs []*ast.Attribute) (*ast.ExportDecl, error) {
	start := p.expect(token.TokenKeywordExport)
	decl, err := p.parseWorldExternDecl(start, attrs, false)
	if err != nil {
		return nil, err
	}
	return &ast.ExportDecl{Attrs: decl.Attrs, ExternalID: decl.ExternalID, Name: decl.Name, Extern: decl.Extern, SpanRange: decl.SpanRange}, nil
}

func (p *Parser) parseWorldExternDecl(start token.Token, attrs []*ast.Attribute, importDecl bool) (*ast.ImportDecl, error) {
	if p.isPathStart() && p.hasSemicolonBeforeColon() {
		path, err := p.parsePath()
		if err != nil {
			return nil, err
		}
		end := p.expect(token.TokenSemicolon)
		name := path.Segments[len(path.Segments)-1]
		return &ast.ImportDecl{Attrs: attrs, Name: name, Extern: &ast.ExternPath{Path: *path, SpanRange: path.SpanRange}, SpanRange: ast.Span{Start: start.Start, End: end.End}}, nil
	}

	var externalID *ast.Name
	if p.match(token.TokenExplicitID) && p.peekAt(1).Type == token.TokenIdentifier && p.peekAt(2).Type == token.TokenColon {
		name, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		externalID = &name
	}
	name, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	p.expect(token.TokenColon)
	extern, end, err := p.parseExternType()
	if err != nil {
		return nil, err
	}
	return &ast.ImportDecl{Attrs: attrs, ExternalID: externalID, Name: name, Extern: extern, SpanRange: ast.Span{Start: start.Start, End: end.End}}, nil
}

func (p *Parser) parseIncludeDecl(attrs []*ast.Attribute) (*ast.IncludeDecl, error) {
	start := p.expect(token.TokenKeywordInclude)
	path, err := p.parsePath()
	if err != nil {
		return nil, err
	}
	var renames []ast.IncludeName
	if p.match(token.TokenKeywordWith) {
		p.next()
		p.expect(token.TokenLBrace)
		for !p.match(token.TokenRBrace) {
			from, err := p.expectAnyName()
			if err != nil {
				return nil, err
			}
			p.expect(token.TokenKeywordAs)
			to, err := p.expectAnyName()
			if err != nil {
				return nil, err
			}
			renames = append(renames, ast.IncludeName{From: from, To: to})
			if p.match(token.TokenComma) {
				p.next()
			}
		}
		p.expect(token.TokenRBrace)
	}
	end := p.expect(token.TokenSemicolon)
	return &ast.IncludeDecl{Attrs: attrs, Path: *path, Renames: renames, SpanRange: span(start, end)}, nil
}

func (p *Parser) parseExternType() (ast.ExternType, token.Token, error) {
	switch p.peek().Type {
	case token.TokenKeywordInterface:
		start := p.next()
		p.expect(token.TokenLBrace)
		var items []ast.InterfaceItem
		for !p.match(token.TokenRBrace) {
			itemAttrs, err := p.parseAttributes()
			if err != nil {
				return nil, token.Token{}, err
			}
			item, err := p.parseInterfaceItem(itemAttrs)
			if err != nil {
				return nil, token.Token{}, err
			}
			items = append(items, item)
		}
		end := p.expect(token.TokenRBrace)
		return &ast.InlineInterface{Items: items, SpanRange: span(start, end)}, end, nil
	case token.TokenKeywordAsync, token.TokenKeywordFunc:
		start := p.peek()
		async := false
		if p.match(token.TokenKeywordAsync) {
			p.next()
			async = true
		}
		p.expect(token.TokenKeywordFunc)
		p.expect(token.TokenLParen)
		params, err := p.parseParams()
		if err != nil {
			return nil, token.Token{}, err
		}
		p.expect(token.TokenRParen)
		var result ast.TypeExpr
		if p.match(token.TokenArrow) {
			p.next()
			result, err = p.parseTypeExpr()
			if err != nil {
				return nil, token.Token{}, err
			}
		}
		end := p.expect(token.TokenSemicolon)
		fn := &ast.FuncDecl{Params: params, Result: result, Async: async, SpanRange: ast.Span{Start: start.Start, End: end.End}}
		return &ast.FuncType{Func: fn, SpanRange: fn.SpanRange}, end, nil
	default:
		path, err := p.parsePath()
		if err != nil {
			return nil, token.Token{}, err
		}
		end := p.expect(token.TokenSemicolon)
		return &ast.ExternPath{Path: *path, SpanRange: ast.Span{Start: path.SpanRange.Start, End: end.End}}, end, nil
	}
}

func (p *Parser) parseParams() ([]ast.Param, error) {
	var params []ast.Param
	if p.match(token.TokenRParen) {
		return nil, nil
	}
	for {
		nameTok := p.expect(token.TokenIdentifier)
		p.expect(token.TokenColon)
		typ, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		params = append(params, ast.Param{Name: nameTok.Slice(p.src), Type: typ, Span: ast.Span{Start: nameTok.Start, End: typ.Span().End}})
		if !p.match(token.TokenComma) {
			break
		}
		p.next()
	}
	return params, nil
}

func (p *Parser) parseTypeExpr() (ast.TypeExpr, error) {
	tok := p.peek()
	switch tok.Type {
	case token.TokenIdentifier, token.TokenExplicitID:
		name, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		return &ast.NamedType{Name: name, SpanRange: name.SpanRange}, nil
	case token.TokenKeywordBool, token.TokenKeywordChar, token.TokenKeywordF32, token.TokenKeywordF64,
		token.TokenKeywordS8, token.TokenKeywordS16, token.TokenKeywordS32, token.TokenKeywordS64,
		token.TokenKeywordString, token.TokenKeywordU8, token.TokenKeywordU16, token.TokenKeywordU32, token.TokenKeywordU64:
		p.next()
		return &ast.PrimitiveType{Name: tok.Slice(p.src), SpanRange: ast.Span{Start: tok.Start, End: tok.End}}, nil
	case token.TokenKeywordBorrow, token.TokenKeywordOwn, token.TokenKeywordList, token.TokenKeywordOption, token.TokenKeywordResult, token.TokenKeywordFuture, token.TokenKeywordStream:
		return p.parseGenericType()
	case token.TokenKeywordMap:
		return p.parseGenericType()
	case token.TokenKeywordTuple:
		return p.parseTupleType()
	case token.TokenUnderscore:
		tok := p.next()
		return &ast.NamedType{
			Name:      ast.Name{Text: tok.Slice(p.src), SpanRange: ast.Span{Start: tok.Start, End: tok.End}},
			Wildcard:  true,
			SpanRange: ast.Span{Start: tok.Start, End: tok.End},
		}, nil
	default:
		return nil, p.errorf(tok, "expected type")
	}
}

func (p *Parser) parseGenericType() (ast.TypeExpr, error) {
	tok := p.next()
	p.expect(token.TokenLAngle)
	var args []ast.TypeExpr
	if !p.match(token.TokenRAngle) {
		for {
			typ, err := p.parseTypeExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, typ)
			if !p.match(token.TokenComma) {
				break
			}
			p.next()
		}
	}
	end := p.expect(token.TokenRAngle)
	return &ast.GenericType{Name: tok.Slice(p.src), Args: args, SpanRange: span(tok, end)}, nil
}

func (p *Parser) parseTupleType() (ast.TypeExpr, error) {
	start := p.expect(token.TokenKeywordTuple)
	p.expect(token.TokenLAngle)
	var items []ast.TypeExpr
	if !p.match(token.TokenRAngle) {
		for {
			item, err := p.parseTypeExpr()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			if !p.match(token.TokenComma) {
				break
			}
			p.next()
		}
	}
	end := p.expect(token.TokenRAngle)
	return &ast.TupleType{Items: items, SpanRange: span(start, end)}, nil
}

func (p *Parser) parsePath() (*ast.Path, error) {
	startTok := p.peek()
	var pkg *ast.PackagePath
	if p.match(token.TokenIdentifier) && p.peekAt(1).Type == token.TokenColon && p.peekAt(2).Type == token.TokenIdentifier && p.peekAt(3).Type == token.TokenSlash {
		ns := p.next()
		p.expect(token.TokenColon)
		name := p.expect(token.TokenIdentifier)
		p.expect(token.TokenSlash)
		pkg = &ast.PackagePath{
			Namespace: ns.Slice(p.src),
			Name:      name.Slice(p.src),
			SpanRange: ast.Span{Start: ns.Start, End: name.End},
		}
	}
	first, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	segments := []ast.Name{first}
	for p.match(token.TokenDot) || p.match(token.TokenSlash) {
		p.next()
		part, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		segments = append(segments, part)
	}
	return &ast.Path{Package: pkg, Segments: segments, SpanRange: ast.Span{Start: startTok.Start, End: segments[len(segments)-1].SpanRange.End}}, nil
}

func (p *Parser) parseUsePath() (*ast.Path, error) {
	startTok := p.peek()
	var pkg *ast.PackagePath
	if p.match(token.TokenIdentifier) && p.peekAt(1).Type == token.TokenColon && p.peekAt(2).Type == token.TokenIdentifier && p.peekAt(3).Type == token.TokenSlash {
		ns := p.next()
		p.expect(token.TokenColon)
		name := p.expect(token.TokenIdentifier)
		p.expect(token.TokenSlash)
		pkg = &ast.PackagePath{
			Namespace: ns.Slice(p.src),
			Name:      name.Slice(p.src),
			SpanRange: ast.Span{Start: ns.Start, End: name.End},
		}
	}
	first, err := p.expectAnyName()
	if err != nil {
		return nil, err
	}
	segments := []ast.Name{first}
	for (p.match(token.TokenDot) && p.peekAt(1).Type != token.TokenLBrace) || p.match(token.TokenSlash) {
		p.next()
		part, err := p.expectAnyName()
		if err != nil {
			return nil, err
		}
		segments = append(segments, part)
	}
	return &ast.Path{
		Package:   pkg,
		Segments:  segments,
		SpanRange: ast.Span{Start: startTok.Start, End: segments[len(segments)-1].SpanRange.End},
	}, nil
}

func (p *Parser) parseVersion() (string, error) {
	first := p.expect(token.TokenInteger)
	version := first.Slice(p.src)
	for p.match(token.TokenDot) {
		p.next()
		version += "." + p.expect(token.TokenInteger).Slice(p.src)
	}
	return version, nil
}

func (p *Parser) parseAttributes() ([]*ast.Attribute, error) {
	var attrs []*ast.Attribute
	for p.match(token.TokenAt) {
		attr, err := p.parseAttribute()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (p *Parser) parseAttribute() (*ast.Attribute, error) {
	start := p.expect(token.TokenAt)
	nameTok := p.expect(token.TokenIdentifier)
	attr := &ast.Attribute{Name: nameTok.Slice(p.src)}
	if p.match(token.TokenLParen) {
		p.next()
		for !p.match(token.TokenRParen) {
			argName := p.expect(token.TokenIdentifier)
			p.expect(token.TokenEquals)
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			attr.Arguments = append(attr.Arguments, ast.AttributeArg{Name: argName.Slice(p.src), Value: val})
			if !p.match(token.TokenComma) {
				break
			}
			p.next()
		}
		end := p.expect(token.TokenRParen)
		attr.SpanRange = span(start, end)
		return attr, nil
	}
	attr.SpanRange = ast.Span{Start: start.Start, End: nameTok.End}
	return attr, nil
}

func (p *Parser) parseValue() (ast.Value, error) {
	tok := p.peek()
	switch tok.Type {
	case token.TokenString:
		p.next()
		return &ast.StringValue{Value: lexer.Unquote(tok.Slice(p.src)), Raw: tok.Slice(p.src), SpanRange: ast.Span{Start: tok.Start, End: tok.End}}, nil
	case token.TokenInteger:
		p.next()
		return &ast.IntValue{Value: tok.Slice(p.src), SpanRange: ast.Span{Start: tok.Start, End: tok.End}}, nil
	default:
		return nil, p.errorf(tok, "expected attribute value")
	}
}

func (p *Parser) expectAnyName() (ast.Name, error) {
	tok := p.peek()
	switch tok.Type {
	case token.TokenIdentifier, token.TokenExplicitID:
		p.next()
		return ast.Name{Text: tok.Slice(p.src), Explicit: tok.Type == token.TokenExplicitID, SpanRange: ast.Span{Start: tok.Start, End: tok.End}}, nil
	default:
		return ast.Name{}, p.errorf(tok, "expected identifier")
	}
}

func (p *Parser) expectName(explicit bool) (ast.Name, error) {
	name, err := p.expectAnyName()
	if err != nil {
		return ast.Name{}, err
	}
	if name.Explicit != explicit {
		return ast.Name{}, p.errorf(p.prev(), "unexpected explicit identifier")
	}
	return name, nil
}

func (p *Parser) isPathStart() bool {
	t := p.peek().Type
	return t == token.TokenIdentifier || t == token.TokenExplicitID
}

func (p *Parser) hasSemicolonBeforeColon() bool {
	for i := p.pos; i < len(p.tokens); i++ {
		switch p.tokens[i].Type {
		case token.TokenColon:
			return false
		case token.TokenSemicolon:
			return true
		case token.TokenLBrace:
			return false
		}
	}
	return false
}

func (p *Parser) expect(tt token.TokenType) token.Token {
	tok := p.peek()
	if tok.Type != tt {
		panic(p.errorf(tok, "expected %s", tt))
	}
	p.pos++
	return tok
}

func (p *Parser) match(tt token.TokenType) bool {
	return p.peek().Type == tt
}

func (p *Parser) next() token.Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) peek() token.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekAt(offset int) token.Token {
	index := p.pos + offset
	if index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[index]
}

func (p *Parser) prev() token.Token {
	if p.pos == 0 {
		return p.tokens[0]
	}
	return p.tokens[p.pos-1]
}

func (p *Parser) errorf(tok token.Token, format string, args ...any) error {
	return fmt.Errorf("parser: %s at %s", fmt.Sprintf(format, args...), tok.Start)
}

func span(start, end token.Token) ast.Span {
	return ast.Span{Start: start.Start, End: end.End}
}
