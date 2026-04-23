package storage

import (
	"fmt"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
	"os"
	"testing"
)

// ── Page tests ────────────────────────────────────────────────────────────

func TestPageInsertGet(t *testing.T) {
	p := NewPage(1, 0x01)

	key   := []byte("hello")
	value := []byte("world")

	if err := p.Insert(key, value); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := p.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("expected %q got %q", value, got)
	}
}

func TestPageMultiInsert(t *testing.T) {
	p := NewPage(1, 0x01)
	for i := 0; i < 10; i++ {
		k := []byte(fmt.Sprintf("key%02d", i))
		v := []byte(fmt.Sprintf("val%02d", i))
		if err := p.Insert(k, v); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	for i := 0; i < 10; i++ {
		k := []byte(fmt.Sprintf("key%02d", i))
		v := []byte(fmt.Sprintf("val%02d", i))
		got, err := p.Get(k)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if string(got) != string(v) {
			t.Errorf("key%d: expected %q got %q", i, v, got)
		}
	}
}

func TestPageDelete(t *testing.T) {
	p := NewPage(1, 0x01)
	p.Insert([]byte("k"), []byte("v"))

	if err := p.Delete([]byte("k")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := p.Get([]byte("k"))
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestPageFull(t *testing.T) {
	p := NewPage(1, 0x01)
	// fill the page
	big := make([]byte, 200)
	for {
		if err := p.Insert(big, big); err == ErrPageFull {
			break
		}
	}
}

func TestPageChecksum(t *testing.T) {
	p := NewPage(1, 0x01)
	p.Insert([]byte("checkme"), []byte("value"))
	p.WriteChecksum()
	if !p.VerifyChecksum() {
		t.Fatal("checksum verification failed")
	}
	// Corrupt a byte
	p.data[100] ^= 0xFF
	if p.VerifyChecksum() {
		t.Fatal("expected checksum to fail after corruption")
	}
}

func TestPageCompact(t *testing.T) {
	p := NewPage(1, 0x01)
	p.Insert([]byte("a"), []byte("1"))
	p.Insert([]byte("b"), []byte("2"))
	p.Insert([]byte("c"), []byte("3"))
	p.Delete([]byte("b"))

	spaceBefore := p.FreeSpace()
	p.Compact()
	spaceAfter := p.FreeSpace()

	// After compaction, deleted record space should be reclaimed
	if spaceAfter <= spaceBefore {
		t.Errorf("expected more free space after compact: before=%d after=%d",
			spaceBefore, spaceAfter)
	}
	// Remaining keys still accessible
	if _, err := p.Get([]byte("a")); err != nil {
		t.Error("key 'a' lost after compact")
	}
	if _, err := p.Get([]byte("c")); err != nil {
		t.Error("key 'c' lost after compact")
	}
}

func TestPageIterate(t *testing.T) {
	p := NewPage(1, 0x01)
	keys := []string{"alpha", "beta", "gamma"}
	for _, k := range keys {
		p.Insert([]byte(k), []byte("v_"+k))
	}
	p.Delete([]byte("beta"))

	var found []string
	p.Iterate(func(rec PageRecord) bool {
		found = append(found, string(rec.Key))
		return true
	})
	if len(found) != 2 {
		t.Errorf("expected 2 live records, got %d: %v", len(found), found)
	}
}

// ── Pager tests ───────────────────────────────────────────────────────────

func TestPagerCreateAndRead(t *testing.T) {
	tmp := t.TempDir() + "/test.kdb"
	p, err := NewPager(tmp)
	if err != nil {
		t.Fatalf("NewPager: %v", err)
	}
	defer p.Close()

	pg, err := p.AllocatePage(0x01)
	if err != nil {
		t.Fatalf("AllocatePage: %v", err)
	}
	pg.Insert([]byte("pager-key"), []byte("pager-value"))
	if err := p.WritePage(pg); err != nil {
		t.Fatalf("WritePage: %v", err)
	}

	pg2, err := p.ReadPage(pg.PageID())
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}
	val, err := pg2.Get([]byte("pager-key"))
	if err != nil {
		t.Fatalf("Get after read: %v", err)
	}
	if string(val) != "pager-value" {
		t.Errorf("expected 'pager-value', got %q", val)
	}
}

func TestPagerMagicCheck(t *testing.T) {
	tmp := t.TempDir() + "/bad.kdb"
	// Write garbage
	os.WriteFile(tmp, make([]byte, 4096), 0644)

	_, err := NewPager(tmp)
	if err == nil {
		t.Fatal("expected error for bad magic number")
	}
}

func TestPagerPersistence(t *testing.T) {
	tmp := t.TempDir() + "/persist.kdb"

	// Write
	p1, _ := NewPager(tmp)
	pg, _ := p1.AllocatePage(0x01)
	pgID := pg.PageID()
	pg.Insert([]byte("persist"), []byte("yes"))
	p1.WritePage(pg)
	p1.Close()

	// Reopen
	p2, err := NewPager(tmp)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer p2.Close()

	pg2, err := p2.ReadPage(pgID)
	if err != nil {
		t.Fatalf("read after reopen: %v", err)
	}
	val, err := pg2.Get([]byte("persist"))
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if string(val) != "yes" {
		t.Errorf("expected 'yes', got %q", val)
	}
}

// ── WAL tests ─────────────────────────────────────────────────────────────

func TestWALWriteAndReplay(t *testing.T) {
	tmp := t.TempDir() + "/test.wal"
	w, err := NewWAL(tmp)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer w.Close()

	w.WritePut([]byte("name"), []byte("koded"))
	w.WritePut([]byte("role"), []byte("backend"))
	w.WriteDelete([]byte("role"))

	var entries []LogEntry
	w.ReadAll(func(e LogEntry) { entries = append(entries, e) })

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if string(entries[0].Key) != "name" || string(entries[0].Value) != "koded" {
		t.Errorf("entry 0 wrong: %+v", entries[0])
	}
	if entries[2].Type != types.WALDelete {
		t.Errorf("entry 2 should be delete")
	}
}

func TestWALChecksum(t *testing.T) {
	tmp := t.TempDir() + "/cs.wal"
	w, _ := NewWAL(tmp)
	w.WritePut([]byte("key"), []byte("val"))
	w.Close()

	// Corrupt the file
	data, _ := os.ReadFile(tmp)
	data[20] ^= 0xFF
	os.WriteFile(tmp, data, 0644)

	w2, _ := NewWAL(tmp)
	var count int
	w2.ReadAll(func(e LogEntry) { count++ })
	// corrupt entry should be skipped
	if count != 0 {
		t.Errorf("expected 0 valid entries after corruption, got %d", count)
	}
}

func TestWALTruncate(t *testing.T) {
	tmp := t.TempDir() + "/trunc.wal"
	w, _ := NewWAL(tmp)
	w.WritePut([]byte("a"), []byte("1"))
	w.WritePut([]byte("b"), []byte("2"))
	w.Truncate()

	var count int
	w.ReadAll(func(e LogEntry) { count++ })
	if count != 0 {
		t.Errorf("expected 0 entries after truncate, got %d", count)
	}
}

// ── MemTable tests ────────────────────────────────────────────────────────

func TestMemTablePutGet(t *testing.T) {
	m := NewMemTable()
	m.Put([]byte("foo"), []byte("bar"))
	val, ok := m.Get([]byte("foo"))
	if !ok {
		t.Fatal("expected to find key")
	}
	if string(val) != "bar" {
		t.Errorf("expected 'bar', got %q", val)
	}
}

func TestMemTableDelete(t *testing.T) {
	m := NewMemTable()
	m.Put([]byte("x"), []byte("1"))
	m.Delete([]byte("x"))
	_, ok := m.Get([]byte("x"))
	if ok {
		t.Error("expected key to be deleted")
	}
	if !m.IsDeleted([]byte("x")) {
		t.Error("expected IsDeleted to return true")
	}
}

func TestMemTableSortedScan(t *testing.T) {
	m := NewMemTable()
	// Insert out of order
	m.Put([]byte("zebra"), []byte("z"))
	m.Put([]byte("apple"), []byte("a"))
	m.Put([]byte("mango"), []byte("m"))

	var keys []string
	m.Scan(func(e MemEntry) bool {
		keys = append(keys, string(e.Key))
		return true
	})

	expected := []string{"apple", "mango", "zebra"}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("position %d: expected %q got %q", i, k, keys[i])
		}
	}
}

func TestMemTableRangeScan(t *testing.T) {
	m := NewMemTable()
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		m.Put([]byte(k), []byte("v_"+k))
	}

	var found []string
	m.RangeScan([]byte("b"), []byte("d"), func(e MemEntry) bool {
		found = append(found, string(e.Key))
		return true
	})

	if len(found) != 2 || found[0] != "b" || found[1] != "c" {
		t.Errorf("range scan wrong: %v", found)
	}
}

func TestMemTableUpdate(t *testing.T) {
	m := NewMemTable()
	m.Put([]byte("k"), []byte("v1"))
	m.Put([]byte("k"), []byte("v2"))
	val, _ := m.Get([]byte("k"))
	if string(val) != "v2" {
		t.Errorf("expected update to v2, got %q", val)
	}
	if m.Size() != 1 {
		t.Errorf("expected size 1 after update, got %d", m.Size())
	}
}

func TestMemTableClear(t *testing.T) {
	m := NewMemTable()
	m.Put([]byte("a"), []byte("1"))
	m.Put([]byte("b"), []byte("2"))
	m.Clear()
	if m.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", m.Size())
	}
	_, ok := m.Get([]byte("a"))
	if ok {
		t.Error("expected key gone after clear")
	}
}