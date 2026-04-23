package query

import (
	"fmt"
	"strings"
	"unicode"
)

// ── Token types ───────────────────────────────────────────────────────────────

type TokenType int

const (
	// Literals
	TOK_IDENT  TokenType = iota // table_name, column_name
	TOK_INT                     // 42
	TOK_FLOAT                   // 3.14
	TOK_STRING                  // 'hello'
	TOK_BOOL                    // true, false

	// Keywords
	TOK_SELECT
	TOK_INSERT
	TOK_UPDATE
	TOK_DELETE
	TOK_CREATE
	TOK_DROP
	TOK_TABLE
	TOK_INTO
	TOK_FROM
	TOK_WHERE
	TOK_SET
	TOK_VALUES
	TOK_AND
	TOK_OR
	TOK_NOT
	TOK_NULL
	TOK_PRIMARY
	TOK_KEY
	TOK_INT_TYPE    // INT keyword as a type
	TOK_FLOAT_TYPE  // FLOAT keyword as a type
	TOK_TEXT_TYPE   // TEXT keyword as a type
	TOK_BOOL_TYPE   // BOOL keyword as a type
	TOK_BYTES_TYPE  // BYTES keyword as a type
	TOK_SEMI_TYPE   // SEMI keyword as a type
	TOK_LIMIT
	TOK_ORDER
	TOK_BY
	TOK_ASC
	TOK_DESC
	TOK_ALL // *

	// Operators
	TOK_EQ   // =
	TOK_NEQ  // !=
	TOK_LT   // <
	TOK_LTE  // <=
	TOK_GT   // >
	TOK_GTE  // >=

	// Punctuation
	TOK_LPAREN  // (
	TOK_RPAREN  // )
	TOK_COMMA   // ,
	TOK_DOT     // .
	TOK_SEMI    // ;

	TOK_EOF
	TOK_ILLEGAL
)

var keywords = map[string]TokenType{
	"SELECT":  TOK_SELECT,
	"INSERT":  TOK_INSERT,
	"UPDATE":  TOK_UPDATE,
	"DELETE":  TOK_DELETE,
	"CREATE":  TOK_CREATE,
	"DROP":    TOK_DROP,
	"TABLE":   TOK_TABLE,
	"INTO":    TOK_INTO,
	"FROM":    TOK_FROM,
	"WHERE":   TOK_WHERE,
	"SET":     TOK_SET,
	"VALUES":  TOK_VALUES,
	"AND":     TOK_AND,
	"OR":      TOK_OR,
	"NOT":     TOK_NOT,
	"NULL":    TOK_NULL,
	"TRUE":    TOK_BOOL,
	"FALSE":   TOK_BOOL,
	"PRIMARY": TOK_PRIMARY,
	"KEY":     TOK_KEY,
	"INT":     TOK_INT_TYPE,
	"INTEGER": TOK_INT_TYPE,
	"BIGINT":  TOK_INT_TYPE,
	"FLOAT":   TOK_FLOAT_TYPE,
	"REAL":    TOK_FLOAT_TYPE,
	"DOUBLE":  TOK_FLOAT_TYPE,
	"TEXT":    TOK_TEXT_TYPE,
	"VARCHAR": TOK_TEXT_TYPE,
	"STRING":  TOK_TEXT_TYPE,
	"BOOL":    TOK_BOOL_TYPE,
	"BOOLEAN": TOK_BOOL_TYPE,
	"BYTES":   TOK_BYTES_TYPE,
	"BLOB":    TOK_BYTES_TYPE,
	"SEMI":    TOK_SEMI_TYPE,
	"JSON":    TOK_SEMI_TYPE,
	"DYNAMIC": TOK_SEMI_TYPE,
	"LIMIT":   TOK_LIMIT,
	"ORDER":   TOK_ORDER,
	"BY":      TOK_BY,
	"ASC":     TOK_ASC,
	"DESC":    TOK_DESC,
}

// ── Token ─────────────────────────────────────────────────────────────────────

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, L%d:C%d)", t.Type, t.Literal, t.Line, t.Col)
}

// ── Lexer ─────────────────────────────────────────────────────────────────────

type Lexer struct {
	input  []rune
	pos    int
	line   int
	col    int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: []rune(input), pos: 0, line: 1, col: 1}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) { return 0 }
	return l.input[l.pos]
}

func (l *Lexer) peekAt(offset int) rune {
	p := l.pos + offset
	if p >= len(l.input) { return 0 }
	return l.input[p]
}

