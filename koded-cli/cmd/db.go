package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "KodedDB interactive query shell",
	Long: `Open an interactive SQL shell connected to a KodedDB server.

  koded db                          connect to local server (127.0.0.1:6380)
  koded db --host 192.168.1.5       connect to remote server
  koded db --port 7000              use a different port
  koded db --exec "SELECT 1;"       run a single query and exit
  koded db --no-server              offline mode (no server needed)`,
	Run: runDB,
}

var (
	dbHost     string
	dbPort     string
	dbExec     string
	dbNoColor  bool
	dbNoServer bool
)

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.Flags().StringVar(&dbHost,     "host",      "127.0.0.1", "KodedDB server host")
	dbCmd.Flags().StringVar(&dbPort,     "port",      "6380",      "KodedDB server port")
	dbCmd.Flags().StringVar(&dbExec,     "exec",      "",          "Execute a single SQL statement and exit")
	dbCmd.Flags().BoolVar(&dbNoColor,    "no-color",  false,       "Disable colored output")
	dbCmd.Flags().BoolVar(&dbNoServer,   "no-server", false,       "Run in offline mode (no server connection)")
}

// ── Colors ────────────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

func color(c, s string) string {
	if dbNoColor { return s }
	return c + s + colorReset
}

// ── Wire types (mirrors api package) ──────────────────────────────────────────

type dbRequest struct {
	SQL string `json:"sql,omitempty"`
	Cmd string `json:"cmd,omitempty"`
}

type dbResponse struct {
	OK       bool       `json:"ok"`
	Columns  []string   `json:"columns,omitempty"`
	Rows     [][]string `json:"rows,omitempty"`
	Affected int        `json:"affected,omitempty"`
	Message  string     `json:"message,omitempty"`
	Error    string     `json:"error,omitempty"`
	TimeMs   float64    `json:"time_ms,omitempty"`
}

// ── TCP client ────────────────────────────────────────────────────────────────

type dbClient struct {
	conn    net.Conn
	scanner *bufio.Scanner
	writer  *bufio.Writer
	addr    string
}

func dialDB(host, port string) (*dbClient, error) {
	addr := host + ":" + port
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	c := &dbClient{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
		writer:  bufio.NewWriter(conn),
		addr:    addr,
	}
	// Read welcome banner
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	c.scanner.Scan() // discard welcome message
	c.conn.SetReadDeadline(time.Time{})
	return c, nil
}

func (c *dbClient) query(sql string) (*dbResponse, error) {
	return c.send(dbRequest{SQL: sql})
}

func (c *dbClient) cmd(command string) (*dbResponse, error) {
	return c.send(dbRequest{Cmd: command})
}

func (c *dbClient) send(req dbRequest) (*dbResponse, error) {
	data, _ := json.Marshal(req)
	c.writer.Write(data)
	c.writer.WriteByte('\n')
	if err := c.writer.Flush(); err != nil {
		return nil, err
	}
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("connection closed")
	}
	var resp dbResponse
	json.Unmarshal(c.scanner.Bytes(), &resp)
	return &resp, nil
}

func (c *dbClient) close() { c.conn.Close() }

// ── Main runner ───────────────────────────────────────────────────────────────

