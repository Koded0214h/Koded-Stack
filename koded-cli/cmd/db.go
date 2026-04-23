package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "KodedDB interactive query shell",
	Long: `Open an interactive SQL shell connected to a KodedDB database.

  koded db                      open default database (~/.koded/data/koded.db)
  koded db --file ./mydb.db    open a specific database file
  koded db --exec "SELECT 1"   run a single query and exit`,
	Run: runDB,
}

var (
	dbFile    string
	dbExec    string
	dbNoColor bool
)

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.Flags().StringVar(&dbFile,    "file",     "", "Path to database file")
	dbCmd.Flags().StringVar(&dbExec,    "exec",     "", "Execute a single SQL statement and exit")
	dbCmd.Flags().BoolVar(&dbNoColor,   "no-color", false, "Disable colored output")
}

// ── Colors ────────────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func color(c, s string) string {
	if dbNoColor { return s }
	return c + s + colorReset
}

// ── Session ───────────────────────────────────────────────────────────────────

type dbSession struct {
	file    string
	tables  map[string]*inMemTable
	history []string
}

type inMemTable struct {
	Schema []string            `json:"schema"`
	Rows   []map[string]string `json:"rows"`
}

func newSession(file string) *dbSession {
	s := &dbSession{file: file, tables: make(map[string]*inMemTable)}
	s.loadState()
	return s
}

// ── Main runner ───────────────────────────────────────────────────────────────

func runDB(cmd *cobra.Command, args []string) {
	if dbFile == "" {
		home, _ := os.UserHomeDir()
		dbFile = filepath.Join(home, ".koded", "data", "koded.db")
	}
	os.MkdirAll(filepath.Dir(dbFile), 0755)

	sess := newSession(dbFile)

	if dbExec != "" {
		result := sess.exec(dbExec)
		fmt.Print(result.render(dbNoColor))
		return
	}

	printBanner()
	fmt.Printf("  %s %s\n",   color(colorGray, "Database:"), color(colorCyan, dbFile))
	fmt.Printf("  %s\n\n",    color(colorGray, "Type .help for commands, .exit to quit"))

	scanner := bufio.NewScanner(os.Stdin)
	var multiLine strings.Builder

	for {
		if multiLine.Len() == 0 {
			fmt.Print(color(colorGreen, "koded") + color(colorGray, "> "))
		} else {
			fmt.Print(color(colorGray, "   ... "))
		}

		if !scanner.Scan() { fmt.Println(color(colorGray, "\nBye.")); break }
		line := strings.TrimSpace(scanner.Text())
		if line == "" { continue }

		if strings.HasPrefix(line, ".") {
			if handleMeta(line, sess) { break }
			continue
		}

		multiLine.WriteString(line + " ")

		if strings.HasSuffix(line, ";") {
			sql := strings.TrimSpace(multiLine.String())
			multiLine.Reset()
			if sql == ";" { continue }

			start  := time.Now()
			result := sess.exec(sql)
			elapsed := time.Since(start)

			fmt.Print(result.render(dbNoColor))
			fmt.Printf("%s\n", color(colorGray, fmt.Sprintf("  (%s)", elapsed.Round(time.Microsecond))))
		}
	}
}

// ── Meta commands ─────────────────────────────────────────────────────────────

func handleMeta(line string, sess *dbSession) bool {
	parts := strings.Fields(line)
	switch parts[0] {
	case ".exit", ".quit", ".q":
		fmt.Println(color(colorGray, "Bye."))
		return true
	case ".tables":
		if len(sess.tables) == 0 {
			fmt.Println(color(colorGray, "  (no tables)"))
		} else {
			for name := range sess.tables {
				fmt.Printf("  %s\n", color(colorCyan, name))
			}
		}
	case ".schema":
		if len(parts) < 2 {
			fmt.Println(color(colorRed, "  Usage: .schema <table>"))
		} else {
			if t, ok := sess.tables[parts[1]]; ok {
				fmt.Printf("  %s (%s)\n", color(colorCyan, parts[1]), strings.Join(t.Schema, ", "))
			} else {
				fmt.Printf("  %s\n", color(colorRed, "table '"+parts[1]+"' not found"))
			}
		}
	case ".clear":
		fmt.Print("\033[2J\033[H")
		printBanner()
	case ".history":
		for i, h := range sess.history {
			fmt.Printf("  %s  %s\n", color(colorGray, fmt.Sprintf("%3d", i+1)), h)
		}
	case ".file":
		fmt.Printf("  %s %s\n", color(colorGray, "File:"), color(colorCyan, sess.file))
	case ".help":
		printMetaHelp()
	default:
		fmt.Printf("  %s Unknown command %q. Type %s for help.\n",
			color(colorRed, "✗"), parts[0], color(colorCyan, ".help"))
	}
	return false
}

