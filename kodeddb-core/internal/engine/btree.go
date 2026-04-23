package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Koded0214h/kodeddb-core/internal/storage"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
B+ Tree — the primary index structure for KodedDB.

Key properties:
  - All values stored at leaf nodes (internal nodes store keys only)
  - Leaf nodes linked in a doubly-linked list → O(n) range scans
  - Internal nodes store separator keys + child page pointers
  - Splits propagate upward; root splits create a new root

Node wire format on a Page (stored as page records):

  Internal node record: [1B type=0x02][4B numKeys][keys...][child pageIDs...]
    - keys:     each key is [2B len][key bytes]
    - children: each is [8B pageID], count = numKeys+1

  Leaf node record: [1B type=0x01][4B numKeys][key-value pairs...]
    - each pair: [2B kLen][4B vLen][key][value]
    - [8B nextLeaf][8B prevLeaf] at the start (after type byte)

We use one record per page (the whole node is one record).
This simplifies the implementation while staying correct.
In production: pack multiple small nodes per page.
*/

const (
	nodeTypeLeaf     = types.PageTypeLeaf
	nodeTypeInternal = types.PageTypeInternal
)

// ── BTree entry and node types ─────────────────────────────────────────────

type BTreeEntry struct {
	Key   []byte
	Value []byte
}

type leafNode struct {
	pageID   uint64
	entries  []BTreeEntry
	nextLeaf uint64 // 0 = no next
	prevLeaf uint64 // 0 = no prev
}

type internalNode struct {
	pageID   uint64
	keys     [][]byte  // separator keys, len = len(children) - 1
	children []uint64  // child page IDs
}

// ── BTree ──────────────────────────────────────────────────────────────────

type BTree struct {
	pager    *storage.Pager
	rootID   uint64
	order    int // max keys per node (split when exceeded)
}

func NewBTree(pager *storage.Pager) (*BTree, error) {
	bt := &BTree{pager: pager, order: types.BTreeOrder}

	rootID := pager.RootPageID()
	if rootID == 0 {
		// No root yet — allocate a new empty leaf as root
		pg, err := pager.AllocatePage(nodeTypeLeaf)
		if err != nil {
			return nil, fmt.Errorf("btree: init root: %w", err)
		}
		leaf := &leafNode{pageID: pg.PageID()}
		if err := bt.writeLeaf(pg, leaf); err != nil {
			return nil, err
		}
		if err := pager.WritePage(pg); err != nil {
			return nil, err
		}
		if err := pager.SetRootPage(pg.PageID()); err != nil {
			return nil, err
		}
		bt.rootID = pg.PageID()
	} else {
		bt.rootID = rootID
	}

	return bt, nil
}

// ── Public API ─────────────────────────────────────────────────────────────

// Get returns the value for key, or (nil, ErrKeyNotFound).
func (bt *BTree) Get(key []byte) ([]byte, error) {
	leaf, err := bt.findLeaf(key)
	if err != nil {
		return nil, err
	}
	for _, e := range leaf.entries {
		if bytes.Equal(e.Key, key) {
			return e.Value, nil
		}
	}
	return nil, storage.ErrKeyNotFound
}

// Put inserts or updates a key-value pair.
func (bt *BTree) Put(key, value []byte) error {
	// Find the leaf that should hold this key
	path, leaf, err := bt.findLeafWithPath(key)
	if err != nil {
		return err
	}

	// Update in place if key exists
	for i, e := range leaf.entries {
		if bytes.Equal(e.Key, key) {
			leaf.entries[i].Value = value
			return bt.flushLeaf(leaf)
		}
	}

	// Insert in sorted position
	pos := bt.leafInsertPos(leaf, key)
	leaf.entries = append(leaf.entries, BTreeEntry{})
	copy(leaf.entries[pos+1:], leaf.entries[pos:])
	leaf.entries[pos] = BTreeEntry{Key: key, Value: value}

	if len(leaf.entries) <= bt.order {
		return bt.flushLeaf(leaf)
	}

	// Leaf overflow → split
	return bt.splitLeaf(leaf, path)
}

// Delete removes a key. Returns ErrKeyNotFound if not present.
func (bt *BTree) Delete(key []byte) error {
	_, leaf, err := bt.findLeafWithPath(key)
	if err != nil {
		return err
	}
	for i, e := range leaf.entries {
		if bytes.Equal(e.Key, key) {
			leaf.entries = append(leaf.entries[:i], leaf.entries[i+1:]...)
			return bt.flushLeaf(leaf)
		}
	}
	return storage.ErrKeyNotFound
}

