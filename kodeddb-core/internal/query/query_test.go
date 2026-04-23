package query

import (
	"fmt"
	"testing"

	"github.com/Koded0214h/kodeddb-core/internal/engine"
	"github.com/Koded0214h/kodeddb-core/internal/storage"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	dir := t.TempDir()
	e, err := engine.Open(engine.Options{
		DataFile: dir + "/koded.db",
		WALFile:  dir + "/koded.wal",
	})
	if err != nil {
		t.Fatalf("engine.Open: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	ss, err := storage.NewSchemaStore(e)
	if err != nil {
		t.Fatalf("NewSchemaStore: %v", err)
	}
	return NewExecutor(e, ss)
}

func mustExec(t *testing.T, ex *Executor, sql string) *ResultSet {
	t.Helper()
	rs, err := ex.Exec(sql)
	if err != nil {
		t.Fatalf("Exec(%q): %v", sql, err)
	}
	return rs
}

// ── Lexer tests ───────────────────────────────────────────────────────────────

func TestLexerBasic(t *testing.T) {
	l := NewLexer("SELECT * FROM users WHERE id = 1;")
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	types := []TokenType{
		TOK_SELECT, TOK_ALL, TOK_FROM, TOK_IDENT,
		TOK_WHERE, TOK_IDENT, TOK_EQ, TOK_INT,
		TOK_SEMI, TOK_EOF,
	}
	for i, expected := range types {
		if tokens[i].Type != expected {
			t.Errorf("token %d: expected type %d got %d (%q)",
				i, expected, tokens[i].Type, tokens[i].Literal)
		}
	}
}

func TestLexerStringLiteral(t *testing.T) {
	l := NewLexer("'hello world'")
	tokens, _ := l.Tokenize()
	if tokens[0].Type != TOK_STRING || tokens[0].Literal != "hello world" {
		t.Errorf("expected string token, got %+v", tokens[0])
	}
}

func TestLexerOperators(t *testing.T) {
	cases := []struct {
		input string
		tt    TokenType
	}{
		{"=", TOK_EQ}, {"!=", TOK_NEQ},
		{"<", TOK_LT}, {"<=", TOK_LTE},
		{">", TOK_GT}, {">=", TOK_GTE},
	}
	for _, c := range cases {
		l := NewLexer(c.input)
		toks, _ := l.Tokenize()
		if toks[0].Type != c.tt {
			t.Errorf("input %q: expected %d got %d", c.input, c.tt, toks[0].Type)
		}
	}
}

func TestLexerLineComment(t *testing.T) {
	l := NewLexer("SELECT -- this is a comment\n* FROM t")
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	// Should be: SELECT, *, FROM, t, EOF
	if len(tokens) != 5 {
		t.Errorf("expected 5 tokens, got %d: %v", len(tokens), tokens)
	}
}

// ── Parser tests ──────────────────────────────────────────────────────────────

func TestParseCreateTable(t *testing.T) {
	stmt, err := Parse("CREATE TABLE users (id INT PRIMARY KEY, name TEXT, score FLOAT)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, ok := stmt.(*CreateTableStmt)
	if !ok {
		t.Fatalf("expected CreateTableStmt")
	}
	if ct.TableName != "users" {
		t.Errorf("expected table 'users', got %q", ct.TableName)
	}
	if len(ct.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(ct.Columns))
	}
	if !ct.Columns[0].PrimaryKey {
		t.Error("first column should be primary key")
	}
	if ct.Columns[0].Type != types.TypeInt {
		t.Errorf("expected INT, got %s", ct.Columns[0].Type)
	}
}

func TestParseInsert(t *testing.T) {
	stmt, err := Parse("INSERT INTO users (id, name) VALUES (1, 'koded')")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ins, ok := stmt.(*InsertStmt)
	if !ok { t.Fatal("expected InsertStmt") }
	if ins.TableName != "users" { t.Errorf("wrong table: %q", ins.TableName) }
	if len(ins.Columns) != 2 { t.Errorf("expected 2 columns, got %d", len(ins.Columns)) }
	if ins.Values[0].IntVal != 1 { t.Errorf("expected id=1") }
	if ins.Values[1].TextVal != "koded" { t.Errorf("expected name='koded'") }
}

func TestParseSelect(t *testing.T) {
	stmt, err := Parse("SELECT id, name FROM users WHERE id > 0 LIMIT 10")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sel, ok := stmt.(*SelectStmt)
	if !ok { t.Fatal("expected SelectStmt") }
	if sel.TableName != "users" { t.Error("wrong table") }
	if len(sel.Columns) != 2 { t.Errorf("expected 2 cols, got %d", len(sel.Columns)) }
	if sel.Where == nil { t.Error("expected WHERE") }
	if sel.Limit != 10 { t.Errorf("expected LIMIT 10, got %d", sel.Limit) }
}

func TestParseSelectStar(t *testing.T) {
	stmt, err := Parse("SELECT * FROM products")
	if err != nil { t.Fatalf("Parse: %v", err) }
	sel := stmt.(*SelectStmt)
	if len(sel.Columns) != 0 { t.Error("SELECT * should have empty column list") }
}

func TestParseUpdate(t *testing.T) {
	stmt, err := Parse("UPDATE users SET name = 'new', score = 9.5 WHERE id = 1")
	if err != nil { t.Fatalf("Parse: %v", err) }
	upd := stmt.(*UpdateStmt)
	if len(upd.Assigns) != 2 { t.Errorf("expected 2 assignments, got %d", len(upd.Assigns)) }
	if upd.Assigns[0].Column != "name" { t.Error("wrong column") }
}

func TestParseDelete(t *testing.T) {
	stmt, err := Parse("DELETE FROM users WHERE id = 42")
	if err != nil { t.Fatalf("Parse: %v", err) }
	del := stmt.(*DeleteStmt)
	if del.TableName != "users" { t.Error("wrong table") }
	if del.Where == nil { t.Error("expected WHERE") }
}

func TestParseWhereAnd(t *testing.T) {
	stmt, err := Parse("SELECT * FROM t WHERE age > 18 AND active = true")
	if err != nil { t.Fatalf("Parse: %v", err) }
	sel := stmt.(*SelectStmt)
	if sel.Where.IsLeaf { t.Error("expected branch WHERE") }
	if sel.Where.LogicOp != "AND" { t.Errorf("expected AND, got %q", sel.Where.LogicOp) }
}

func TestParseDropTable(t *testing.T) {
	stmt, err := Parse("DROP TABLE users")
	if err != nil { t.Fatalf("Parse: %v", err) }
	dt := stmt.(*DropTableStmt)
	if dt.TableName != "users" { t.Error("wrong table") }
}

// ── WHERE eval tests ──────────────────────────────────────────────────────────

func TestWhereEval(t *testing.T) {
	row := map[string]types.Value{
		"age":  types.IntValue(25),
		"name": types.TextValue("koded"),
	}

	cases := []struct {
		expr   *WhereExpr
		expect bool
	}{
		{
			&WhereExpr{IsLeaf: true, Column: "age", Op: ">", Value: types.IntValue(18)},
			true,
		},
		{
			&WhereExpr{IsLeaf: true, Column: "age", Op: "=", Value: types.IntValue(30)},
			false,
		},
		{
			&WhereExpr{IsLeaf: true, Column: "name", Op: "=", Value: types.TextValue("koded")},
			true,
		},
		{
			&WhereExpr{
				IsLeaf:  false,
				LogicOp: "AND",
				Left:  &WhereExpr{IsLeaf: true, Column: "age",  Op: ">",  Value: types.IntValue(18)},
				Right: &WhereExpr{IsLeaf: true, Column: "name", Op: "!=", Value: types.TextValue("bob")},
			},
			true,
		},
	}

	for i, c := range cases {
		got, err := c.expr.Eval(row)
		if err != nil { t.Errorf("case %d: eval error: %v", i, err) }
		if got != c.expect { t.Errorf("case %d: expected %v got %v", i, c.expect, got) }
	}
}

// ── Schema + value serialization tests ───────────────────────────────────────

func TestValueRoundtrip(t *testing.T) {
	vals := []types.Value{
		types.IntValue(42),
		types.FloatValue(3.14),
		types.TextValue("hello koded"),
		types.BoolValue(true),
		types.BoolValue(false),
		types.NullValue,
		types.BytesValue([]byte{0xDE, 0xAD, 0xBE, 0xEF}),
	}

	for _, v := range vals {
		encoded := types.EncodeValue(v)
		decoded, _, err := types.DecodeValue(encoded)
		if err != nil {
			t.Errorf("decode %v: %v", v, err)
			continue
		}
		if !decoded.EqualTo(v) {
			t.Errorf("roundtrip mismatch: in=%v out=%v", v, decoded)
		}
	}
}

func TestSemiValueRoundtrip(t *testing.T) {
	semi := types.SemiValue(map[string]types.Value{
		"age":     types.IntValue(25),
		"company": types.TextValue("Teenovatex"),
		"active":  types.BoolValue(true),
	})
	encoded := types.EncodeValue(semi)
	decoded, _, err := types.DecodeValue(encoded)
	if err != nil { t.Fatalf("decode semi: %v", err) }
	if decoded.Type != types.TypeSemi { t.Fatal("expected SEMI type") }
	if decoded.SemiVal["age"].IntVal != 25 { t.Error("age mismatch") }
	if decoded.SemiVal["company"].TextVal != "Teenovatex" { t.Error("company mismatch") }
}

func TestSchemaRoundtrip(t *testing.T) {
	schema := &types.TableSchema{
		Name: "users",
		Columns: []types.Column{
			{Name: "id",   Type: types.TypeInt,  PrimaryKey: true},
			{Name: "name", Type: types.TypeText, Nullable: true},
			{Name: "meta", Type: types.TypeSemi, Nullable: true},
		},
	}
	encoded := types.EncodeSchema(schema)
	decoded, err := types.DecodeSchema(encoded)
	if err != nil { t.Fatalf("decode schema: %v", err) }
	if decoded.Name != schema.Name { t.Errorf("name mismatch") }
	if len(decoded.Columns) != 3 { t.Errorf("wrong column count") }
	if !decoded.Columns[0].PrimaryKey { t.Error("pk lost") }
}

// ── Full integration: SQL → engine ───────────────────────────────────────────

func TestExecCreateAndInsert(t *testing.T) {
	ex := newTestExecutor(t)

	mustExec(t, ex, "CREATE TABLE users (id INT PRIMARY KEY, name TEXT, score FLOAT)")
	mustExec(t, ex, "INSERT INTO users VALUES (1, 'koded', 9.5)")
	mustExec(t, ex, "INSERT INTO users VALUES (2, 'shaz', 8.0)")

	rs := mustExec(t, ex, "SELECT * FROM users")
	if len(rs.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rs.Rows))
	}
}