func printMetaHelp() {
	cmds := [][]string{
		{".tables",        "List all tables"},
		{".schema <table>","Show schema for a table"},
		{".file",          "Show current database file"},
		{".history",       "Show command history"},
		{".clear",         "Clear the screen"},
		{".help",          "Show this help"},
		{".exit / .quit",  "Exit the shell"},
	}
	fmt.Println()
	for _, c := range cmds {
		fmt.Printf("  %-22s %s\n", color(colorCyan, c[0]), color(colorGray, c[1]))
	}
	fmt.Println()
}

// ── Result rendering ──────────────────────────────────────────────────────────

type QueryResult struct {
	Columns  []string
	Rows     [][]string
	Affected int
	Message  string
	IsError  bool
}

func (r *QueryResult) render(noColor bool) string {
	c := func(col, s string) string {
		if noColor { return s }
		return col + s + colorReset
	}
	var sb strings.Builder

	if r.IsError {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c(colorRed, "✗"), r.Message))
		return sb.String()
	}
	if r.Message != "" && len(r.Columns) == 0 {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c(colorGreen, "✓"), r.Message))
		return sb.String()
	}
	if len(r.Columns) == 0 { return "" }

	// Column widths
	widths := make([]int, len(r.Columns))
	for i, col := range r.Columns { widths[i] = len(col) }
	for _, row := range r.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] { widths[i] = len(cell) }
		}
	}

	// Header
	sb.WriteString("\n")
	header, divider := "  ", "  "
	for i, col := range r.Columns {
		w := widths[i]
		header  += c(colorBold+colorCyan, fmt.Sprintf("%-*s", w, col))
		divider += strings.Repeat("─", w)
		if i < len(r.Columns)-1 {
			header  += c(colorGray, "  │  ")
			divider += c(colorGray, "──┼──")
		}
	}
	sb.WriteString(header + "\n")
	sb.WriteString(c(colorGray, divider) + "\n")

	if len(r.Rows) == 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", c(colorGray, "(empty)")))
	}
	for _, row := range r.Rows {
		line := "  "
		for i, cell := range row {
			if i >= len(widths) { break }
			line += fmt.Sprintf("%-*s", widths[i], cell)
			if i < len(r.Columns)-1 { line += c(colorGray, "  │  ") }
		}
		sb.WriteString(line + "\n")
	}
	count := fmt.Sprintf("%d row", len(r.Rows))
	if len(r.Rows) != 1 { count += "s" }
	sb.WriteString(fmt.Sprintf("\n  %s\n", c(colorGray, count)))
	return sb.String()
}

// ── SQL executor ──────────────────────────────────────────────────────────────

func (s *dbSession) exec(sql string) *QueryResult {
	s.history = append(s.history, strings.TrimRight(sql, ";"))
	sql   = strings.TrimRight(strings.TrimSpace(sql), ";")
	upper := strings.ToUpper(sql)

	switch {
	case strings.HasPrefix(upper, "SELECT 1"):
		return &QueryResult{Columns: []string{"1"}, Rows: [][]string{{"1"}}}
	case strings.HasPrefix(upper, "CREATE TABLE"):
		return s.execCreate(sql)
	case strings.HasPrefix(upper, "DROP TABLE"):
		return s.execDrop(sql)
	case strings.HasPrefix(upper, "INSERT INTO"):
		return s.execInsert(sql)
	case strings.HasPrefix(upper, "SELECT"):
		return s.execSelect(sql)
	case strings.HasPrefix(upper, "DELETE FROM"):
		return s.execDelete(sql)
	case strings.HasPrefix(upper, "SHOW TABLES"):
		return s.execShowTables()
	default:
		return &QueryResult{IsError: true,
			Message: "unsupported statement — connect kodeddb-core for full SQL support"}
	}
}

