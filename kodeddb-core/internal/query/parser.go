package query

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

// ── AST node types ────────────────────────────────────────────────────────────

type Statement interface{ stmtNode() }

// CREATE TABLE users (id INT PRIMARY KEY, name TEXT, meta SEMI)
type CreateTableStmt struct {
	TableName string
	Columns   []types.Column
}
func (s *CreateTableStmt) stmtNode() {}

// DROP TABLE users
type DropTableStmt struct {
	TableName string
}
func (s *DropTableStmt) stmtNode() {}

// INSERT INTO users VALUES (1, 'koded', null)
// INSERT INTO users (id, name) VALUES (1, 'koded')
type InsertStmt struct {
	TableName string
	Columns   []string        // nil = all columns in schema order
	Values    []types.Value
}
func (s *InsertStmt) stmtNode() {}

// SELECT * FROM users WHERE id = 1
// SELECT id, name FROM users WHERE age > 18 LIMIT 10
type SelectStmt struct {
	TableName string
	Columns   []string  // empty = SELECT *
	Where     *WhereExpr
	Limit     int       // 0 = no limit
	OrderBy   string
	OrderDesc bool
}
func (s *SelectStmt) stmtNode() {}

// UPDATE users SET name = 'new' WHERE id = 1
type UpdateStmt struct {
	TableName string
	Assigns   []Assignment
	Where     *WhereExpr
}
func (s *UpdateStmt) stmtNode() {}

type Assignment struct {
	Column string
	Value  types.Value
}

// DELETE FROM users WHERE id = 1
type DeleteStmt struct {
	TableName string
	Where     *WhereExpr
}
func (s *DeleteStmt) stmtNode() {}

// ── WHERE expression tree ─────────────────────────────────────────────────────

type WhereExpr struct {
	// Leaf: a single condition  (col op val)
	IsLeaf bool
	Column string
	Op     string      // "=", "!=", "<", "<=", ">", ">="
	Value  types.Value

	// Branch: two sub-expressions joined by AND/OR
	Left     *WhereExpr
	Right    *WhereExpr
	LogicOp  string // "AND" or "OR"
}

// Eval returns true if the given row values satisfy this expression.
func (w *WhereExpr) Eval(rowVals map[string]types.Value) (bool, error) {
	if w == nil { return true, nil }

	if w.IsLeaf {
		rowVal, ok := rowVals[w.Column]
		if !ok {
			return false, fmt.Errorf("column %q not found in row", w.Column)
		}
		cmp, err := rowVal.Compare(w.Value)
		if err != nil {
			// Type mismatch — treat as false (don't error on scan)
			return false, nil
		}
		switch w.Op {
		case "=":  return cmp == 0, nil
		case "!=": return cmp != 0, nil
		case "<":  return cmp < 0, nil
		case "<=": return cmp <= 0, nil
		case ">":  return cmp > 0, nil
		case ">=": return cmp >= 0, nil
		default:
			return false, fmt.Errorf("unknown operator %q", w.Op)
		}
	}

	// Branch
	left, err := w.Left.Eval(rowVals)
	if err != nil { return false, err }
	right, err := w.Right.Eval(rowVals)
	if err != nil { return false, err }

	switch strings.ToUpper(w.LogicOp) {
	case "AND": return left && right, nil
	case "OR":  return left || right, nil
	default:
		return false, fmt.Errorf("unknown logic op %q", w.LogicOp)
	}
}

// ── Parser ────────────────────────────────────────────────────────────────────

type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

func Parse(sql string) (Statement, error) {
	l := NewLexer(sql)
	tokens, err := l.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lex: %w", err)
	}
	p := NewParser(tokens)
	return p.ParseStatement()
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOK_EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *Parser) expect(tt TokenType) (Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected token type %d, got %d (%q) at L%d:C%d",
			tt, tok.Type, tok.Literal, tok.Line, tok.Col)
	}
	return p.advance(), nil
}

func (p *Parser) expectIdent() (string, error) {
	tok, err := p.expect(TOK_IDENT)
	return tok.Literal, err
}

func (p *Parser) check(tt TokenType) bool {
	return p.peek().Type == tt
}

func (p *Parser) match(tt TokenType) bool {
	if p.check(tt) { p.advance(); return true }
	return false
}

// ── Top-level dispatch ────────────────────────────────────────────────────────

func (p *Parser) ParseStatement() (Statement, error) {
	tok := p.peek()
	switch tok.Type {
	case TOK_CREATE: return p.parseCreate()
	case TOK_DROP:   return p.parseDrop()
	case TOK_INSERT: return p.parseInsert()
	case TOK_SELECT: return p.parseSelect()
	case TOK_UPDATE: return p.parseUpdate()
	case TOK_DELETE: return p.parseDelete()
	default:
		return nil, fmt.Errorf("unexpected token %q at L%d:C%d", tok.Literal, tok.Line, tok.Col)
	}
}

// ── CREATE TABLE ──────────────────────────────────────────────────────────────
// CREATE TABLE name (col type [PRIMARY KEY], ...)

