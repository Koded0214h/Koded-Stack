package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

/*
KodedDB TCP client.
Used by koded-cli's db.go to connect to a running kodeddb-core server.

Usage:
  client, err := api.Dial("127.0.0.1:6380")
  defer client.Close()

  rs, err := client.Query("SELECT * FROM users;")
  rs, err := client.Ping()
  rs, err := client.Stats()
*/

type Client struct {
	conn    net.Conn
	scanner *bufio.Scanner
	writer  *bufio.Writer
	addr    string
}

// Dial connects to a KodedDB server and reads the welcome message.
func Dial(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("client: dial %s: %w", addr, err)
	}

	c := &Client{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
		writer:  bufio.NewWriter(conn),
		addr:    addr,
	}

	// Read and discard welcome banner
	if _, err := c.readResponse(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("client: welcome: %w", err)
	}

	return c, nil
}

// Query executes a SQL string and returns the response.
func (c *Client) Query(sql string) (*Response, error) {
	return c.send(Request{SQL: sql})
}

// Ping checks if the server is alive.
func (c *Client) Ping() (*Response, error) {
	return c.send(Request{Cmd: "ping"})
}

// Stats returns server statistics.
func (c *Client) Stats() (*Response, error) {
	return c.send(Request{Cmd: "stats"})
}

// Tables returns all table names from the server.
func (c *Client) Tables() (*Response, error) {
	return c.send(Request{Cmd: "tables"})
}

// Flush forces a memtable flush to disk.
func (c *Client) Flush() (*Response, error) {
	return c.send(Request{Cmd: "flush"})
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Addr() string {
	return c.addr
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (c *Client) send(req Request) (*Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal: %w", err)
	}

	c.writer.Write(data)
	c.writer.WriteByte('\n')
	if err := c.writer.Flush(); err != nil {
		return nil, fmt.Errorf("client: send: %w", err)
	}

	return c.readResponse()
}

func (c *Client) readResponse() (*Response, error) {
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("client: read: %w", err)
		}
		return nil, fmt.Errorf("client: connection closed")
	}

	var resp Response
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("client: parse response: %w", err)
	}
	return &resp, nil
}

// ── Formatting helpers (used by koded-cli) ────────────────────────────────────

// RenderTable formats a Response as an aligned text table.
// Same layout as the REPL's render() method so output is consistent.
func RenderTable(resp *Response, noColor bool) string {
	c := func(col, s string) string {
		if noColor { return s }
		return col + s + "\033[0m"
	}

	var sb strings.Builder

	if !resp.OK {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c("\033[31m", "✗"), resp.Error))
		return sb.String()
	}

	if resp.Message != "" && len(resp.Columns) == 0 {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c("\033[32m", "✓"), resp.Message))
		return sb.String()
	}

	if len(resp.Columns) == 0 {
		return ""
	}

	// Column widths
	widths := make([]int, len(resp.Columns))
	for i, col := range resp.Columns { widths[i] = len(col) }
	for _, row := range resp.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] { widths[i] = len(cell) }
		}
	}

	// Header
	sb.WriteString("\n")
	header, divider := "  ", "  "
	for i, col := range resp.Columns {
		w := widths[i]
		header  += c("\033[1m\033[36m", fmt.Sprintf("%-*s", w, col))
		divider += strings.Repeat("─", w)
		if i < len(resp.Columns)-1 {
			header  += c("\033[90m", "  │  ")
			divider += c("\033[90m", "──┼──")
		}
	}
	sb.WriteString(header + "\n")
	sb.WriteString(c("\033[90m", divider) + "\n")

	if len(resp.Rows) == 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", c("\033[90m", "(empty)")))
	}
	for _, row := range resp.Rows {
		line := "  "
		for i, cell := range row {
			if i >= len(widths) { break }
			line += fmt.Sprintf("%-*s", widths[i], cell)
			if i < len(resp.Columns)-1 { line += c("\033[90m", "  │  ") }
		}
		sb.WriteString(line + "\n")
	}

	count := fmt.Sprintf("%d row", len(resp.Rows))
	if len(resp.Rows) != 1 { count += "s" }
	sb.WriteString(fmt.Sprintf("\n  %s\n", c("\033[90m", count)))
	return sb.String()
}