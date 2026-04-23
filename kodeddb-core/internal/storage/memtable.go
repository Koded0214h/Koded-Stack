package storage

import (
	"math/rand"
	"sync"

	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
MemTable — the hot in-memory write buffer.

Every write goes: WAL → MemTable → (flush) → BTree pages on disk.

Why a skip list instead of a regular map?
  - Sorted iteration in O(n) — needed for ordered range scans
  - O(log n) insert/get/delete with good cache locality
  - Lock-free reads possible (we use RWMutex here for simplicity)
  - No rebalancing (unlike a BST) — predictable write latency

Skip list levels: each node is promoted to the next level with
probability 0.25. Max 16 levels → handles ~4^16 = 4 billion entries.
*/

const (
	maxLevel    = 16
	probability = 0.25
)

type memEntry struct {
	key     []byte
	value   []byte
	deleted bool // tombstone for delete operations
}

type skipNode struct {
	entry   memEntry
	forward []*skipNode
}

type MemTable struct {
	mu       sync.RWMutex
	head     *skipNode
	size     int  // number of live entries
	byteSize int  // approximate memory usage in bytes
	level    int  // current max level in use
}

func NewMemTable() *MemTable {
	head := &skipNode{forward: make([]*skipNode, maxLevel)}
	return &MemTable{head: head, level: 1}
}

// ── Skip list internals ────────────────────────────────────────────────────

func randomLevel() int {
	level := 1
	for level < maxLevel && rand.Float32() < probability {
		level++
	}
	return level
}

func compareKeys(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// findUpdate returns the update array (predecessor nodes at each level).
func (m *MemTable) findUpdate(key []byte) ([maxLevel]*skipNode, *skipNode) {
	var update [maxLevel]*skipNode
	curr := m.head

	for i := m.level - 1; i >= 0; i-- {
		for curr.forward[i] != nil && compareKeys(curr.forward[i].entry.key, key) < 0 {
			curr = curr.forward[i]
		}
		update[i] = curr
	}

	return update, curr.forward[0]
}

// ── Public API ─────────────────────────────────────────────────────────────

// Put inserts or updates a key-value pair.
func (m *MemTable) Put(key, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	update, next := m.findUpdate(key)

	// Key already exists — update in place
	if next != nil && compareKeys(next.entry.key, key) == 0 {
		oldSize := len(next.entry.value)
		next.entry.value   = make([]byte, len(value))
		next.entry.deleted = false
		copy(next.entry.value, value)
		m.byteSize += len(value) - oldSize
		return
	}

	// New node
	level := randomLevel()
	if level > m.level {
		for i := m.level; i < level; i++ {
			update[i] = m.head
		}
		m.level = level
	}

	node := &skipNode{
		entry: memEntry{
			key:   make([]byte, len(key)),
			value: make([]byte, len(value)),
		},
		forward: make([]*skipNode, level),
	}
	copy(node.entry.key,   key)
	copy(node.entry.value, value)

	for i := 0; i < level; i++ {
		node.forward[i]   = update[i].forward[i]
		update[i].forward[i] = node
	}

	m.size++
	m.byteSize += len(key) + len(value)
}

// Delete marks a key as deleted (tombstone).
func (m *MemTable) Delete(key []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	update, next := m.findUpdate(key)

	if next == nil || compareKeys(next.entry.key, key) != 0 {
		// Key doesn't exist — insert tombstone anyway
		// so we can shadow an older value on disk
		level := 1
		node := &skipNode{
			entry: memEntry{
				key:     make([]byte, len(key)),
				deleted: true,
			},
			forward: make([]*skipNode, level),
		}
		copy(node.entry.key, key)
		node.forward[0]   = update[0].forward[0]
		update[0].forward[0] = node
		m.size++
		m.byteSize += len(key)
		return
	}

	next.entry.deleted = true
	next.entry.value   = nil
}

// Get returns the value for a key. Returns (nil, false) if deleted or not found.
func (m *MemTable) Get(key []byte) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, next := m.findUpdate(key)
	if next == nil || compareKeys(next.entry.key, key) != 0 {
		return nil, false
	}
	if next.entry.deleted {
		return nil, false // tombstone — key was deleted
	}
	val := make([]byte, len(next.entry.value))
	copy(val, next.entry.value)
	return val, true
}

// Has returns true if the key exists (even as a tombstone).
// Used to short-circuit B+tree lookups.
func (m *MemTable) Has(key []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, next := m.findUpdate(key)
	return next != nil && compareKeys(next.entry.key, key) == 0
}

// IsDeleted returns true if the key has a tombstone in the memtable.
func (m *MemTable) IsDeleted(key []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, next := m.findUpdate(key)
	if next == nil || compareKeys(next.entry.key, key) != 0 {
		return false
	}
	return next.entry.deleted
}

// ── Iteration ─────────────────────────────────────────────────────────────

type MemEntry struct {
	Key     []byte
	Value   []byte
	Deleted bool
}

// Scan iterates all entries in sorted key order.
// Calls fn for each; return false to stop early.
func (m *MemTable) Scan(fn func(MemEntry) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	curr := m.head.forward[0]
	for curr != nil {
		if !fn(MemEntry{
			Key:     curr.entry.key,
			Value:   curr.entry.value,
			Deleted: curr.entry.deleted,
		}) {
			return
		}
		curr = curr.forward[0]
	}
}

// RangeScan iterates entries with keys in [start, end).
func (m *MemTable) RangeScan(start, end []byte, fn func(MemEntry) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	curr := m.head
	// advance to start
	for i := m.level - 1; i >= 0; i-- {
		for curr.forward[i] != nil && compareKeys(curr.forward[i].entry.key, start) < 0 {
			curr = curr.forward[i]
		}
	}
	curr = curr.forward[0]

	for curr != nil {
		if compareKeys(curr.entry.key, end) >= 0 {
			break
		}
		if !fn(MemEntry{
			Key:     curr.entry.key,
			Value:   curr.entry.value,
			Deleted: curr.entry.deleted,
		}) {
			return
		}
		curr = curr.forward[0]
	}
}

// ── Flush check ───────────────────────────────────────────────────────────

// ShouldFlush returns true when the memtable has grown past the flush threshold.
func (m *MemTable) ShouldFlush() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size >= types.MemTableFlushThreshold
}

func (m *MemTable) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

func (m *MemTable) ByteSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byteSize
}

// Clear resets the memtable (called after a successful flush).
func (m *MemTable) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.head     = &skipNode{forward: make([]*skipNode, maxLevel)}
	m.size     = 0
	m.byteSize = 0
	m.level    = 1
}