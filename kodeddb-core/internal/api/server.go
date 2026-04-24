package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Koded0214h/kodeddb-core/internal/engine"
	"github.com/Koded0214h/kodeddb-core/internal/query"
	"github.com/Koded0214h/kodeddb-core/internal/storage"
)

/*
KodedDB TCP Query Server

Protocol (newline-delimited JSON over TCP):

  Client → Server:  {"sql": "SELECT * FROM users;"}
  Server → Client:  {"ok": true,  "columns": [...], "rows": [[...]], "affected": 0, "time_ms": 1}
  Server → Client:  {"ok": false, "error": "table not found"}

  Special commands (not SQL):
  Client → Server:  {"cmd": "ping"}
  Server → Client:  {"ok": true, "message": "pong"}

  Client → Server:  {"cmd": "stats"}
  Server → Client:  {"ok": true, "stats": {...}}

One request per line. Server keeps connection open (persistent).
Multiple clients can connect simultaneously.
*/

const defaultPort = "6380" // KodedDB default port

// ── Wire types ────────────────────────────────────────────────────────────────

type Request struct {
	SQL string `json:"sql,omitempty"`
	Cmd string `json:"cmd,omitempty"`
}

type Response struct {
	OK       bool            `json:"ok"`
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]string      `json:"rows,omitempty"`
	Affected int             `json:"affected,omitempty"`
	Message  string          `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
	TimeMs   float64         `json:"time_ms,omitempty"`
	Stats    *engine.Stats   `json:"stats,omitempty"`
}

// ── Server ────────────────────────────────────────────────────────────────────

type Server struct {
	listener net.Listener
	engine   *engine.Engine
	schemas  *storage.SchemaStore
	executor *query.Executor
	mu       sync.Mutex
	clients  int
	quit     chan struct{}
}

type Options struct {
	DataFile string
	WALFile  string
	Addr     string // host:port, default "0.0.0.0:6380"
}

func NewServer(opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = "0.0.0.0:" + defaultPort
	}

	// Open engine
	eng, err := engine.Open(engine.Options{
		DataFile: opts.DataFile,
		WALFile:  opts.WALFile,
	})
	if err != nil {
		return nil, fmt.Errorf("server: open engine: %w", err)
	}

	// Open schema store
	ss, err := storage.NewSchemaStore(eng)
	if err != nil {
		return nil, fmt.Errorf("server: open schema store: %w", err)
	}

	// Build executor
	exec := query.NewExecutor(eng, ss)

	// Start listening
	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("server: listen %s: %w", opts.Addr, err)
	}

	return &Server{
		listener: ln,
		engine:   eng,
		schemas:  ss,
		executor: exec,
		quit:     make(chan struct{}),
	}, nil
}

// Run accepts connections until Close() is called.
func (s *Server) Run() error {
	fmt.Printf("[kodeddb] listening on %s\n", s.listener.Addr())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil // clean shutdown
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		s.mu.Lock()
		s.clients++
		s.mu.Unlock()
		go s.handleConn(conn)
	}
}

func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	close(s.quit)
	if err := s.listener.Close(); err != nil {
		return err
	}
	return s.engine.Close()
}

// ── Connection handler ────────────────────────────────────────────────────────

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		s.clients--
		s.mu.Unlock()
	}()

	remote := conn.RemoteAddr().String()
	fmt.Printf("[kodeddb] client connected: %s\n", remote)

	scanner := bufio.NewScanner(conn)
	writer  := bufio.NewWriter(conn)

	// Send welcome banner
	s.writeResponse(writer, Response{
		OK:      true,
		Message: "KodedDB ready — send {\"sql\": \"...\"} or {\"cmd\": \"ping\"}",
	})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeResponse(writer, Response{
				OK:    false,
				Error: fmt.Sprintf("invalid request JSON: %v", err),
			})
			continue
		}

		resp := s.handle(req)
		s.writeResponse(writer, resp)
	}

	fmt.Printf("[kodeddb] client disconnected: %s\n", remote)
}

func (s *Server) writeResponse(w *bufio.Writer, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		data = []byte(`{"ok":false,"error":"internal marshal error"}`)
	}
	w.Write(data)
	w.WriteByte('\n')
	w.Flush()
}

// ── Request dispatch ──────────────────────────────────────────────────────────

func (s *Server) handle(req Request) Response {
	// Special commands
	if req.Cmd != "" {
		return s.handleCmd(req.Cmd)
	}

	if req.SQL == "" {
		return Response{OK: false, Error: "request must have 'sql' or 'cmd' field"}
	}

	// SQL execution
	start := time.Now()
	rs, err := s.executor.Exec(req.SQL)
	elapsed := time.Since(start)

	if err != nil {
		return Response{
			OK:     false,
			Error:  err.Error(),
			TimeMs: float64(elapsed.Microseconds()) / 1000.0,
		}
	}

	// Convert ResultSet → Response
	resp := Response{
		OK:       true,
		Columns:  rs.Columns,
		Affected: rs.Affected,
		Message:  rs.Message,
		TimeMs:   float64(elapsed.Microseconds()) / 1000.0,
	}

	// Convert [][]types.Value → [][]string for JSON
	for _, row := range rs.Rows {
		var strRow []string
		for _, val := range row {
			strRow = append(strRow, val.String())
		}
		resp.Rows = append(resp.Rows, strRow)
	}

	return resp
}

func (s *Server) handleCmd(cmd string) Response {
	switch strings.ToLower(cmd) {
	case "ping":
		return Response{OK: true, Message: "pong"}

	case "stats":
		stats := s.engine.Stats()
		s.mu.Lock()
		clients := s.clients
		s.mu.Unlock()
		_ = clients
		return Response{OK: true, Stats: &stats}

	case "tables":
		tables := s.schemas.ListTables()
		var rows [][]string
		for _, t := range tables {
			rows = append(rows, []string{t})
		}
		return Response{
			OK:      true,
			Columns: []string{"table_name"},
			Rows:    rows,
		}

	case "flush":
		if err := s.engine.Flush(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Message: "flushed memtable to disk"}

	default:
		return Response{
			OK:    false,
			Error: fmt.Sprintf("unknown command %q — supported: ping, stats, tables, flush", cmd),
		}
	}
}