func (s *dbSession) execCreate(sql string) *QueryResult {
	upper := strings.ToUpper(sql)
	after := strings.TrimSpace(sql[strings.Index(upper, "TABLE")+5:])
	paren := strings.Index(after, "(")
	if paren < 0 { return errResult("invalid CREATE TABLE syntax") }
	tableName := strings.TrimSpace(after[:paren])
	colsDef   := after[paren+1 : strings.LastIndex(after, ")")]
	if _, exists := s.tables[tableName]; exists {
		return errResult(fmt.Sprintf("table %q already exists", tableName))
	}
	var cols []string
	for _, part := range strings.Split(colsDef, ",") {
		if f := strings.Fields(strings.TrimSpace(part)); len(f) >= 1 {
			cols = append(cols, f[0])
		}
	}
	s.tables[tableName] = &inMemTable{Schema: cols}
	s.persistState()
	return &QueryResult{Message: fmt.Sprintf("table %q created", tableName)}
}

func (s *dbSession) execDrop(sql string) *QueryResult {
	upper := strings.ToUpper(sql)
	after := strings.TrimSpace(sql[strings.Index(upper, "TABLE")+5:])
	name  := strings.Fields(after)[0]
	if _, ok := s.tables[name]; !ok { return errResult(fmt.Sprintf("table %q not found", name)) }
	delete(s.tables, name)
	s.persistState()
	return &QueryResult{Message: fmt.Sprintf("table %q dropped", name)}
}

func (s *dbSession) execInsert(sql string) *QueryResult {
	upper := strings.ToUpper(sql)
	after := strings.TrimSpace(sql[strings.Index(upper, "INTO")+4:])
	parts := strings.Fields(after)
	if len(parts) == 0 { return errResult("invalid INSERT syntax") }
	tableName := parts[0]
	tbl, ok := s.tables[tableName]
	if !ok { return errResult(fmt.Sprintf("table %q not found", tableName)) }

	valIdx := strings.Index(strings.ToUpper(after), "VALUES")
	if valIdx < 0 { return errResult("missing VALUES keyword") }
	vals := parseValueList(after[valIdx+6:])

	row := make(map[string]string)
	for i, col := range tbl.Schema {
		if i < len(vals) { row[col] = vals[i] } else { row[col] = "NULL" }
	}
	tbl.Rows = append(tbl.Rows, row)
	s.persistState()
	return &QueryResult{Message: "1 row inserted", Affected: 1}
}

func (s *dbSession) execSelect(sql string) *QueryResult {
	upper   := strings.ToUpper(sql)
	fromIdx := strings.Index(upper, "FROM")
	if fromIdx < 0 { return errResult("missing FROM clause") }
	afterFrom := strings.TrimSpace(sql[fromIdx+4:])
	tableName  := strings.Fields(afterFrom)[0]
	tbl, ok := s.tables[tableName]
	if !ok { return errResult(fmt.Sprintf("table %q not found", tableName)) }

	between := strings.TrimSpace(sql[6:fromIdx])
	var cols []string
	if strings.TrimSpace(between) == "*" {
		cols = tbl.Schema
	} else {
		for _, c := range strings.Split(between, ",") { cols = append(cols, strings.TrimSpace(c)) }
	}

	filterCol, filterVal := "", ""
	if whereIdx := strings.Index(upper, "WHERE"); whereIdx >= 0 {
		wherePart := strings.TrimSpace(sql[whereIdx+5:])
		for _, kw := range []string{"LIMIT", "ORDER"} {
			if idx := strings.Index(strings.ToUpper(wherePart), kw); idx >= 0 {
				wherePart = wherePart[:idx]
			}
		}
		if eqIdx := strings.Index(wherePart, "="); eqIdx >= 0 {
			filterCol = strings.TrimSpace(wherePart[:eqIdx])
			filterVal = strings.Trim(strings.TrimSpace(wherePart[eqIdx+1:]), "'\"")
		}
	}

	limit := -1
	if limitIdx := strings.Index(upper, "LIMIT"); limitIdx >= 0 {
		fmt.Sscanf(strings.TrimSpace(sql[limitIdx+5:]), "%d", &limit)
	}

	var rows [][]string
	for _, row := range tbl.Rows {
		if filterCol != "" {
			if v, ok := row[filterCol]; !ok || v != filterVal { continue }
		}
		var cells []string
		for _, col := range cols { cells = append(cells, row[col]) }
		rows = append(rows, cells)
		if limit > 0 && len(rows) >= limit { break }
	}
	return &QueryResult{Columns: cols, Rows: rows}
}