func TestExecSelectWhere(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE products (id INT PRIMARY KEY, name TEXT, price FLOAT)")
	for i := 1; i <= 5; i++ {
		mustExec(t, ex, fmt.Sprintf(
			"INSERT INTO products VALUES (%d, 'item%d', %d.0)", i, i, i*10,
		))
	}

	rs := mustExec(t, ex, "SELECT * FROM products WHERE price > 20.0")
	if len(rs.Rows) != 3 {
		t.Errorf("expected 3 rows with price > 20, got %d", len(rs.Rows))
	}
}

func TestExecUpdate(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE items (id INT PRIMARY KEY, status TEXT)")
	mustExec(t, ex, "INSERT INTO items VALUES (1, 'pending')")
	mustExec(t, ex, "INSERT INTO items VALUES (2, 'pending')")
	mustExec(t, ex, "INSERT INTO items VALUES (3, 'done')")

	rs := mustExec(t, ex, "UPDATE items SET status = 'active' WHERE status = 'pending'")
	if rs.Affected != 2 {
		t.Errorf("expected 2 updated, got %d", rs.Affected)
	}

	rs2 := mustExec(t, ex, "SELECT * FROM items WHERE status = 'active'")
	if len(rs2.Rows) != 2 {
		t.Errorf("expected 2 active rows, got %d", len(rs2.Rows))
	}
}