func (p *Parser) parseCreate() (*CreateTableStmt, error) {
	p.advance() // CREATE
	if _, err := p.expect(TOK_TABLE); err != nil { return nil, err }
	name, err := p.expectIdent()
	if err != nil { return nil, err }

	if _, err := p.expect(TOK_LPAREN); err != nil { return nil, err }

	var cols []types.Column
	for !p.check(TOK_RPAREN) && !p.check(TOK_EOF) {
		col, err := p.parseColumnDef()
		if err != nil { return nil, err }
		cols = append(cols, col)
		if !p.match(TOK_COMMA) { break }
	}
	if _, err := p.expect(TOK_RPAREN); err != nil { return nil, err }
	p.match(TOK_SEMI)

	return &CreateTableStmt{TableName: name, Columns: cols}, nil
}

func (p *Parser) parseColumnDef() (types.Column, error) {
	name, err := p.expectIdent()
	if err != nil { return types.Column{}, err }

	dtTok := p.advance()
	dt, err := tokenToDataType(dtTok)
	if err != nil { return types.Column{}, err }

	col := types.Column{Name: name, Type: dt, Nullable: true}

	// Optional: PRIMARY KEY
	if p.check(TOK_PRIMARY) {
		p.advance()
		if _, err := p.expect(TOK_KEY); err != nil { return col, err }
		col.PrimaryKey = true
		col.Nullable   = false
	}

	return col, nil
}

func tokenToDataType(tok Token) (types.DataType, error) {
	switch tok.Type {
	case TOK_INT_TYPE:   return types.TypeInt, nil
	case TOK_FLOAT_TYPE: return types.TypeFloat, nil
	case TOK_TEXT_TYPE:  return types.TypeText, nil
	case TOK_BOOL_TYPE:  return types.TypeBool, nil
	case TOK_BYTES_TYPE: return types.TypeBytes, nil
	case TOK_SEMI_TYPE:  return types.TypeSemi, nil
	default:
		return 0, fmt.Errorf("expected a type keyword, got %q", tok.Literal)
	}
}

// ── DROP TABLE ────────────────────────────────────────────────────────────────

func (p *Parser) parseDrop() (*DropTableStmt, error) {
	p.advance() // DROP
	if _, err := p.expect(TOK_TABLE); err != nil { return nil, err }
	name, err := p.expectIdent()
	if err != nil { return nil, err }
	p.match(TOK_SEMI)
	return &DropTableStmt{TableName: name}, nil
}

// ── INSERT ────────────────────────────────────────────────────────────────────
// INSERT INTO name VALUES (v1, v2, ...)
// INSERT INTO name (c1, c2) VALUES (v1, v2)

func (p *Parser) parseInsert() (*InsertStmt, error) {
	p.advance() // INSERT
	if _, err := p.expect(TOK_INTO); err != nil { return nil, err }
	name, err := p.expectIdent()
	if err != nil { return nil, err }

	stmt := &InsertStmt{TableName: name}

	// Optional column list
	if p.check(TOK_LPAREN) {
		p.advance()
		for !p.check(TOK_RPAREN) && !p.check(TOK_EOF) {
			col, err := p.expectIdent()
			if err != nil { return nil, err }
			stmt.Columns = append(stmt.Columns, col)
			if !p.match(TOK_COMMA) { break }
		}
		if _, err := p.expect(TOK_RPAREN); err != nil { return nil, err }
	}

	// VALUES (...)
	if tok := p.advance(); tok.Type != TOK_VALUES {
		return nil, fmt.Errorf("expected VALUES, got %q", tok.Literal)
	}
	if _, err := p.expect(TOK_LPAREN); err != nil { return nil, err }
	for !p.check(TOK_RPAREN) && !p.check(TOK_EOF) {
		val, err := p.parseValue()
		if err != nil { return nil, err }
		stmt.Values = append(stmt.Values, val)
		if !p.match(TOK_COMMA) { break }
	}
	if _, err := p.expect(TOK_RPAREN); err != nil { return nil, err }
	p.match(TOK_SEMI)

	return stmt, nil
}

// ── SELECT ────────────────────────────────────────────────────────────────────
// SELECT * FROM name [WHERE ...] [ORDER BY col [ASC|DESC]] [LIMIT n]
// SELECT c1, c2 FROM name [WHERE ...]

func (p *Parser) parseSelect() (*SelectStmt, error) {
	p.advance() // SELECT
	stmt := &SelectStmt{Limit: 0}

	// Column list or *
	if p.check(TOK_ALL) {
		p.advance()
	} else {
		for {
			col, err := p.expectIdent()
			if err != nil { return nil, err }
			stmt.Columns = append(stmt.Columns, col)
			if !p.match(TOK_COMMA) { break }
		}
	}

	if _, err := p.expect(TOK_FROM); err != nil { return nil, err }
	name, err := p.expectIdent()
	if err != nil { return nil, err }
	stmt.TableName = name

	// Optional WHERE
	if p.check(TOK_WHERE) {
		p.advance()
		where, err := p.parseWhereExpr()
		if err != nil { return nil, err }
		stmt.Where = where
	}

	// Optional ORDER BY
	if p.check(TOK_ORDER) {
		p.advance()
		if _, err := p.expect(TOK_BY); err != nil { return nil, err }
		col, err := p.expectIdent()
		if err != nil { return nil, err }
		stmt.OrderBy = col
		if p.check(TOK_DESC) { p.advance(); stmt.OrderDesc = true  }
		if p.check(TOK_ASC)  { p.advance(); stmt.OrderDesc = false }
	}

	// Optional LIMIT
	if p.check(TOK_LIMIT) {
		p.advance()
		tok, err := p.expect(TOK_INT)
		if err != nil { return nil, err }
		n, _ := strconv.Atoi(tok.Literal)
		stmt.Limit = n
	}

	p.match(TOK_SEMI)
	return stmt, nil
}

