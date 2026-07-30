package token

import "fmt"

type TokenType uint8

const (
	TokenInvalid TokenType = iota
	TokenEOF
	TokenIdentifier
	TokenInteger
	TokenString
	TokenExplicitID
	TokenUnderscore
	TokenDocComment

	TokenAt
	TokenColon
	TokenSemicolon
	TokenComma
	TokenDot
	TokenEquals
	TokenLParen
	TokenRParen
	TokenLBrace
	TokenRBrace
	TokenLBracket
	TokenRBracket
	TokenLAngle
	TokenRAngle
	TokenStar
	TokenSlash
	TokenArrow

	TokenKeywordAs
	TokenKeywordAsync
	TokenKeywordBool
	TokenKeywordBorrow
	TokenKeywordChar
	TokenKeywordConstructor
	TokenKeywordEnum
	TokenKeywordExport
	TokenKeywordF32
	TokenKeywordF64
	TokenKeywordFlags
	TokenKeywordFrom
	TokenKeywordFunc
	TokenKeywordFuture
	TokenKeywordImport
	TokenKeywordInclude
	TokenKeywordInterface
	TokenKeywordList
	TokenKeywordMap
	TokenKeywordOption
	TokenKeywordOwn
	TokenKeywordPackage
	TokenKeywordRecord
	TokenKeywordResource
	TokenKeywordResult
	TokenKeywordS8
	TokenKeywordS16
	TokenKeywordS32
	TokenKeywordS64
	TokenKeywordStatic
	TokenKeywordStream
	TokenKeywordString
	TokenKeywordTuple
	TokenKeywordType
	TokenKeywordU8
	TokenKeywordU16
	TokenKeywordU32
	TokenKeywordU64
	TokenKeywordUse
	TokenKeywordVariant
	TokenKeywordWith
	TokenKeywordWorld
)

var keywords = map[string]TokenType{
	"as":          TokenKeywordAs,
	"async":       TokenKeywordAsync,
	"bool":        TokenKeywordBool,
	"borrow":      TokenKeywordBorrow,
	"char":        TokenKeywordChar,
	"constructor": TokenKeywordConstructor,
	"enum":        TokenKeywordEnum,
	"export":      TokenKeywordExport,
	"f32":         TokenKeywordF32,
	"f64":         TokenKeywordF64,
	"flags":       TokenKeywordFlags,
	"from":        TokenKeywordFrom,
	"func":        TokenKeywordFunc,
	"future":      TokenKeywordFuture,
	"import":      TokenKeywordImport,
	"include":     TokenKeywordInclude,
	"interface":   TokenKeywordInterface,
	"list":        TokenKeywordList,
	"map":         TokenKeywordMap,
	"option":      TokenKeywordOption,
	"own":         TokenKeywordOwn,
	"package":     TokenKeywordPackage,
	"record":      TokenKeywordRecord,
	"resource":    TokenKeywordResource,
	"result":      TokenKeywordResult,
	"s8":          TokenKeywordS8,
	"s16":         TokenKeywordS16,
	"s32":         TokenKeywordS32,
	"s64":         TokenKeywordS64,
	"static":      TokenKeywordStatic,
	"stream":      TokenKeywordStream,
	"string":      TokenKeywordString,
	"tuple":       TokenKeywordTuple,
	"type":        TokenKeywordType,
	"u8":          TokenKeywordU8,
	"u16":         TokenKeywordU16,
	"u32":         TokenKeywordU32,
	"u64":         TokenKeywordU64,
	"use":         TokenKeywordUse,
	"variant":     TokenKeywordVariant,
	"with":        TokenKeywordWith,
	"world":       TokenKeywordWorld,
}

func LookupKeyword(lit string) (TokenType, bool) {
	tok, ok := keywords[lit]
	return tok, ok
}

type Position struct {
	Offset int
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type Token struct {
	Type  TokenType
	Start Position
	End   Position
}

func (t Token) String() string {
	return fmt.Sprintf("%s@%s", t.Type, t.Start)
}

func (t Token) Slice(src string) string {
	if t.Start.Offset < 0 || t.End.Offset < t.Start.Offset || t.End.Offset > len(src) {
		return ""
	}
	return src[t.Start.Offset:t.End.Offset]
}

func (t TokenType) String() string {
	switch t {
	case TokenInvalid:
		return "invalid"
	case TokenEOF:
		return "eof"
	case TokenIdentifier:
		return "identifier"
	case TokenInteger:
		return "integer"
	case TokenString:
		return "string"
	case TokenExplicitID:
		return "explicit_id"
	case TokenUnderscore:
		return "_"
	case TokenDocComment:
		return "doc_comment"
	case TokenAt:
		return "@"
	case TokenColon:
		return ":"
	case TokenSemicolon:
		return ";"
	case TokenComma:
		return ","
	case TokenDot:
		return "."
	case TokenEquals:
		return "="
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBrace:
		return "{"
	case TokenRBrace:
		return "}"
	case TokenLBracket:
		return "["
	case TokenRBracket:
		return "]"
	case TokenLAngle:
		return "<"
	case TokenRAngle:
		return ">"
	case TokenStar:
		return "*"
	case TokenSlash:
		return "/"
	case TokenArrow:
		return "->"
	case TokenKeywordAs:
		return "as"
	case TokenKeywordAsync:
		return "async"
	case TokenKeywordBool:
		return "bool"
	case TokenKeywordBorrow:
		return "borrow"
	case TokenKeywordChar:
		return "char"
	case TokenKeywordConstructor:
		return "constructor"
	case TokenKeywordEnum:
		return "enum"
	case TokenKeywordExport:
		return "export"
	case TokenKeywordF32:
		return "f32"
	case TokenKeywordF64:
		return "f64"
	case TokenKeywordFlags:
		return "flags"
	case TokenKeywordFrom:
		return "from"
	case TokenKeywordFunc:
		return "func"
	case TokenKeywordFuture:
		return "future"
	case TokenKeywordImport:
		return "import"
	case TokenKeywordInclude:
		return "include"
	case TokenKeywordInterface:
		return "interface"
	case TokenKeywordList:
		return "list"
	case TokenKeywordMap:
		return "map"
	case TokenKeywordOption:
		return "option"
	case TokenKeywordOwn:
		return "own"
	case TokenKeywordPackage:
		return "package"
	case TokenKeywordRecord:
		return "record"
	case TokenKeywordResource:
		return "resource"
	case TokenKeywordResult:
		return "result"
	case TokenKeywordS8:
		return "s8"
	case TokenKeywordS16:
		return "s16"
	case TokenKeywordS32:
		return "s32"
	case TokenKeywordS64:
		return "s64"
	case TokenKeywordStatic:
		return "static"
	case TokenKeywordStream:
		return "stream"
	case TokenKeywordString:
		return "string"
	case TokenKeywordTuple:
		return "tuple"
	case TokenKeywordType:
		return "type"
	case TokenKeywordU8:
		return "u8"
	case TokenKeywordU16:
		return "u16"
	case TokenKeywordU32:
		return "u32"
	case TokenKeywordU64:
		return "u64"
	case TokenKeywordUse:
		return "use"
	case TokenKeywordVariant:
		return "variant"
	case TokenKeywordWith:
		return "with"
	case TokenKeywordWorld:
		return "world"
	default:
		return fmt.Sprintf("token(%d)", uint8(t))
	}
}