func TestExecDelete(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE logs (id INT PRIMARY KEY, level TEXT)")
	for i := 1; i <= 5; i++ {
		mustExec(t, ex, fmt.Sprintf("INSERT INTO logs VALUES (%d, 'info')", i))
	}
	mustExec(t, ex, "INSERT INTO logs VALUES (6, 'error')")

	rs := mustExec(t, ex, "DELETE FROM logs WHERE level = 'info'")
	if rs.Affected != 5 { t.Errorf("expected 5 deleted, got %d", rs.Affected) }

	rs2 := mustExec(t, ex, "SELECT * FROM logs")
	if len(rs2.Rows) != 1 { t.Errorf("expected 1 row remaining, got %d", len(rs2.Rows)) }
}

func TestExecDropTable(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE temp (id INT PRIMARY KEY, val TEXT)")
	mustExec(t, ex, "INSERT INTO temp VALUES (1, 'x')")
	mustExec(t, ex, "DROP TABLE temp")

	_, err := ex.Exec("SELECT * FROM temp")
	if err == nil {
		t.Error("expected error querying dropped table")
	}
}

func TestExecLimit(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE nums (id INT PRIMARY KEY, val INT)")
	for i := 1; i <= 10; i++ {
		mustExec(t, ex, fmt.Sprintf("INSERT INTO nums VALUES (%d, %d)", i, i))
	}
	rs := mustExec(t, ex, "SELECT * FROM nums LIMIT 3")
	if len(rs.Rows) != 3 {
		t.Errorf("expected 3 rows with LIMIT 3, got %d", len(rs.Rows))
	}
}