func (s *dbSession) execDelete(sql string) *QueryResult {
	upper  := strings.ToUpper(sql)
	after  := strings.TrimSpace(sql[strings.Index(upper, "FROM")+4:])
	name   := strings.Fields(after)[0]
	tbl, ok := s.tables[name]
	if !ok { return errResult(fmt.Sprintf("table %q not found", name)) }

	whereIdx := strings.Index(strings.ToUpper(after), "WHERE")
	if whereIdx < 0 {
		n := len(tbl.Rows); tbl.Rows = nil; s.persistState()
		return &QueryResult{Message: fmt.Sprintf("%d row(s) deleted", n), Affected: n}
	}
	wherePart := strings.TrimSpace(after[whereIdx+5:])
	eqIdx := strings.Index(wherePart, "=")
	if eqIdx < 0 { return errResult("unsupported WHERE in DELETE") }
	fc := strings.TrimSpace(wherePart[:eqIdx])
	fv := strings.Trim(strings.TrimSpace(wherePart[eqIdx+1:]), "'\"")

	var kept []map[string]string
	deleted := 0
	for _, row := range tbl.Rows {
		if row[fc] == fv { deleted++ } else { kept = append(kept, row) }
	}
	tbl.Rows = kept; s.persistState()
	return &QueryResult{Message: fmt.Sprintf("%d row(s) deleted", deleted), Affected: deleted}
}

func (s *dbSession) execShowTables() *QueryResult {
	var rows [][]string
	for name := range s.tables { rows = append(rows, []string{name}) }
	return &QueryResult{Columns: []string{"table_name"}, Rows: rows}
}

// ── Persistence ───────────────────────────────────────────────────────────────

type persistedState struct {
	Tables map[string]*inMemTable `json:"tables"`
}

func (s *dbSession) persistState() {
	data, err := json.MarshalIndent(persistedState{Tables: s.tables}, "", "  ")
	if err != nil { return }
	os.WriteFile(s.file+".repl.json", data, 0644)
}

func (s *dbSession) loadState() {
	data, err := os.ReadFile(s.file + ".repl.json")
	if err != nil { return }
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil { return }
	if state.Tables != nil { s.tables = state.Tables }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func errResult(msg string) *QueryResult {
	return &QueryResult{IsError: true, Message: msg}
}

func parseValueList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	var vals []string
	for _, v := range strings.Split(s, ",") {
		vals = append(vals, strings.Trim(strings.TrimSpace(v), "'\""))
	}
	return vals
}

func printBanner() {
	fmt.Print(color(colorGreen, `
  ██╗  ██╗ ██████╗ ██████╗ ███████╗██████╗     ██████╗ ██████╗ 
  ██║ ██╔╝██╔═══██╗██╔══██╗██╔════╝██╔══██╗    ██╔══██╗██╔══██╗
  █████╔╝ ██║   ██║██║  ██║█████╗  ██║  ██║    ██║  ██║██████╔╝
  ██╔═██╗ ██║   ██║██║  ██║██╔══╝  ██║  ██║    ██║  ██║██╔══██╗
  ██║  ██╗╚██████╔╝██████╔╝███████╗██████╔╝    ██████╔╝██████╔╝
  ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚═════╝     ╚═════╝ ╚═════╝ 
`))
}