// RangeScan calls fn for every entry with key in [start, end).
// Leverages the leaf linked list for O(n) sequential scan.
func (bt *BTree) RangeScan(start, end []byte, fn func(BTreeEntry) bool) error {
	leaf, err := bt.findLeaf(start)
	if err != nil {
		return err
	}

	for {
		for _, e := range leaf.entries {
			if bytes.Compare(e.Key, start) < 0 {
				continue
			}
			if bytes.Compare(e.Key, end) >= 0 {
				return nil
			}
			if !fn(e) {
				return nil
			}
		}
		if leaf.nextLeaf == 0 {
			break
		}
		leaf, err = bt.readLeaf(leaf.nextLeaf)
		if err != nil {
			return err
		}
	}
	return nil
}

// FullScan iterates every entry in key order (all leaves left to right).
func (bt *BTree) FullScan(fn func(BTreeEntry) bool) error {
	// Find leftmost leaf
	leaf, err := bt.leftmostLeaf()
	if err != nil {
		return err
	}
	for {
		for _, e := range leaf.entries {
			if !fn(e) {
				return nil
			}
		}
		if leaf.nextLeaf == 0 {
			break
		}
		leaf, err = bt.readLeaf(leaf.nextLeaf)
		if err != nil {
			return err
		}
	}
	return nil
}

// ── Tree traversal ─────────────────────────────────────────────────────────

// findLeaf walks from root to the leaf that should contain key.
func (bt *BTree) findLeaf(key []byte) (*leafNode, error) {
	_, leaf, err := bt.findLeafWithPath(key)
	return leaf, err
}

// findLeafWithPath returns the traversal path (internal node page IDs) + leaf.
func (bt *BTree) findLeafWithPath(key []byte) ([]uint64, *leafNode, error) {
	var path []uint64
	currID := bt.rootID

	for {
		pg, err := bt.pager.ReadPage(currID)
		if err != nil {
			return nil, nil, fmt.Errorf("btree: read page %d: %w", currID, err)
		}

		switch pg.GetPageType() {
		case nodeTypeLeaf:
			leaf, err := bt.readLeafFromPage(pg)
			if err != nil {
				return nil, nil, err
			}
			return path, leaf, nil

		case nodeTypeInternal:
			node, err := bt.readInternal(currID)
			if err != nil {
				return nil, nil, err
			}
			path = append(path, currID)
			currID = bt.chooseChild(node, key)

		default:
			return nil, nil, fmt.Errorf("btree: unknown page type %d", pg.GetPageType())
		}
	}
}

func (bt *BTree) chooseChild(node *internalNode, key []byte) uint64 {
	for i, k := range node.keys {
		if bytes.Compare(key, k) < 0 {
			return node.children[i]
		}
	}
	return node.children[len(node.children)-1]
}

func (bt *BTree) leftmostLeaf() (*leafNode, error) {
	currID := bt.rootID
	for {
		pg, err := bt.pager.ReadPage(currID)
		if err != nil {
			return nil, err
		}
		if pg.GetPageType() == nodeTypeLeaf {
			return bt.readLeafFromPage(pg)
		}
		node, err := bt.readInternal(currID)
		if err != nil {
			return nil, err
		}
		currID = node.children[0]
	}
}

func (bt *BTree) leafInsertPos(leaf *leafNode, key []byte) int {
	for i, e := range leaf.entries {
		if bytes.Compare(key, e.Key) < 0 {
			return i
		}
	}
	return len(leaf.entries)
}

// ── Splits ─────────────────────────────────────────────────────────────────

func (bt *BTree) splitLeaf(leaf *leafNode, path []uint64) error {
	mid := len(leaf.entries) / 2

	// New right sibling
	rightPg, err := bt.pager.AllocatePage(nodeTypeLeaf)
	if err != nil {
		return fmt.Errorf("btree: alloc right leaf: %w", err)
	}
	right := &leafNode{
		pageID:   rightPg.PageID(),
		entries:  make([]BTreeEntry, len(leaf.entries[mid:])),
		prevLeaf: leaf.pageID,
		nextLeaf: leaf.nextLeaf,
	}
	copy(right.entries, leaf.entries[mid:])
	leaf.entries  = leaf.entries[:mid]
	leaf.nextLeaf = right.pageID

	// Flush both leaves
	if err := bt.flushLeaf(leaf);  err != nil { return err }
	if err := bt.flushLeaf(right); err != nil { return err }

	// Propagate separator key upward
	separator := right.entries[0].Key
	return bt.insertIntoParent(leaf.pageID, separator, right.pageID, path)
}

