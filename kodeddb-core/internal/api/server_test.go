package api

import (
	"fmt"
	"testing"
	"time"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

func newTestServer(t *testing.T) (*Server, *Client) {
	t.Helper()
	dir := t.TempDir()

	srv, err := NewServer(Options{
		DataFile: dir + "/koded.db",
		WALFile:  dir + "/koded.wal",
		Addr:     "127.0.0.1:0", // OS picks a free port
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Run server in background
	go func() {
		if err := srv.Run(); err != nil {
			// Expected on Close()
		}
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	client, err := Dial(srv.Addr())
	if err != nil {
		srv.Close()
		t.Fatalf("Dial: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
		srv.Close()
	})

	return srv, client
}

// ── Server tests ──────────────────────────────────────────────────────────────

func TestServerPing(t *testing.T) {
	_, client := newTestServer(t)
	resp, err := client.Ping()
	if err != nil { t.Fatalf("Ping: %v", err) }
	if !resp.OK { t.Errorf("expected OK, got error: %s", resp.Error) }
	if resp.Message != "pong" { t.Errorf("expected 'pong', got %q", resp.Message) }
}

func TestServerStats(t *testing.T) {
	_, client := newTestServer(t)
	resp, err := client.Stats()
	if err != nil { t.Fatalf("Stats: %v", err) }
	if !resp.OK { t.Errorf("expected OK") }
	if resp.Stats == nil { t.Error("expected stats payload") }
}

func TestServerCreateAndSelect(t *testing.T) {
	_, client := newTestServer(t)

	// Create table
	resp, err := client.Query("CREATE TABLE users (id INT PRIMARY KEY, name TEXT)")
	if err != nil { t.Fatalf("create: %v", err) }
	if !resp.OK { t.Fatalf("create failed: %s", resp.Error) }

	// Insert
	resp, err = client.Query("INSERT INTO users VALUES (1, 'koded')")
	if err != nil { t.Fatalf("insert: %v", err) }
	if !resp.OK { t.Fatalf("insert failed: %s", resp.Error) }
	if resp.Affected != 1 { t.Errorf("expected affected=1") }

	// Select
	resp, err = client.Query("SELECT * FROM users")
	if err != nil { t.Fatalf("select: %v", err) }
	if !resp.OK { t.Fatalf("select failed: %s", resp.Error) }
	if len(resp.Rows) != 1 { t.Fatalf("expected 1 row, got %d", len(resp.Rows)) }
	if resp.Rows[0][0] != "1" { t.Errorf("wrong id: %s", resp.Rows[0][0]) }
	if resp.Rows[0][1] != "koded" { t.Errorf("wrong name: %s", resp.Rows[0][1]) }
}

func TestServerWhere(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE items (id INT PRIMARY KEY, val INT)")
	for i := 1; i <= 5; i++ {
		client.Query(fmt.Sprintf("INSERT INTO items VALUES (%d, %d)", i, i*10))
	}

	resp, err := client.Query("SELECT * FROM items WHERE val > 20")
	if err != nil { t.Fatalf("select: %v", err) }
	if !resp.OK { t.Fatalf("select failed: %s", resp.Error) }
	if len(resp.Rows) != 3 {
		t.Errorf("expected 3 rows with val > 20, got %d", len(resp.Rows))
	}
}

func TestServerDelete(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE logs (id INT PRIMARY KEY, level TEXT)")
	client.Query("INSERT INTO logs VALUES (1, 'info')")
	client.Query("INSERT INTO logs VALUES (2, 'error')")
	client.Query("INSERT INTO logs VALUES (3, 'info')")

	resp, err := client.Query("DELETE FROM logs WHERE level = 'info'")
	if err != nil { t.Fatalf("delete: %v", err) }
	if !resp.OK { t.Fatalf("delete failed: %s", resp.Error) }
	if resp.Affected != 2 { t.Errorf("expected 2 deleted, got %d", resp.Affected) }

	resp2, _ := client.Query("SELECT * FROM logs")
	if len(resp2.Rows) != 1 { t.Errorf("expected 1 remaining row, got %d", len(resp2.Rows)) }
}

func TestServerUpdate(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE p (id INT PRIMARY KEY, status TEXT)")
	client.Query("INSERT INTO p VALUES (1, 'pending')")
	client.Query("INSERT INTO p VALUES (2, 'pending')")

	resp, err := client.Query("UPDATE p SET status = 'done' WHERE status = 'pending'")
	if err != nil { t.Fatalf("update: %v", err) }
	if !resp.OK { t.Fatalf("update failed: %s", resp.Error) }
	if resp.Affected != 2 { t.Errorf("expected 2 updated, got %d", resp.Affected) }
}

func TestServerDropTable(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE temp (id INT PRIMARY KEY)")
	client.Query("INSERT INTO temp VALUES (1)")

	resp, err := client.Query("DROP TABLE temp")
	if err != nil { t.Fatalf("drop: %v", err) }
	if !resp.OK { t.Fatalf("drop failed: %s", resp.Error) }

	resp2, _ := client.Query("SELECT * FROM temp")
	if resp2.OK { t.Error("expected error querying dropped table") }
}

func TestServerTablesCmd(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE a (id INT PRIMARY KEY)")
	client.Query("CREATE TABLE b (id INT PRIMARY KEY)")

	resp, err := client.Tables()
	if err != nil { t.Fatalf("tables: %v", err) }
	if !resp.OK { t.Fatalf("tables failed: %s", resp.Error) }
	if len(resp.Rows) != 2 { t.Errorf("expected 2 tables, got %d", len(resp.Rows)) }
}

func TestServerFlushCmd(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE x (id INT PRIMARY KEY)")
	resp, err := client.Flush()
	if err != nil { t.Fatalf("flush: %v", err) }
	if !resp.OK { t.Fatalf("flush failed: %s", resp.Error) }
}

func TestServerInvalidJSON(t *testing.T) {
	_, client := newTestServer(t)
	// Send raw invalid JSON
	resp, err := client.send(Request{}) // empty request
	if err != nil { t.Fatalf("send: %v", err) }
	if resp.OK { t.Error("expected error for empty request") }
}

func TestServerTimingReported(t *testing.T) {
	_, client := newTestServer(t)
	client.Query("CREATE TABLE t (id INT PRIMARY KEY)")
	resp, _ := client.Query("SELECT * FROM t")
	if resp.TimeMs < 0 { t.Error("expected non-negative time_ms") }
}

func TestServerMultipleClients(t *testing.T) {
	srv, _ := newTestServer(t)

	// Connect two clients simultaneously
	c1, err := Dial(srv.Addr())
	if err != nil { t.Fatalf("c1 dial: %v", err) }
	defer c1.Close()

	c2, err := Dial(srv.Addr())
	if err != nil { t.Fatalf("c2 dial: %v", err) }
	defer c2.Close()

	// Both can operate independently
	c1.Query("CREATE TABLE shared (id INT PRIMARY KEY, val TEXT)")
	c1.Query("INSERT INTO shared VALUES (1, 'from-c1')")
	c2.Query("INSERT INTO shared VALUES (2, 'from-c2')")

	resp, _ := c1.Query("SELECT * FROM shared")
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 rows from both clients, got %d", len(resp.Rows))
	}
}

func TestRenderTable(t *testing.T) {
	resp := &Response{
		OK:      true,
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "koded"}, {"2", "shaz"}},
	}
	out := RenderTable(resp, true) // no-color for testability
	if out == "" { t.Error("expected non-empty render") }
	if !containsStr(out, "koded") { t.Error("expected 'koded' in output") }
	if !containsStr(out, "2 rows") { t.Error("expected row count") }
}

func TestRenderError(t *testing.T) {
	resp := &Response{OK: false, Error: "table not found"}
	out := RenderTable(resp, true)
	if !containsStr(out, "table not found") { t.Error("expected error in render") }
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub { return true }
			}
			return false
		}())
}