func runDB(cmd *cobra.Command, args []string) {
	addr := dbHost + ":" + dbPort

	// ── Single exec mode ──────────────────────────────────────────────────────
	if dbExec != "" {
		if dbNoServer {
			fmt.Println(color(colorRed, "✗ --exec requires a server connection (remove --no-server)"))
			os.Exit(1)
		}
		client, err := dialDB(dbHost, dbPort)
		if err != nil {
			fmt.Printf("  %s Cannot connect to %s: %v\n", color(colorRed, "✗"), addr, err)
			os.Exit(1)
		}
		defer client.close()

		resp, err := client.query(dbExec)
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			os.Exit(1)
		}
		fmt.Print(renderResponse(resp))
		return
	}

	// ── Interactive REPL ──────────────────────────────────────────────────────
	printBanner()

	var client *dbClient
	var offline bool

	if dbNoServer {
		offline = true
		fmt.Printf("  %s running in offline mode\n\n",
			color(colorGray, "⚠"),
		)
	} else {
		var err error
		client, err = dialDB(dbHost, dbPort)
		if err != nil {
			fmt.Printf("  %s Cannot connect to %s\n",
				color(colorRed, "✗"), color(colorCyan, addr))
			fmt.Printf("  %s Start the server with: %s\n\n",
				color(colorGray, "→"),
				color(colorCyan, "go run . --addr "+addr),
			)
			fmt.Printf("  %s Falling back to offline mode...\n\n",
				color(colorGray, "⚠"),
			)
			offline = true
		} else {
			defer client.close()
			fmt.Printf("  %s %s\n\n",
				color(colorGray, "Connected:"),
				color(colorGreen, "✓ "+addr),
			)
		}
	}

	if offline {
		fmt.Printf("  %s\n\n",
			color(colorGray, "Offline mode: only .help, .exit available — connect a server for SQL"),
		)
	}

	fmt.Printf("  %s\n\n", color(colorGray, "Type .help for commands  |  End SQL with ;  |  .exit to quit"))

	// ── Input loop ────────────────────────────────────────────────────────────
	scanner := bufio.NewScanner(os.Stdin)
	var multiLine strings.Builder
	history  := []string{}

	for {
		if multiLine.Len() == 0 {
			fmt.Print(color(colorGreen, "koded") + color(colorGray, "> "))
		} else {
			fmt.Print(color(colorGray, "   ... "))
		}

		if !scanner.Scan() {
			fmt.Println(color(colorGray, "\nBye."))
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" { continue }

		// Dot commands
		if strings.HasPrefix(line, ".") {
			if handleMeta(line, client, offline) { break }
			continue
		}

		multiLine.WriteString(line + " ")

		// Execute on semicolon
		if strings.HasSuffix(line, ";") {
			sql := strings.TrimSpace(multiLine.String())
			multiLine.Reset()
			if sql == ";" { continue }

			history = append(history, strings.TrimSuffix(sql, ";"))

			if offline {
				fmt.Printf("  %s Not connected to a server. Start with: %s\n",
					color(colorRed, "✗"),
					color(colorCyan, "go run . --addr 0.0.0.0:6380"),
				)
				continue
			}

			start := time.Now()
			resp, err := client.query(sql)
			elapsed := time.Since(start)

			if err != nil {
				// Try to reconnect once
				fmt.Printf("  %s Connection lost, reconnecting...\n", color(colorGray, "⚠"))
				client, err = dialDB(dbHost, dbPort)
				if err != nil {
					fmt.Printf("  %s Reconnect failed: %v\n", color(colorRed, "✗"), err)
					offline = true
					continue
				}
				resp, _ = client.query(sql)
			}

			fmt.Print(renderResponse(resp))
			fmt.Printf("%s\n", color(colorGray,
				fmt.Sprintf("  (%.2fms)", float64(elapsed.Microseconds())/1000.0)))
		}
	}
}

// ── Meta commands ─────────────────────────────────────────────────────────────

func handleMeta(line string, client *dbClient, offline bool) bool {
	parts := strings.Fields(line)
	switch parts[0] {
	case ".exit", ".quit", ".q":
		fmt.Println(color(colorGray, "Bye."))
		return true

	case ".tables":
		if offline || client == nil {
			fmt.Printf("  %s Not connected\n", color(colorRed, "✗"))
			return false
		}
		resp, err := client.cmd("tables")
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			return false
		}
		fmt.Print(renderResponse(resp))

	case ".ping":
		if offline || client == nil {
			fmt.Printf("  %s Not connected\n", color(colorRed, "✗"))
			return false
		}
		start := time.Now()
		resp, err := client.cmd("ping")
		rtt := time.Since(start)
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			return false
		}
		if resp.OK {
			fmt.Printf("  %s pong  %s\n",
				color(colorGreen, "✓"),
				color(colorGray, rtt.Round(time.Microsecond).String()),
			)
		}

	case ".stats":
		if offline || client == nil {
			fmt.Printf("  %s Not connected\n", color(colorRed, "✗"))
			return false
		}
		resp, err := client.cmd("stats")
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			return false
		}
		if resp.OK && resp.Message != "" {
			fmt.Printf("  %s\n", resp.Message)
		} else {
			data, _ := json.MarshalIndent(resp, "  ", "  ")
			fmt.Println(string(data))
		}

	case ".flush":
		if offline || client == nil {
			fmt.Printf("  %s Not connected\n", color(colorRed, "✗"))
			return false
		}
		resp, err := client.cmd("flush")
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			return false
		}
		fmt.Printf("  %s %s\n",
			color(colorGreen, "✓"), resp.Message)

	case ".clear":
		fmt.Print("\033[2J\033[H")
		printBanner()

	case ".help":
		printMetaHelp()

	case ".server":
		if offline || client == nil {
			fmt.Printf("  %s Not connected\n", color(colorRed, "✗"))
		} else {
			fmt.Printf("  %s %s\n",
				color(colorGray, "Server:"),
				color(colorCyan, client.addr))
		}

	default:
		fmt.Printf("  %s Unknown command %q — type %s\n",
			color(colorRed, "✗"), parts[0], color(colorCyan, ".help"))
	}
	return false
}