func (bt *BTree) splitInternal(node *internalNode, path []uint64) error {
	mid := len(node.keys) / 2
	separator := node.keys[mid]

	rightPg, err := bt.pager.AllocatePage(nodeTypeInternal)
	if err != nil {
		return err
	}
	right := &internalNode{
		pageID:   rightPg.PageID(),
		keys:     make([][]byte, len(node.keys[mid+1:])),
		children: make([]uint64, len(node.children[mid+1:])),
	}
	copy(right.keys,     node.keys[mid+1:])
	copy(right.children, node.children[mid+1:])
	node.keys     = node.keys[:mid]
	node.children = node.children[:mid+1]

	if err := bt.flushInternal(node);  err != nil { return err }
	if err := bt.flushInternal(right); err != nil { return err }

	return bt.insertIntoParent(node.pageID, separator, right.pageID, path)
}

func (bt *BTree) insertIntoParent(leftID uint64, key []byte, rightID uint64, path []uint64) error {
	if len(path) == 0 {
		// Split the root — create a new root
		rootPg, err := bt.pager.AllocatePage(nodeTypeInternal)
		if err != nil {
			return err
		}
		newRoot := &internalNode{
			pageID:   rootPg.PageID(),
			keys:     [][]byte{key},
			children: []uint64{leftID, rightID},
		}
		if err := bt.flushInternal(newRoot); err != nil {
			return err
		}
		bt.rootID = newRoot.pageID
		return bt.pager.SetRootPage(newRoot.pageID)
	}

	parentID := path[len(path)-1]
	parent, err := bt.readInternal(parentID)
	if err != nil {
		return err
	}

	// Insert key + right child into parent
	pos := 0
	for pos < len(parent.keys) && bytes.Compare(key, parent.keys[pos]) > 0 {
		pos++
	}
	parent.keys     = append(parent.keys, nil)
	parent.children = append(parent.children, 0)
	copy(parent.keys[pos+1:],     parent.keys[pos:])
	copy(parent.children[pos+2:], parent.children[pos+1:])
	parent.keys[pos]       = key
	parent.children[pos+1] = rightID

	if len(parent.keys) <= bt.order {
		return bt.flushInternal(parent)
	}
	return bt.splitInternal(parent, path[:len(path)-1])
}

// ── Page serialization ─────────────────────────────────────────────────────


// writeLeaf serializes a leafNode into a Page record.
// Format: [8B nextLeaf][8B prevLeaf][4B numEntries]
//         repeated: [2B kLen][4B vLen][key][value]
func (bt *BTree) writeLeaf(pg *storage.Page, leaf *leafNode) error {
	buf := make([]byte, 0, 256)

	header := make([]byte, 20)
	binary.LittleEndian.PutUint64(header[0:8],  leaf.nextLeaf)
	binary.LittleEndian.PutUint64(header[8:16], leaf.prevLeaf)
	binary.LittleEndian.PutUint32(header[16:20], uint32(len(leaf.entries)))
	buf = append(buf, header...)

	for _, e := range leaf.entries {
		rec := make([]byte, 6+len(e.Key)+len(e.Value))
		binary.LittleEndian.PutUint16(rec[0:2], uint16(len(e.Key)))
		binary.LittleEndian.PutUint32(rec[2:6], uint32(len(e.Value)))
		copy(rec[6:], e.Key)
		copy(rec[6+len(e.Key):], e.Value)
		buf = append(buf, rec...)
	}

	// Store as the sole record on this page
	// Clear any existing record first
	freshPg := storage.NewPage(pg.PageID(), nodeTypeLeaf)
	if err := freshPg.Insert([]byte("__leaf__"), buf); err != nil {
		return fmt.Errorf("btree: write leaf page %d: %w", pg.PageID(), err)
	}
	copy(pg.Bytes(), freshPg.Bytes())
	return nil
}

func (bt *BTree) readLeaf(pageID uint64) (*leafNode, error) {
	pg, err := bt.pager.ReadPage(pageID)
	if err != nil {
		return nil, err
	}
	return bt.readLeafFromPage(pg)
}

