package engine

import (
	"fmt"
	"sync"

	"github.com/Koded0214h/kodeddb-core/internal/storage"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
KodedDB Engine — the coordinator.

Write path:
  1. WAL.WritePut(key, value)      — durability first, always
  2. MemTable.Put(key, value)      — hot in-memory write
  3. If MemTable.ShouldFlush():
       flush MemTable → BTree pages on disk
       WAL.Truncate()              — WAL entries now baked into pages

Read path:
  1. Check MemTable (fastest — in memory)
  2. If not found: check BTree (disk, but cached)

Delete path:
  1. WAL.WriteDelete(key)
  2. MemTable.Delete(key)          — inserts tombstone
  3. BTree.Delete(key)             — removes from disk index

Recovery (on startup):
  1. Replay WAL into MemTable
  2. Flush MemTable → BTree (replays any unflushed mutations)
  3. Truncate WAL

This gives us: D (WAL) + C (BTree ACID) + I (RWMutex) + A (WAL replay)
*/

type Engine struct {
	mu       sync.RWMutex
	pager    *storage.Pager
	wal      *storage.WAL
	memtable *storage.MemTable
	btree    *BTree
	closed   bool
}

type Options struct {
	DataFile string
	WALFile  string
}

func Open(opts Options) (*Engine, error) {
	pager, err := storage.NewPager(opts.DataFile)
	if err != nil {
		return nil, fmt.Errorf("engine: open pager: %w", err)
	}

	wal, err := storage.NewWAL(opts.WALFile)
	if err != nil {
		return nil, fmt.Errorf("engine: open wal: %w", err)
	}

	btree, err := NewBTree(pager)
	if err != nil {
		return nil, fmt.Errorf("engine: init btree: %w", err)
	}

	e := &Engine{
		pager:    pager,
		wal:      wal,
		memtable: storage.NewMemTable(),
		btree:    btree,
	}

	// Crash recovery: replay WAL → memtable → btree
	if err := e.recover(); err != nil {
		return nil, fmt.Errorf("engine: recovery: %w", err)
	}

	return e, nil
}

// ── Recovery ───────────────────────────────────────────────────────────────

func (e *Engine) recover() error {
	var count int
	e.wal.ReadAll(func(entry storage.LogEntry) {
		switch entry.Type {
		case types.WALPut:
			e.memtable.Put(entry.Key, entry.Value)
		case types.WALDelete:
			e.memtable.Delete(entry.Key)
		}
		count++
	})

	if count == 0 {
		return nil // nothing to recover
	}

	fmt.Printf("[engine] recovering %d WAL entries\n", count)

	// Flush recovered memtable into BTree
	if err := e.flushMemTable(); err != nil {
		return fmt.Errorf("recovery flush: %w", err)
	}

	// WAL entries are now in BTree pages — safe to truncate
	return e.wal.Truncate()
}

// ── Write operations ───────────────────────────────────────────────────────

// Put writes a key-value pair.
func (e *Engine) Put(key, value []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("engine: empty key")
	}
	if len(key) > types.MaxKeySize {
		return fmt.Errorf("engine: key too large (%d > %d)", len(key), types.MaxKeySize)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. WAL first — durability guaranteed before we touch memory
	if err := e.wal.WritePut(key, value); err != nil {
		return fmt.Errorf("engine: wal put: %w", err)
	}

	// 2. MemTable
	e.memtable.Put(key, value)

	// 3. Flush if threshold reached
	if e.memtable.ShouldFlush() {
		if err := e.flushMemTable(); err != nil {
			return fmt.Errorf("engine: flush: %w", err)
		}
	}

	return nil
}

// Delete removes a key.
func (e *Engine) Delete(key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("engine: empty key")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.wal.WriteDelete(key); err != nil {
		return fmt.Errorf("engine: wal delete: %w", err)
	}

	e.memtable.Delete(key)

	// Also remove from BTree immediately
	_ = e.btree.Delete(key) // ignore ErrKeyNotFound

	return nil
}

// ── Read operations ────────────────────────────────────────────────────────

// Get returns the value for key.
func (e *Engine) Get(key []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// MemTable first (most recent writes live here)
	if e.memtable.IsDeleted(key) {
		return nil, storage.ErrKeyNotFound
	}
	if val, ok := e.memtable.Get(key); ok {
		return val, nil
	}

	// Fall through to BTree (disk)
	val, err := e.btree.Get(key)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Has returns true if the key exists.
func (e *Engine) Has(key []byte) bool {
	_, err := e.Get(key)
	return err == nil
}

// ── Scan operations ────────────────────────────────────────────────────────

// Scan iterates all key-value pairs in sorted order.
// Merges MemTable (in-memory) with BTree (disk) — deduplicated by key.
func (e *Engine) Scan(fn func(key, value []byte) bool) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Collect memtable entries
	memEntries := make(map[string]storage.MemEntry)
	e.memtable.Scan(func(me storage.MemEntry) bool {
		memEntries[string(me.Key)] = me
		return true
	})

	// Iterate BTree, merge with memtable
	seen := make(map[string]bool)
	err := e.btree.FullScan(func(be BTreeEntry) bool {
		k := string(be.Key)
		seen[k] = true

		// Memtable overrides disk
		if me, ok := memEntries[k]; ok {
			if me.Deleted {
				return true // skip deleted
			}
			return fn(me.Key, me.Value)
		}
		return fn(be.Key, be.Value)
	})
	if err != nil {
		return err
	}

	// Emit memtable-only entries (not yet flushed to BTree)
	e.memtable.Scan(func(me storage.MemEntry) bool {
		if seen[string(me.Key)] || me.Deleted {
			return true
		}
		return fn(me.Key, me.Value)
	})

	return nil
}

// RangeScan iterates keys in [start, end).
func (e *Engine) RangeScan(start, end []byte, fn func(key, value []byte) bool) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Merge BTree range with memtable range
	memEntries := make(map[string]storage.MemEntry)
	e.memtable.RangeScan(start, end, func(me storage.MemEntry) bool {
		memEntries[string(me.Key)] = me
		return true
	})

	seen := make(map[string]bool)
	err := e.btree.RangeScan(start, end, func(be BTreeEntry) bool {
		k := string(be.Key)
		seen[k] = true
		if me, ok := memEntries[k]; ok {
			if me.Deleted {
				return true
			}
			return fn(me.Key, me.Value)
		}
		return fn(be.Key, be.Value)
	})
	if err != nil {
		return err
	}

	// Emit memtable-only entries in range
	e.memtable.RangeScan(start, end, func(me storage.MemEntry) bool {
		if seen[string(me.Key)] || me.Deleted {
			return true
		}
		return fn(me.Key, me.Value)
	})

	return nil
}

