package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/slavkiy/witgo/internal/parser/token"
)

type Lexer struct {
	input  string
	offset int
	line   int
	column int
}

func New(input string) *Lexer {
	return &Lexer{input: input, line: 1, column: 1}
}

func Lex(input string) ([]token.Token, error) {
	l := New(input)
	var out []token.Token
	for {
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
		if tok.Type == token.TokenEOF {
			return out, nil
		}
	}
}

func (l *Lexer) Next() (token.Token, error) {
	for {
		if l.done() {
			pos := l.position()
			return token.Token{Type: token.TokenEOF, Start: pos, End: pos}, nil
		}

		switch r := l.peek(); {
		case isWhitespace(r):
			l.advance()
		case r == '/':
			tok, skip, err := l.lexCommentOrSlash()
			if err != nil {
				return tok, err
			}
			if !skip {
				return tok, nil
			}
		default:
			goto lex
		}
	}

lex:
	start := l.position()
	r := l.peek()

	switch {
	case isDigit(r):
		return l.lexInteger(start), nil
	case isIdentifierStart(r):
		return l.lexIdentifier(start), nil
	}

	switch r {
	case '%':
		return l.lexExplicitID(start)
	case '_':
		l.advance()
		return token.Token{Type: token.TokenUnderscore, Start: start, End: l.position()}, nil
	case '"':
		return l.lexString(start)
	case '@':
		l.advance()
		return token.Token{Type: token.TokenAt, Start: start, End: l.position()}, nil
	case ':':
		l.advance()
		return token.Token{Type: token.TokenColon, Start: start, End: l.position()}, nil
	case ';':
		l.advance()
		return token.Token{Type: token.TokenSemicolon, Start: start, End: l.position()}, nil
	case ',':
		l.advance()
		return token.Token{Type: token.TokenComma, Start: start, End: l.position()}, nil
	case '.':
		l.advance()
		return token.Token{Type: token.TokenDot, Start: start, End: l.position()}, nil
	case '=':
		l.advance()
		return token.Token{Type: token.TokenEquals, Start: start, End: l.position()}, nil
	case '(':
		l.advance()
		return token.Token{Type: token.TokenLParen, Start: start, End: l.position()}, nil
	case ')':
		l.advance()
		return token.Token{Type: token.TokenRParen, Start: start, End: l.position()}, nil
	case '{':
		l.advance()
		return token.Token{Type: token.TokenLBrace, Start: start, End: l.position()}, nil
	case '}':
		l.advance()
		return token.Token{Type: token.TokenRBrace, Start: start, End: l.position()}, nil
	case '[':
		l.advance()
		return token.Token{Type: token.TokenLBracket, Start: start, End: l.position()}, nil
	case ']':
		l.advance()
		return token.Token{Type: token.TokenRBracket, Start: start, End: l.position()}, nil
	case '<':
		l.advance()
		return token.Token{Type: token.TokenLAngle, Start: start, End: l.position()}, nil
	case '>':
		l.advance()
		return token.Token{Type: token.TokenRAngle, Start: start, End: l.position()}, nil
	case '*':
		l.advance()
		return token.Token{Type: token.TokenStar, Start: start, End: l.position()}, nil
	case '-':
		if l.peekN(1) == '>' {
			l.advance()
			l.advance()
			return token.Token{Type: token.TokenArrow, Start: start, End: l.position()}, nil
		}
	}

	l.advance()
	return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: unexpected character %q at %s", r, start)
}