func printMetaHelp() {
	cmds := [][]string{
		{".tables",        "List all tables (from server)"},
		{".ping",          "Ping the server, show RTT"},
		{".stats",         "Show server + engine statistics"},
		{".flush",         "Flush memtable to disk"},
		{".server",        "Show connected server address"},
		{".clear",         "Clear the screen"},
		{".help",          "Show this help"},
		{".exit / .quit",  "Exit the shell"},
	}
	fmt.Println()
	for _, c := range cmds {
		fmt.Printf("  %-22s %s\n",
			color(colorCyan, c[0]),
			color(colorGray, c[1]),
		)
	}
	fmt.Println()
	fmt.Printf("  %s\n",
		color(colorGray, "SQL: end statements with ; — multi-line supported"),
	)
	fmt.Println()
}

// ── Response rendering ────────────────────────────────────────────────────────

func renderResponse(resp *dbResponse) string {
	c := func(col, s string) string {
		if dbNoColor { return s }
		return col + s + colorReset
	}
	var sb strings.Builder

	if !resp.OK {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c(colorRed, "✗"), resp.Error))
		return sb.String()
	}
	if resp.Message != "" && len(resp.Columns) == 0 {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c(colorGreen, "✓"), resp.Message))
		return sb.String()
	}
	if len(resp.Columns) == 0 { return "" }

	// Column widths
	widths := make([]int, len(resp.Columns))
	for i, col := range resp.Columns { widths[i] = len(col) }
	for _, row := range resp.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] { widths[i] = len(cell) }
		}
	}

	sb.WriteString("\n")
	header, divider := "  ", "  "
	for i, col := range resp.Columns {
		w := widths[i]
		header  += c(colorBold+colorCyan, fmt.Sprintf("%-*s", w, col))
		divider += strings.Repeat("─", w)
		if i < len(resp.Columns)-1 {
			header  += c(colorGray, "  │  ")
			divider += c(colorGray, "──┼──")
		}
	}
	sb.WriteString(header + "\n")
	sb.WriteString(c(colorGray, divider) + "\n")

	if len(resp.Rows) == 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", c(colorGray, "(empty)")))
	}
	for _, row := range resp.Rows {
		line := "  "
		for i, cell := range row {
			if i >= len(widths) { break }
			line += fmt.Sprintf("%-*s", widths[i], cell)
			if i < len(resp.Columns)-1 { line += c(colorGray, "  │  ") }
		}
		sb.WriteString(line + "\n")
	}
	count := fmt.Sprintf("%d row", len(resp.Rows))
	if len(resp.Rows) != 1 { count += "s" }
	sb.WriteString(fmt.Sprintf("\n  %s\n", c(colorGray, count)))
	return sb.String()
}

// ── Banner ────────────────────────────────────────────────────────────────────

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

// ── Unused but kept for offline fallback persistence ──────────────────────────

type persistedState struct {
	Tables map[string]*offlineTable `json:"tables"`
}
type offlineTable struct {
	Schema []string            `json:"schema"`
	Rows   []map[string]string `json:"rows"`
}

func loadOfflineState(file string) map[string]*offlineTable {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "koded.repl.json"))
	if err != nil { return make(map[string]*offlineTable) }
	var state persistedState
	json.Unmarshal(data, &state)
	if state.Tables == nil { return make(map[string]*offlineTable) }
	return state.Tables
}