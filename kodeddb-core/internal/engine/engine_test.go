package engine

import (
	"fmt"
	"testing"

	"github.com/Koded0214h/kodeddb-core/internal/storage"
)

// ── BTree tests ───────────────────────────────────────────────────────────

func newTestBTree(t *testing.T) (*BTree, *storage.Pager) {
	t.Helper()
	tmp := t.TempDir() + "/btree.kdb"
	pager, err := storage.NewPager(tmp)
	if err != nil {
		t.Fatalf("NewPager: %v", err)
	}
	bt, err := NewBTree(pager)
	if err != nil {
		t.Fatalf("NewBTree: %v", err)
	}
	return bt, pager
}

func TestBTreePutGet(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	bt.Put([]byte("name"), []byte("kodedDB"))
	val, err := bt.Get([]byte("name"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "kodedDB" {
		t.Errorf("expected 'kodedDB', got %q", val)
	}
}

func TestBTreeUpdate(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	bt.Put([]byte("k"), []byte("v1"))
	bt.Put([]byte("k"), []byte("v2"))
	val, err := bt.Get([]byte("k"))
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if string(val) != "v2" {
		t.Errorf("expected 'v2', got %q", val)
	}
}

func TestBTreeDelete(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	bt.Put([]byte("del"), []byte("gone"))
	if err := bt.Delete([]byte("del")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := bt.Get([]byte("del"))
	if err != storage.ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestBTreeManyKeys(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	n := 200
	for i := 0; i < n; i++ {
		k := []byte(fmt.Sprintf("key:%06d", i))
		v := []byte(fmt.Sprintf("val:%06d", i))
		if err := bt.Put(k, v); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	for i := 0; i < n; i++ {
		k := []byte(fmt.Sprintf("key:%06d", i))
		v := []byte(fmt.Sprintf("val:%06d", i))
		got, err := bt.Get(k)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if string(got) != string(v) {
			t.Errorf("key %d: expected %q got %q", i, v, got)
		}
	}
}

func TestBTreeRangeScan(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	for i := 0; i < 20; i++ {
		k := []byte(fmt.Sprintf("%03d", i))
		bt.Put(k, []byte(fmt.Sprintf("v%d", i)))
	}

	var found []string
	bt.RangeScan([]byte("005"), []byte("010"), func(e BTreeEntry) bool {
		found = append(found, string(e.Key))
		return true
	})

	if len(found) != 5 {
		t.Errorf("expected 5 results in range, got %d: %v", len(found), found)
	}
	if found[0] != "005" || found[4] != "009" {
		t.Errorf("wrong range bounds: %v", found)
	}
}

func TestBTreeFullScan(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	n := 50
	for i := 0; i < n; i++ {
		k := []byte(fmt.Sprintf("k:%04d", i))
		bt.Put(k, []byte("v"))
	}

	var count int
	bt.FullScan(func(e BTreeEntry) bool {
		count++
		return true
	})
	if count != n {
		t.Errorf("expected %d entries in full scan, got %d", n, count)
	}
}

func TestBTreeSplit(t *testing.T) {
	bt, pager := newTestBTree(t)
	defer pager.Close()

	// Insert enough to force splits (BTreeOrder = 100, so 150 keys causes a split)
	for i := 0; i < 150; i++ {
		k := []byte(fmt.Sprintf("split:%08d", i))
		if err := bt.Put(k, []byte("x")); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	// All keys should still be retrievable after splits
	for i := 0; i < 150; i++ {
		k := []byte(fmt.Sprintf("split:%08d", i))
		if _, err := bt.Get(k); err != nil {
			t.Fatalf("Get after split, key %d: %v", i, err)
		}
	}
}

// ── Engine tests ──────────────────────────────────────────────────────────

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	e, err := Open(Options{
		DataFile: dir + "/koded.db",
		WALFile:  dir + "/koded.wal",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestEnginePutGet(t *testing.T) {
	e := newTestEngine(t)
	e.Put([]byte("hello"), []byte("world"))
	val, err := e.Get([]byte("hello"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "world" {
		t.Errorf("expected 'world', got %q", val)
	}
}

func TestEngineDelete(t *testing.T) {
	e := newTestEngine(t)
	e.Put([]byte("bye"), []byte("gone"))
	e.Delete([]byte("bye"))
	_, err := e.Get([]byte("bye"))
	if err != storage.ErrKeyNotFound {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

func TestEngineOverwrite(t *testing.T) {
	e := newTestEngine(t)
	e.Put([]byte("x"), []byte("1"))
	e.Put([]byte("x"), []byte("2"))
	val, _ := e.Get([]byte("x"))
	if string(val) != "2" {
		t.Errorf("expected '2', got %q", val)
	}
}

func TestEngineScan(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 10; i++ {
		e.Put([]byte(fmt.Sprintf("scan:%02d", i)), []byte("v"))
	}
	e.Delete([]byte("scan:05"))

	var count int
	e.Scan(func(k, v []byte) bool {
		count++
		return true
	})
	if count != 9 {
		t.Errorf("expected 9 entries (1 deleted), got %d", count)
	}
}

func TestEngineRangeScan(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 20; i++ {
		e.Put([]byte(fmt.Sprintf("%03d", i)), []byte("v"))
	}

	var found []string
	e.RangeScan([]byte("005"), []byte("010"), func(k, v []byte) bool {
		found = append(found, string(k))
		return true
	})

	if len(found) != 5 {
		t.Errorf("expected 5 in range, got %d: %v", len(found), found)
	}
}

func TestEngineFlushAndRecover(t *testing.T) {
	dir := t.TempDir()
	opts := Options{DataFile: dir + "/koded.db", WALFile: dir + "/koded.wal"}

	// Write data + flush
	e1, _ := Open(opts)
	for i := 0; i < 50; i++ {
		e1.Put([]byte(fmt.Sprintf("rec:%04d", i)), []byte(fmt.Sprintf("v%d", i)))
	}
	e1.Flush()
	e1.Close()

	// Reopen — data should survive
	e2, err := Open(opts)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer e2.Close()

	for i := 0; i < 50; i++ {
		k := []byte(fmt.Sprintf("rec:%04d", i))
		expected := fmt.Sprintf("v%d", i)
		val, err := e2.Get(k)
		if err != nil {
			t.Fatalf("key %d not found after reopen: %v", i, err)
		}
		if string(val) != expected {
			t.Errorf("key %d: expected %q got %q", i, expected, val)
		}
	}
}

func TestEngineWALRecovery(t *testing.T) {
	dir := t.TempDir()
	opts := Options{DataFile: dir + "/koded.db", WALFile: dir + "/koded.wal"}

	// Write data WITHOUT flushing (simulates crash)
	e1, _ := Open(opts)
	e1.Put([]byte("crash_key"), []byte("survived"))
	// Close without flush (WAL still has entries)
	e1.wal.Close()
	e1.pager.Close()
	e1.closed = true

	// Reopen — WAL replay should recover the data
	e2, err := Open(opts)
	if err != nil {
		t.Fatalf("reopen after crash: %v", err)
	}
	defer e2.Close()

	val, err := e2.Get([]byte("crash_key"))
	if err != nil {
		t.Fatalf("crash recovery failed: %v", err)
	}
	if string(val) != "survived" {
		t.Errorf("expected 'survived', got %q", val)
	}
}

func TestEngineStats(t *testing.T) {
	e := newTestEngine(t)
	e.Put([]byte("a"), []byte("1"))
	e.Put([]byte("b"), []byte("2"))

	stats := e.Stats()
	if stats.MemTableSize != 2 {
		t.Errorf("expected memtable size 2, got %d", stats.MemTableSize)
	}
	if stats.WALSequence != 2 {
		t.Errorf("expected WAL sequence 2, got %d", stats.WALSequence)
	}
}