func (l *Lexer) lexCommentOrSlash() (token.Token, bool, error) {
	start := l.position()

	if l.peekN(1) == '/' {
		l.advance()
		l.advance()
		doc := l.peek() == '/'
		if doc {
			l.advance()
		}
		for !l.done() {
			r := l.peek()
			if r == '\n' || r == '\r' {
				break
			}
			l.advance()
		}
		if doc {
			return token.Token{Type: token.TokenDocComment, Start: start, End: l.position()}, false, nil
		}
		return token.Token{}, true, nil
	}

	if l.peekN(1) == '*' {
		l.advance()
		l.advance()
		doc := l.peek() == '*'
		if doc {
			l.advance()
		}
		depth := 1
		for depth > 0 {
			if l.done() {
				return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, false, fmt.Errorf("lexer: unterminated block comment at %s", start)
			}
			switch {
			case l.peek() == '/' && l.peekN(1) == '*':
				depth++
				l.advance()
				l.advance()
			case l.peek() == '*' && l.peekN(1) == '/':
				depth--
				l.advance()
				l.advance()
			default:
				l.advance()
			}
		}
		if doc {
			return token.Token{Type: token.TokenDocComment, Start: start, End: l.position()}, false, nil
		}
		return token.Token{}, true, nil
	}

	l.advance()
	return token.Token{Type: token.TokenSlash, Start: start, End: l.position()}, false, nil
}

func (l *Lexer) lexInteger(start token.Position) token.Token {
	for !l.done() && isDigit(l.peek()) {
		l.advance()
	}
	return token.Token{Type: token.TokenInteger, Start: start, End: l.position()}
}

func (l *Lexer) lexIdentifier(start token.Position) token.Token {
	for !l.done() && isIdentifierPart(l.peek()) {
		l.advance()
	}
	lit := l.input[start.Offset:l.offset]
	if kw, ok := token.LookupKeyword(lit); ok {
		return token.Token{Type: kw, Start: start, End: l.position()}
	}
	return token.Token{Type: token.TokenIdentifier, Start: start, End: l.position()}
}

func (l *Lexer) lexExplicitID(start token.Position) (token.Token, error) {
	l.advance()
	if l.done() {
		return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: invalid explicit identifier at %s", start)
	}

	if l.peek() == '[' {
		l.advance()
		for !l.done() && l.peek() != ']' {
			l.advance()
		}
		if l.done() {
			return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: unterminated explicit identifier at %s", start)
		}
		l.advance()
	}

	if !isIdentifierStart(l.peek()) {
		return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: invalid explicit identifier at %s", start)
	}
	for !l.done() {
		r := l.peek()
		if isIdentifierPart(r) || r == '.' {
			l.advance()
			continue
		}
		break
	}
	return token.Token{Type: token.TokenExplicitID, Start: start, End: l.position()}, nil
}

func (l *Lexer) lexString(start token.Position) (token.Token, error) {
	l.advance()
	for !l.done() {
		r := l.advance()
		if r == '\\' {
			if l.done() {
				return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: unterminated string literal at %s", start)
			}
			l.advance()
			continue
		}
		if r == '"' {
			return token.Token{Type: token.TokenString, Start: start, End: l.position()}, nil
		}
		if r == '\n' || r == '\r' {
			return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: newline in string literal at %s", start)
		}
	}
	return token.Token{Type: token.TokenInvalid, Start: start, End: l.position()}, fmt.Errorf("lexer: unterminated string literal at %s", start)
}

func (l *Lexer) done() bool {
	return l.offset >= len(l.input)
}

func (l *Lexer) peek() rune {
	return l.peekN(0)
}

func (l *Lexer) peekN(n int) rune {
	offset := l.offset
	for i := 0; i < n; i++ {
		if offset >= len(l.input) {
			return utf8.RuneError
		}
		_, size := utf8.DecodeRuneInString(l.input[offset:])
		offset += size
	}
	if offset >= len(l.input) {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRuneInString(l.input[offset:])
	return r
}

func (l *Lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.input[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

func (l *Lexer) position() token.Position {
	return token.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isIdentifierStart(r rune) bool {
	return unicode.IsLetter(r)
}

func isIdentifierPart(r rune) bool {
	return unicode.IsLetter(r) || isDigit(r) || r == '-'
}

func Unquote(src string) string {
	if len(src) < 2 {
		return src
	}
	var b strings.Builder
	for i := 1; i < len(src)-1; i++ {
		if src[i] == '\\' && i+1 < len(src)-1 {
			i++
			switch src[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(src[i])
			}
			continue
		}
		b.WriteByte(src[i])
	}
	return b.String()
}