// ── Flush ─────────────────────────────────────────────────────────────────

// flushMemTable writes all memtable entries into the BTree and clears the memtable.
// Must be called with e.mu held (write).
func (e *Engine) flushMemTable() error {
	var flushErr error
	count := 0

	e.memtable.Scan(func(me storage.MemEntry) bool {
		if me.Deleted {
			_ = e.btree.Delete(me.Key)
		} else {
			if err := e.btree.Put(me.Key, me.Value); err != nil {
				flushErr = err
				return false
			}
		}
		count++
		return true
	})

	if flushErr != nil {
		return flushErr
	}

	if err := e.pager.FlushDirty(); err != nil {
		return fmt.Errorf("flush dirty pages: %w", err)
	}

	e.memtable.Clear()

	// Truncate WAL — data is now in pages
	if err := e.wal.Truncate(); err != nil {
		return fmt.Errorf("wal truncate: %w", err)
	}

	fmt.Printf("[engine] flushed %d entries to disk\n", count)
	return nil
}

// Flush forces an immediate memtable flush (for testing / checkpoint).
func (e *Engine) Flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.flushMemTable()
}

// ── Stats ──────────────────────────────────────────────────────────────────

type Stats struct {
	MemTableSize  int
	MemTableBytes int
	PageCount     uint64
	WALSequence   uint64
}

func (e *Engine) Stats() Stats {
	return Stats{
		MemTableSize:  e.memtable.Size(),
		MemTableBytes: e.memtable.ByteSize(),
		PageCount:     e.pager.PageCount(),
		WALSequence:   e.wal.Sequence(),
	}
}

// ── Lifecycle ─────────────────────────────────────────────────────────────

// Close flushes everything and closes all files.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	// Final flush
	if e.memtable.Size() > 0 {
		if err := e.flushMemTable(); err != nil {
			return fmt.Errorf("engine: close flush: %w", err)
		}
	}

	if err := e.pager.Close(); err != nil {
		return fmt.Errorf("engine: close pager: %w", err)
	}
	if err := e.wal.Close(); err != nil {
		return fmt.Errorf("engine: close wal: %w", err)
	}

	return nil
}