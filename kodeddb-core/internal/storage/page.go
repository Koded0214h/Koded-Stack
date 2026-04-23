package storage

import (
	"encoding/binary"
	"errors"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
Page Layout (4096 bytes):
┌─────────────────────────────────────┐
│ Header (64 bytes)                   │
│   [0:4]   Checksum   (uint32)       │
│   [4:6]   SlotCount  (uint16)       │
│   [6:8]   FreeSpacePtr (uint16)     │
│   [8:9]   PageType   (uint8)        │
│   [9:17]  PageID     (uint64)       │
│   [17:25] ParentID   (uint64)       │
│   [25:64] Reserved                  │
├─────────────────────────────────────┤
│ Slot Array (grows downward)         │
│   Each slot: [2B offset][2B length] │
├─────────────────────────────────────┤
│ Free Space                          │
├─────────────────────────────────────┤
│ Records (grow upward from bottom)   │
│   Each record: [2B keyLen][key][val]│
└─────────────────────────────────────┘
*/

var (
	ErrPageFull    = errors.New("page full")
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyTooLarge = errors.New("key exceeds maximum size")
)

type Page struct {
	data   []byte
	pageID uint64
	dirty  bool
}

func NewPage(pageID uint64, pageType uint8) *Page {
	p := &Page{
		data:   make([]byte, types.PageSize),
		pageID: pageID,
	}
	p.setFreeSpacePointer(uint16(types.PageSize))
	p.setSlotCount(0)
	p.setPageType(pageType)
	p.setPageID(pageID)
	return p
}

// NewPageFromBytes wraps raw disk bytes into a Page struct
func NewPageFromBytes(data []byte, pageID uint64) *Page {
	p := &Page{
		data:   make([]byte, types.PageSize),
		pageID: pageID,
	}
	copy(p.data, data)
	return p
}

// ── Header field accessors ─────────────────────────────────────────────────

func (p *Page) getChecksum() uint32 {
	return binary.LittleEndian.Uint32(p.data[0:4])
}
func (p *Page) setChecksum(v uint32) {
	binary.LittleEndian.PutUint32(p.data[0:4], v)
}
func (p *Page) getSlotCount() uint16 {
	return binary.LittleEndian.Uint16(p.data[4:6])
}
func (p *Page) setSlotCount(count uint16) {
	binary.LittleEndian.PutUint16(p.data[4:6], count)
}
func (p *Page) getFreeSpacePointer() uint16 {
	return binary.LittleEndian.Uint16(p.data[6:8])
}
func (p *Page) setFreeSpacePointer(offset uint16) {
	binary.LittleEndian.PutUint16(p.data[6:8], offset)
}
func (p *Page) getPageType() uint8 { return p.data[8] }
func (p *Page) setPageType(t uint8) { p.data[8] = t }
func (p *Page) getPageID() uint64 {
	return binary.LittleEndian.Uint64(p.data[9:17])
}
func (p *Page) setPageID(id uint64) {
	binary.LittleEndian.PutUint64(p.data[9:17], id)
}
func (p *Page) getParentID() uint64 {
	return binary.LittleEndian.Uint64(p.data[17:25])
}
func (p *Page) setParentID(id uint64) {
	binary.LittleEndian.PutUint64(p.data[17:25], id)
}

// ── Slot array helpers ─────────────────────────────────────────────────────
// Each slot is 4 bytes: [2B offset from page start][2B record length]

func (p *Page) slotOffset(slotIdx uint16) int {
	return types.PageHeaderSize + int(slotIdx)*4
}
func (p *Page) getSlot(slotIdx uint16) (offset, length uint16) {
	base := p.slotOffset(slotIdx)
	offset = binary.LittleEndian.Uint16(p.data[base : base+2])
	length = binary.LittleEndian.Uint16(p.data[base+2 : base+4])
	return
}
func (p *Page) setSlot(slotIdx uint16, offset, length uint16) {
	base := p.slotOffset(slotIdx)
	binary.LittleEndian.PutUint16(p.data[base:base+2], offset)
	binary.LittleEndian.PutUint16(p.data[base+2:base+4], length)
}

// ── Free space calculation ─────────────────────────────────────────────────

func (p *Page) freeSpace() int {
	slotArrayEnd := types.PageHeaderSize + int(p.getSlotCount())*4
	freePtr := int(p.getFreeSpacePointer())
	return freePtr - slotArrayEnd
}

// ── Record format: [2B keyLen][key bytes][value bytes] ─────────────────────

func (p *Page) Insert(key, value []byte) error {
	if len(key) > types.MaxKeySize {
		return ErrKeyTooLarge
	}

	recordSize := 2 + len(key) + len(value) // 2B keyLen header + key + value
	needed := recordSize + 4                 // + 4B for the new slot entry

	if p.freeSpace() < needed {
		return ErrPageFull
	}

	// Write record growing upward from free space pointer
	freePtr := p.getFreeSpacePointer()
	recordStart := int(freePtr) - recordSize

	binary.LittleEndian.PutUint16(p.data[recordStart:recordStart+2], uint16(len(key)))
	copy(p.data[recordStart+2:recordStart+2+len(key)], key)
	copy(p.data[recordStart+2+len(key):], value)

	// Write slot
	slotIdx := p.getSlotCount()
	p.setSlot(slotIdx, uint16(recordStart), uint16(recordSize))
	p.setSlotCount(slotIdx + 1)
	p.setFreeSpacePointer(uint16(recordStart))

	p.dirty = true
	return nil
}

// Get searches the slot array for a matching key, returns the value.
func (p *Page) Get(key []byte) ([]byte, error) {
	slotCount := p.getSlotCount()
	for i := uint16(0); i < slotCount; i++ {
		offset, length := p.getSlot(i)
		rec := p.data[offset : offset+length]

		kLen := int(binary.LittleEndian.Uint16(rec[0:2]))
		if kLen > len(rec)-2 {
			continue
		}
		k := rec[2 : 2+kLen]
		if bytesEqual(k, key) {
			value := make([]byte, int(length)-2-kLen)
			copy(value, rec[2+kLen:])
			return value, nil
		}
	}
	return nil, ErrKeyNotFound
}

// Delete marks a slot as deleted (tombstone: offset=0, length=0).
// Actual space is reclaimed on compaction.
func (p *Page) Delete(key []byte) error {
	slotCount := p.getSlotCount()
	for i := uint16(0); i < slotCount; i++ {
		offset, length := p.getSlot(i)
		if offset == 0 {
			continue // already deleted
		}
		rec := p.data[offset : offset+length]
		kLen := int(binary.LittleEndian.Uint16(rec[0:2]))
		if kLen > len(rec)-2 {
			continue
		}
		k := rec[2 : 2+kLen]
		if bytesEqual(k, key) {
			p.setSlot(i, 0, 0) // tombstone
			p.dirty = true
			return nil
		}
	}
	return ErrKeyNotFound
}

// ── Iterator — scan all live records on this page ─────────────────────────

type PageRecord struct {
	Key   []byte
	Value []byte
}

func (p *Page) Iterate(fn func(rec PageRecord) bool) {
	slotCount := p.getSlotCount()
	for i := uint16(0); i < slotCount; i++ {
		offset, length := p.getSlot(i)
		if offset == 0 {
			continue // tombstone
		}
		rec := p.data[offset : offset+length]
		kLen := int(binary.LittleEndian.Uint16(rec[0:2]))
		if kLen > len(rec)-2 {
			continue
		}
		key   := make([]byte, kLen)
		value := make([]byte, int(length)-2-kLen)
		copy(key,   rec[2:2+kLen])
		copy(value, rec[2+kLen:])
		if !fn(PageRecord{Key: key, Value: value}) {
			return
		}
	}
}

// ── Compaction — reclaim space from deleted records ────────────────────────
// Rewrites the page in a fresh layout with all live records.

func (p *Page) Compact() {
	fresh := NewPage(p.pageID, p.getPageType())
	fresh.setParentID(p.getParentID())

	p.Iterate(func(rec PageRecord) bool {
		fresh.Insert(rec.Key, rec.Value)
		return true
	})

	copy(p.data, fresh.data)
	p.dirty = true
}

// ── Checksum (simple XOR32 — replace with CRC32 in production) ────────────

func (p *Page) ComputeChecksum() uint32 {
	var cs uint32
	for i := 4; i < types.PageSize; i++ { // skip checksum field itself
		cs ^= uint32(p.data[i])
	}
	return cs
}

func (p *Page) WriteChecksum() {
	p.setChecksum(p.ComputeChecksum())
}

func (p *Page) VerifyChecksum() bool {
	return p.getChecksum() == p.ComputeChecksum()
}

// ── Raw bytes (for pager I/O) ──────────────────────────────────────────────

func (p *Page) Bytes() []byte { return p.data }
func (p *Page) IsDirty() bool { return p.dirty }
func (p *Page) ClearDirty()   { p.dirty = false }
func (p *Page) PageID() uint64 { return p.pageID }
func (p *Page) FreeSpace() int { return p.freeSpace() }
func (p *Page) SlotCount() uint16 { return p.getSlotCount() }

// ── Helpers ────────────────────────────────────────────────────────────────

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GetPageType is exported for use by the engine package
func (p *Page) GetPageType() uint8 { return p.getPageType() }