func (l *Lexer) advance() rune {
	ch := l.input[l.pos]
	l.pos++
	if ch == '\n' { l.line++; l.col = 1 } else { l.col++ }
	return ch
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsSpace(ch) {
			l.advance()
		} else if ch == '-' && l.peekAt(1) == '-' {
			// -- line comment
			for l.pos < len(l.input) && l.peek() != '\n' { l.advance() }
		} else {
			break
		}
	}
}

// Tokenize returns all tokens from the input.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.NextToken()
		if err != nil { return nil, err }
		tokens = append(tokens, tok)
		if tok.Type == TOK_EOF { break }
	}
	return tokens, nil
}

func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespaceAndComments()

	line, col := l.line, l.col

	if l.pos >= len(l.input) {
		return Token{Type: TOK_EOF, Literal: "", Line: line, Col: col}, nil
	}

	ch := l.peek()

	// String literal
	if ch == '\'' {
		return l.readString(line, col)
	}

	// Number
	if unicode.IsDigit(ch) || (ch == '-' && unicode.IsDigit(l.peekAt(1))) {
		return l.readNumber(line, col)
	}

	// Identifier or keyword
	if unicode.IsLetter(ch) || ch == '_' {
		return l.readIdent(line, col)
	}

	// Operators and punctuation
	l.advance()
	switch ch {
	case '=': return Token{TOK_EQ, "=", line, col}, nil
	case '!':
		if l.peek() == '=' { l.advance(); return Token{TOK_NEQ, "!=", line, col}, nil }
		return Token{TOK_ILLEGAL, "!", line, col}, fmt.Errorf("unexpected '!' at L%d:C%d", line, col)
	case '<':
		if l.peek() == '=' { l.advance(); return Token{TOK_LTE, "<=", line, col}, nil }
		return Token{TOK_LT, "<", line, col}, nil
	case '>':
		if l.peek() == '=' { l.advance(); return Token{TOK_GTE, ">=", line, col}, nil }
		return Token{TOK_GT, ">", line, col}, nil
	case '(': return Token{TOK_LPAREN, "(", line, col}, nil
	case ')': return Token{TOK_RPAREN, ")", line, col}, nil
	case ',': return Token{TOK_COMMA, ",", line, col}, nil
	case '.': return Token{TOK_DOT, ".", line, col}, nil
	case ';': return Token{TOK_SEMI, ";", line, col}, nil
	case '*': return Token{TOK_ALL, "*", line, col}, nil
	}

	return Token{TOK_ILLEGAL, string(ch), line, col},
		fmt.Errorf("unexpected character %q at L%d:C%d", ch, line, col)
}

func (l *Lexer) readString(line, col int) (Token, error) {
	l.advance() // consume opening '
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == '\'' {
			if l.peek() == '\'' {
				// Escaped quote ''
				l.advance()
				sb.WriteRune('\'')
			} else {
				return Token{TOK_STRING, sb.String(), line, col}, nil
			}
		} else {
			sb.WriteRune(ch)
		}
	}
	return Token{TOK_ILLEGAL, "", line, col}, fmt.Errorf("unterminated string at L%d:C%d", line, col)
}

func (l *Lexer) readNumber(line, col int) (Token, error) {
	var sb strings.Builder
	isFloat := false

	if l.peek() == '-' { sb.WriteRune(l.advance()) }

	for l.pos < len(l.input) && (unicode.IsDigit(l.peek()) || l.peek() == '.') {
		ch := l.advance()
		if ch == '.' {
			if isFloat {
				return Token{TOK_ILLEGAL, sb.String(), line, col},
					fmt.Errorf("malformed number at L%d:C%d", line, col)
			}
			isFloat = true
		}
		sb.WriteRune(ch)
	}

	lit := sb.String()
	if isFloat {
		return Token{TOK_FLOAT, lit, line, col}, nil
	}
	return Token{TOK_INT, lit, line, col}, nil
}

func (l *Lexer) readIdent(line, col int) (Token, error) {
	var sb strings.Builder
	for l.pos < len(l.input) && (unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_') {
		sb.WriteRune(l.advance())
	}
	lit := sb.String()
	upper := strings.ToUpper(lit)
	if tt, ok := keywords[upper]; ok {
		return Token{tt, lit, line, col}, nil
	}
	return Token{TOK_IDENT, lit, line, col}, nil
}