func TestExecSemiColumn(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE users (id INT PRIMARY KEY, name TEXT, meta SEMI)")
	mustExec(t, ex, "INSERT INTO users VALUES (1, 'koded', null)")

	rs := mustExec(t, ex, "SELECT id, name FROM users WHERE id = 1")
	if len(rs.Rows) != 1 { t.Fatalf("expected 1 row, got %d", len(rs.Rows)) }
	if rs.Rows[0][0].IntVal != 1 { t.Error("wrong id") }
	if rs.Rows[0][1].TextVal != "koded" { t.Error("wrong name") }
}

func TestExecTableAlreadyExists(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE x (id INT PRIMARY KEY)")
	_, err := ex.Exec("CREATE TABLE x (id INT PRIMARY KEY)")
	if err == nil { t.Error("expected error for duplicate table") }
}

func TestExecSelectProjection(t *testing.T) {
	ex := newTestExecutor(t)
	mustExec(t, ex, "CREATE TABLE p (id INT PRIMARY KEY, a TEXT, b TEXT, c INT)")
	mustExec(t, ex, "INSERT INTO p VALUES (1, 'hello', 'world', 42)")

	rs := mustExec(t, ex, "SELECT a, c FROM p WHERE id = 1")
	if len(rs.Columns) != 2 { t.Fatalf("expected 2 columns, got %d", len(rs.Columns)) }
	if rs.Rows[0][0].TextVal != "hello" { t.Error("wrong a") }
	if rs.Rows[0][1].IntVal != 42 { t.Error("wrong c") }
}