// ── UPDATE ────────────────────────────────────────────────────────────────────
// UPDATE name SET col = val [, col = val ...] [WHERE ...]

func (p *Parser) parseUpdate() (*UpdateStmt, error) {
	p.advance() // UPDATE
	name, err := p.expectIdent()
	if err != nil { return nil, err }

	if tok := p.advance(); tok.Type != TOK_SET {
		return nil, fmt.Errorf("expected SET, got %q", tok.Literal)
	}

	stmt := &UpdateStmt{TableName: name}
	for {
		col, err := p.expectIdent()
		if err != nil { return nil, err }
		if _, err := p.expect(TOK_EQ); err != nil { return nil, err }
		val, err := p.parseValue()
		if err != nil { return nil, err }
		stmt.Assigns = append(stmt.Assigns, Assignment{Column: col, Value: val})
		if !p.match(TOK_COMMA) { break }
	}

	if p.check(TOK_WHERE) {
		p.advance()
		where, err := p.parseWhereExpr()
		if err != nil { return nil, err }
		stmt.Where = where
	}

	p.match(TOK_SEMI)
	return stmt, nil
}

// ── DELETE ────────────────────────────────────────────────────────────────────
// DELETE FROM name [WHERE ...]

func (p *Parser) parseDelete() (*DeleteStmt, error) {
	p.advance() // DELETE
	if _, err := p.expect(TOK_FROM); err != nil { return nil, err }
	name, err := p.expectIdent()
	if err != nil { return nil, err }

	stmt := &DeleteStmt{TableName: name}
	if p.check(TOK_WHERE) {
		p.advance()
		where, err := p.parseWhereExpr()
		if err != nil { return nil, err }
		stmt.Where = where
	}
	p.match(TOK_SEMI)
	return stmt, nil
}

// ── WHERE expression parser (recursive descent) ───────────────────────────────
// Handles: col op val [AND|OR col op val ...]

func (p *Parser) parseWhereExpr() (*WhereExpr, error) {
	left, err := p.parseCondition()
	if err != nil { return nil, err }

	for p.check(TOK_AND) || p.check(TOK_OR) {
		opTok := p.advance()
		right, err := p.parseCondition()
		if err != nil { return nil, err }
		left = &WhereExpr{
			IsLeaf:  false,
			Left:    left,
			Right:   right,
			LogicOp: strings.ToUpper(opTok.Literal),
		}
	}
	return left, nil
}

func (p *Parser) parseCondition() (*WhereExpr, error) {
	col, err := p.expectIdent()
	if err != nil { return nil, err }

	opTok := p.advance()
	op, err := tokenToOp(opTok)
	if err != nil { return nil, err }

	val, err := p.parseValue()
	if err != nil { return nil, err }

	return &WhereExpr{IsLeaf: true, Column: col, Op: op, Value: val}, nil
}

func tokenToOp(tok Token) (string, error) {
	switch tok.Type {
	case TOK_EQ:  return "=", nil
	case TOK_NEQ: return "!=", nil
	case TOK_LT:  return "<", nil
	case TOK_LTE: return "<=", nil
	case TOK_GT:  return ">", nil
	case TOK_GTE: return ">=", nil
	default:
		return "", fmt.Errorf("expected comparison operator, got %q", tok.Literal)
	}
}

// ── Value parsing ─────────────────────────────────────────────────────────────

func (p *Parser) parseValue() (types.Value, error) {
	tok := p.advance()
	switch tok.Type {
	case TOK_INT:
		n, err := strconv.ParseInt(tok.Literal, 10, 64)
		if err != nil { return types.NullValue, fmt.Errorf("bad int %q", tok.Literal) }
		return types.IntValue(n), nil
	case TOK_FLOAT:
		f, err := strconv.ParseFloat(tok.Literal, 64)
		if err != nil { return types.NullValue, fmt.Errorf("bad float %q", tok.Literal) }
		return types.FloatValue(f), nil
	case TOK_STRING:
		return types.TextValue(tok.Literal), nil
	case TOK_BOOL:
		return types.BoolValue(strings.ToUpper(tok.Literal) == "TRUE"), nil
	case TOK_NULL:
		return types.NullValue, nil
	default:
		return types.NullValue, fmt.Errorf("expected a value, got %q at L%d:C%d",
			tok.Literal, tok.Line, tok.Col)
	}
}