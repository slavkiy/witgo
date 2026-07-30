package lexer

import (
	"reflect"
	"testing"

	"github.com/slavkiy/witgo/internal/parser/token"
)

func TestLexCurrentWITShape(t *testing.T) {
	input := `package local:demo@1.2.3;

@since(version = "1.2.3")
interface host-functions {
  /// Host functions
  use types.{errno as host-errno};
  ping: func() -> string;
  read: func(self: borrow<file>, path: string) -> result<string>;
  %[method]file.open: func(path: string) -> own<file>;
}
`

	tokens, err := Lex(input)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}

	got := make([]token.TokenType, 0, len(tokens))
	for _, tok := range tokens {
		got = append(got, tok.Type)
	}

	want := []token.TokenType{
		token.TokenKeywordPackage, token.TokenIdentifier, token.TokenColon, token.TokenIdentifier, token.TokenAt,
		token.TokenInteger, token.TokenDot, token.TokenInteger, token.TokenDot, token.TokenInteger, token.TokenSemicolon,
		token.TokenAt, token.TokenIdentifier, token.TokenLParen, token.TokenIdentifier, token.TokenEquals, token.TokenString, token.TokenRParen,
		token.TokenKeywordInterface, token.TokenIdentifier, token.TokenLBrace,
		token.TokenDocComment,
		token.TokenKeywordUse, token.TokenIdentifier, token.TokenDot, token.TokenLBrace, token.TokenIdentifier, token.TokenKeywordAs, token.TokenIdentifier, token.TokenRBrace, token.TokenSemicolon,
		token.TokenIdentifier, token.TokenColon, token.TokenKeywordFunc, token.TokenLParen, token.TokenRParen, token.TokenArrow, token.TokenKeywordString, token.TokenSemicolon,
		token.TokenIdentifier, token.TokenColon, token.TokenKeywordFunc, token.TokenLParen, token.TokenIdentifier, token.TokenColon, token.TokenKeywordBorrow, token.TokenLAngle, token.TokenIdentifier, token.TokenRAngle, token.TokenComma, token.TokenIdentifier, token.TokenColon, token.TokenKeywordString, token.TokenRParen, token.TokenArrow, token.TokenKeywordResult, token.TokenLAngle, token.TokenKeywordString, token.TokenRAngle, token.TokenSemicolon,
		token.TokenExplicitID, token.TokenColon, token.TokenKeywordFunc, token.TokenLParen, token.TokenIdentifier, token.TokenColon, token.TokenKeywordString, token.TokenRParen, token.TokenArrow, token.TokenKeywordOwn, token.TokenLAngle, token.TokenIdentifier, token.TokenRAngle, token.TokenSemicolon,
		token.TokenRBrace, token.TokenEOF,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected token types:\nwant: %v\ngot:  %v", want, got)
	}
}

func TestLexNestedBlockComment(t *testing.T) {
	input := `interface foo { /* a /* nested */ block */ }`
	tokens, err := Lex(input)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if tokens[0].Type != token.TokenKeywordInterface {
		t.Fatalf("unexpected first token: %#v", tokens[0])
	}
}

func TestUnquote(t *testing.T) {
	if got := Unquote(`"a\nb"`); got != "a\nb" {
		t.Fatalf("unexpected unquote: %q", got)
	}
}