func (bt *BTree) readLeafFromPage(pg *storage.Page) (*leafNode, error) {
	data, err := pg.Get([]byte("__leaf__"))
	if err != nil {
		// Empty leaf (just allocated)
		return &leafNode{pageID: pg.PageID()}, nil
	}
	if len(data) < 20 {
		return nil, fmt.Errorf("btree: leaf page %d: short data", pg.PageID())
	}

	leaf := &leafNode{pageID: pg.PageID()}
	leaf.nextLeaf = binary.LittleEndian.Uint64(data[0:8])
	leaf.prevLeaf = binary.LittleEndian.Uint64(data[8:16])
	numEntries   := binary.LittleEndian.Uint32(data[16:20])
	leaf.entries  = make([]BTreeEntry, 0, numEntries)

	off := 20
	for i := uint32(0); i < numEntries; i++ {
		if off+6 > len(data) {
			break
		}
		kLen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		vLen := int(binary.LittleEndian.Uint32(data[off+2 : off+6]))
		off += 6
		if off+kLen+vLen > len(data) {
			break
		}
		key   := make([]byte, kLen)
		value := make([]byte, vLen)
		copy(key,   data[off:off+kLen])
		copy(value, data[off+kLen:off+kLen+vLen])
		leaf.entries = append(leaf.entries, BTreeEntry{Key: key, Value: value})
		off += kLen + vLen
	}
	return leaf, nil
}

func (bt *BTree) flushLeaf(leaf *leafNode) error {
	pg, err := bt.pager.ReadPage(leaf.pageID)
	if err != nil {
		return err
	}
	if err := bt.writeLeaf(pg, leaf); err != nil {
		return err
	}
	return bt.pager.WritePage(pg)
}

// writeInternal serializes an internalNode.
// Format: [4B numKeys] repeated keys: [2B kLen][key] then children: [8B each]
func (bt *BTree) writeInternal(pg *storage.Page, node *internalNode) error {
	buf := make([]byte, 0, 256)

	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(node.keys)))
	buf = append(buf, header...)

	for _, k := range node.keys {
		kh := make([]byte, 2+len(k))
		binary.LittleEndian.PutUint16(kh[0:2], uint16(len(k)))
		copy(kh[2:], k)
		buf = append(buf, kh...)
	}
	for _, child := range node.children {
		cb := make([]byte, 8)
		binary.LittleEndian.PutUint64(cb, child)
		buf = append(buf, cb...)
	}

	freshPg := storage.NewPage(pg.PageID(), nodeTypeInternal)
	if err := freshPg.Insert([]byte("__internal__"), buf); err != nil {
		return fmt.Errorf("btree: write internal page %d: %w", pg.PageID(), err)
	}
	copy(pg.Bytes(), freshPg.Bytes())
	return nil
}

func (bt *BTree) readInternal(pageID uint64) (*internalNode, error) {
	pg, err := bt.pager.ReadPage(pageID)
	if err != nil {
		return nil, err
	}
	data, err := pg.Get([]byte("__internal__"))
	if err != nil {
		return &internalNode{pageID: pageID}, nil
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("btree: internal page %d: short data", pageID)
	}

	node := &internalNode{pageID: pageID}
	numKeys := int(binary.LittleEndian.Uint32(data[0:4]))
	node.keys     = make([][]byte, 0, numKeys)
	node.children = make([]uint64, 0, numKeys+1)

	off := 4
	for i := 0; i < numKeys; i++ {
		if off+2 > len(data) { break }
		kLen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if off+kLen > len(data) { break }
		k := make([]byte, kLen)
		copy(k, data[off:off+kLen])
		node.keys = append(node.keys, k)
		off += kLen
	}
	for off+8 <= len(data) {
		child := binary.LittleEndian.Uint64(data[off : off+8])
		node.children = append(node.children, child)
		off += 8
	}
	return node, nil
}

func (bt *BTree) flushInternal(node *internalNode) error {
	pg, err := bt.pager.ReadPage(node.pageID)
	if err != nil {
		return err
	}
	if err := bt.writeInternal(pg, node); err != nil {
		return err
	}
	return bt.pager.WritePage(pg)
}

// RootID returns the current root page ID.
func (bt *BTree) RootID() uint64 { return bt.rootID }