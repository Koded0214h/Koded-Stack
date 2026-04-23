package query

import (
	"fmt"
	"sort"

	"github.com/Koded0214h/kodeddb-core/internal/storage"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

// ── Result types ──────────────────────────────────────────────────────────────

type ResultSet struct {
	Columns []string
	Rows    [][]types.Value
	Affected int // rows inserted/updated/deleted
	Message  string
}

func (r *ResultSet) String() string {
	if r.Message != "" { return r.Message }
	if len(r.Rows) == 0 { return "(empty)" }
	out := fmt.Sprintf("(%d rows)\n", len(r.Rows))
	for _, row := range r.Rows {
		for i, v := range row {
			if i > 0 { out += " | " }
			out += v.String()
		}
		out += "\n"
	}
	return out
}

// ── Executor ──────────────────────────────────────────────────────────────────

type Executor struct {
	engine  storage.Storer
	schemas *storage.SchemaStore
}

func NewExecutor(engine storage.Storer, schemas *storage.SchemaStore) *Executor {
	return &Executor{engine: engine, schemas: schemas}
}

// Exec parses and executes a SQL string, returns a ResultSet.
func (ex *Executor) Exec(sql string) (*ResultSet, error) {
	stmt, err := Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return ex.ExecStmt(stmt)
}

// ExecStmt executes a pre-parsed statement.
func (ex *Executor) ExecStmt(stmt Statement) (*ResultSet, error) {
	switch s := stmt.(type) {
	case *CreateTableStmt: return ex.execCreateTable(s)
	case *DropTableStmt:   return ex.execDropTable(s)
	case *InsertStmt:      return ex.execInsert(s)
	case *SelectStmt:      return ex.execSelect(s)
	case *UpdateStmt:      return ex.execUpdate(s)
	case *DeleteStmt:      return ex.execDelete(s)
	default:
		return nil, fmt.Errorf("unknown statement type %T", stmt)
	}
}

// ── CREATE TABLE ──────────────────────────────────────────────────────────────

func (ex *Executor) execCreateTable(s *CreateTableStmt) (*ResultSet, error) {
	schema := &types.TableSchema{
		Name:    s.TableName,
		Columns: s.Columns,
	}
	if err := ex.schemas.CreateTable(schema); err != nil {
		return nil, err
	}
	return &ResultSet{Message: fmt.Sprintf("table %q created", s.TableName)}, nil
}

// ── DROP TABLE ────────────────────────────────────────────────────────────────

func (ex *Executor) execDropTable(s *DropTableStmt) (*ResultSet, error) {
	// First delete all rows in the table from the engine
	schema, err := ex.schemas.GetTable(s.TableName)
	if err != nil {
		return nil, err
	}

	// Collect all row keys for this table
	prefix := s.TableName + ":"
	var rowKeys [][]byte
	ex.engine.Scan(func(k, _ []byte) bool {
		if len(k) > len(prefix) {
			match := true
			for i := range prefix {
				if k[i] != prefix[i] { match = false; break }
			}
			if match { rowKeys = append(rowKeys, k) }
		}
		return true
	})

	for _, k := range rowKeys {
		ex.engine.Delete(k)
	}

	_ = schema
	if err := ex.schemas.DropTable(s.TableName); err != nil {
		return nil, err
	}
	return &ResultSet{Message: fmt.Sprintf("table %q dropped", s.TableName)}, nil
}

// ── INSERT ────────────────────────────────────────────────────────────────────

func (ex *Executor) execInsert(s *InsertStmt) (*ResultSet, error) {
	schema, err := ex.schemas.GetTable(s.TableName)
	if err != nil {
		return nil, err
	}

	row := types.NewRow(schema)

	// Map values to columns
	cols := s.Columns
	if len(cols) == 0 {
		// No column list — use schema order
		for _, c := range schema.Columns {
			cols = append(cols, c.Name)
		}
	}

	if len(cols) != len(s.Values) {
		return nil, fmt.Errorf("insert: %d columns but %d values", len(cols), len(s.Values))
	}

	for i, col := range cols {
		c, _, ok := schema.ColByName(col)
		if !ok {
			return nil, fmt.Errorf("insert: unknown column %q in table %q", col, s.TableName)
		}
		val := s.Values[i]
		// Type check (skip for SEMI and NULL)
		if !val.IsNull && c.Type != types.TypeSemi {
			if err := checkType(c, val); err != nil {
				return nil, fmt.Errorf("insert col %q: %w", col, err)
			}
		}
		row.Set(col, val)
	}

	// Validate primary key present
	pkCol, _ := schema.PrimaryKeyCol()
	if _, ok := row.Values[pkCol.Name]; !ok {
		return nil, fmt.Errorf("insert: missing primary key %q", pkCol.Name)
	}

	key, err := row.PrimaryKeyBytes()
	if err != nil { return nil, err }

	value, err := row.Encode()
	if err != nil { return nil, err }

	if err := ex.engine.Put(key, value); err != nil {
		return nil, fmt.Errorf("insert: engine put: %w", err)
	}

	return &ResultSet{Affected: 1, Message: "1 row inserted"}, nil
}

// ── SELECT ────────────────────────────────────────────────────────────────────

func (ex *Executor) execSelect(s *SelectStmt) (*ResultSet, error) {
	schema, err := ex.schemas.GetTable(s.TableName)
	if err != nil {
		return nil, err
	}

	// Determine output columns
	outCols := s.Columns
	if len(outCols) == 0 {
		outCols = schema.ColNames()
	}

	// Validate column names
	for _, c := range outCols {
		if _, _, ok := schema.ColByName(c); !ok {
			return nil, fmt.Errorf("select: unknown column %q", c)
		}
	}

	prefix := s.TableName + ":"
	prefixBytes := []byte(prefix)

	var rows [][]types.Value

	ex.engine.Scan(func(k, v []byte) bool {
		// Filter to this table's key space
		if len(k) <= len(prefixBytes) { return true }
		for i, b := range prefixBytes {
			if k[i] != b { return true }
		}

		row, err := types.DecodeRow(schema, v)
		if err != nil { return true }

		// Apply WHERE filter
		if s.Where != nil {
			pass, _ := s.Where.Eval(row.Values)
			if !pass { return true }
		}

		// Project requested columns
		var projected []types.Value
		for _, col := range outCols {
			if val, ok := row.Values[col]; ok {
				projected = append(projected, val)
			} else {
				projected = append(projected, types.NullValue)
			}
		}
		rows = append(rows, projected)
		return true
	})

	// ORDER BY
	if s.OrderBy != "" {
		colIdx := -1
		for i, c := range outCols {
			if c == s.OrderBy { colIdx = i; break }
		}
		if colIdx >= 0 {
			sort.SliceStable(rows, func(i, j int) bool {
				cmp, _ := rows[i][colIdx].Compare(rows[j][colIdx])
				if s.OrderDesc { return cmp > 0 }
				return cmp < 0
			})
		}
	}

	// LIMIT
	if s.Limit > 0 && len(rows) > s.Limit {
		rows = rows[:s.Limit]
	}

	return &ResultSet{Columns: outCols, Rows: rows}, nil
}

// ── UPDATE ────────────────────────────────────────────────────────────────────

func (ex *Executor) execUpdate(s *UpdateStmt) (*ResultSet, error) {
	schema, err := ex.schemas.GetTable(s.TableName)
	if err != nil {
		return nil, err
	}

	prefix := s.TableName + ":"
	prefixBytes := []byte(prefix)
	affected := 0

	// Collect matching keys first, then update (avoid mutating during scan)
	type pending struct {
		key []byte
		row *types.Row
	}
	var updates []pending

	ex.engine.Scan(func(k, v []byte) bool {
		if len(k) <= len(prefixBytes) { return true }
		for i, b := range prefixBytes {
			if k[i] != b { return true }
		}
		row, err := types.DecodeRow(schema, v)
		if err != nil { return true }

		if s.Where != nil {
			pass, _ := s.Where.Eval(row.Values)
			if !pass { return true }
		}
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		updates = append(updates, pending{key: keyCopy, row: row})
		return true
	})

	for _, u := range updates {
		for _, assign := range s.Assigns {
			u.row.Set(assign.Column, assign.Value)
		}
		encoded, err := u.row.Encode()
		if err != nil { continue }
		ex.engine.Put(u.key, encoded)
		affected++
	}

	return &ResultSet{
		Affected: affected,
		Message:  fmt.Sprintf("%d row(s) updated", affected),
	}, nil
}

// ── DELETE ────────────────────────────────────────────────────────────────────

func (ex *Executor) execDelete(s *DeleteStmt) (*ResultSet, error) {
	schema, err := ex.schemas.GetTable(s.TableName)
	if err != nil {
		return nil, err
	}

	prefix := s.TableName + ":"
	prefixBytes := []byte(prefix)
	var toDelete [][]byte

	ex.engine.Scan(func(k, v []byte) bool {
		if len(k) <= len(prefixBytes) { return true }
		for i, b := range prefixBytes {
			if k[i] != b { return true }
		}

		if s.Where != nil {
			row, err := types.DecodeRow(schema, v)
			if err != nil { return true }
			pass, _ := s.Where.Eval(row.Values)
			if !pass { return true }
		}
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		toDelete = append(toDelete, keyCopy)
		return true
	})

	for _, k := range toDelete {
		ex.engine.Delete(k)
	}

	return &ResultSet{
		Affected: len(toDelete),
		Message:  fmt.Sprintf("%d row(s) deleted", len(toDelete)),
	}, nil
}

// ── Type validation ───────────────────────────────────────────────────────────

func checkType(col types.Column, val types.Value) error {
	if val.Type == col.Type { return nil }
	// Allow int→float coercion
	if col.Type == types.TypeFloat && val.Type == types.TypeInt { return nil }
	return fmt.Errorf("type mismatch: column is %s but value is %s", col.Type, val.Type)
}