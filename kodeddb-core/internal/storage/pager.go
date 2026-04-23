package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
Database file layout:
┌──────────────────────────────┐
│ Page 0: Header Page          │
│   [0:8]  Magic number        │
│   [8:16] Page count          │
│   [16:24] Root page ID       │
│   [24:32] WAL sequence       │
│   [32:64] Reserved           │
├──────────────────────────────┤
│ Page 1: First data page      │
│ Page 2: ...                  │
│ ...                          │
└──────────────────────────────┘
*/

const (
	headerMagicOffset    = 0
	headerPageCountOffset = 8
	headerRootPageOffset  = 16
	headerWALSeqOffset    = 24
)

// PageCache — simple LRU-ish in-memory cache (fixed size map for now)
// In production: replace with a proper LRU eviction policy
type PageCache struct {
	mu    sync.RWMutex
	pages map[uint64]*Page
	cap   int
}

func newPageCache(capacity int) *PageCache {
	return &PageCache{pages: make(map[uint64]*Page, capacity), cap: capacity}
}

func (c *PageCache) get(id uint64) (*Page, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.pages[id]
	return p, ok
}

func (c *PageCache) put(p *Page) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Simple eviction: if at capacity, evict a random clean page
	if len(c.pages) >= c.cap {
		for id, pg := range c.pages {
			if !pg.IsDirty() {
				delete(c.pages, id)
				break
			}
		}
	}
	c.pages[p.PageID()] = p
}

func (c *PageCache) invalidate(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pages, id)
}

func (c *PageCache) dirtyPages() []*Page {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var dirty []*Page
	for _, p := range c.pages {
		if p.IsDirty() {
			dirty = append(dirty, p)
		}
	}
	return dirty
}

// ── Pager ─────────────────────────────────────────────────────────────────

type Pager struct {
	mu        sync.RWMutex
	file      *os.File
	pageCount uint64
	rootPage  uint64
	walSeq    uint64
	cache     *PageCache
}

func NewPager(fileName string) (*Pager, error) {
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("pager: open %s: %w", fileName, err)
	}

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	p := &Pager{
		file:  f,
		cache: newPageCache(256), // cache up to 256 pages (~1MB)
	}

	if info.Size() == 0 {
		// Brand new database — write header page
		if err := p.initHeader(); err != nil {
			return nil, fmt.Errorf("pager: init header: %w", err)
		}
	} else {
		// Existing database — read header
		if err := p.readHeader(); err != nil {
			return nil, fmt.Errorf("pager: read header: %w", err)
		}
	}

	return p, nil
}

// ── Header page ───────────────────────────────────────────────────────────

func (p *Pager) initHeader() error {
	header := make([]byte, types.PageSize)
	binary.LittleEndian.PutUint64(header[headerMagicOffset:],    types.KodedMagic)
	binary.LittleEndian.PutUint64(header[headerPageCountOffset:], 1) // page 0 exists
	binary.LittleEndian.PutUint64(header[headerRootPageOffset:],  0) // no root yet
	binary.LittleEndian.PutUint64(header[headerWALSeqOffset:],    0)

	_, err := p.file.WriteAt(header, 0)
	if err != nil {
		return err
	}
	p.pageCount = 1
	p.rootPage  = 0
	return p.file.Sync()
}

func (p *Pager) readHeader() error {
	header := make([]byte, types.PageSize)
	if _, err := p.file.ReadAt(header, 0); err != nil {
		return err
	}
	magic := binary.LittleEndian.Uint64(header[headerMagicOffset:])
	if magic != types.KodedMagic {
		return fmt.Errorf("pager: not a KodedDB file (magic=%x)", magic)
	}
	p.pageCount = binary.LittleEndian.Uint64(header[headerPageCountOffset:])
	p.rootPage  = binary.LittleEndian.Uint64(header[headerRootPageOffset:])
	p.walSeq    = binary.LittleEndian.Uint64(header[headerWALSeqOffset:])
	return nil
}

func (p *Pager) writeHeader() error {
	header := make([]byte, types.PageSize)
	// preserve magic
	binary.LittleEndian.PutUint64(header[headerMagicOffset:],     types.KodedMagic)
	binary.LittleEndian.PutUint64(header[headerPageCountOffset:],  p.pageCount)
	binary.LittleEndian.PutUint64(header[headerRootPageOffset:],   p.rootPage)
	binary.LittleEndian.PutUint64(header[headerWALSeqOffset:],     p.walSeq)
	_, err := p.file.WriteAt(header, 0)
	return err
}

// ── Page allocation ────────────────────────────────────────────────────────

// AllocatePage reserves a new page ID, extends the file, returns blank Page.
func (p *Pager) AllocatePage(pageType uint8) (*Page, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newID := p.pageCount
	p.pageCount++

	pg := NewPage(newID, pageType)

	// Write blank page to extend the file
	offset := int64(newID) * types.PageSize
	if _, err := p.file.WriteAt(pg.Bytes(), offset); err != nil {
		p.pageCount-- // rollback
		return nil, fmt.Errorf("pager: allocate page %d: %w", newID, err)
	}

	// Update header with new page count
	if err := p.writeHeader(); err != nil {
		return nil, err
	}

	p.cache.put(pg)
	return pg, nil
}

// ── Page read / write ──────────────────────────────────────────────────────

// ReadPage fetches a page — from cache first, then disk.
func (p *Pager) ReadPage(pageID uint64) (*Page, error) {
	// Cache hit
	if pg, ok := p.cache.get(pageID); ok {
		return pg, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if pageID >= p.pageCount {
		return nil, fmt.Errorf("pager: page %d out of bounds (count=%d)", pageID, p.pageCount)
	}

	buf := make([]byte, types.PageSize)
	offset := int64(pageID) * types.PageSize
	if _, err := p.file.ReadAt(buf, offset); err != nil {
		return nil, fmt.Errorf("pager: read page %d: %w", pageID, err)
	}

	pg := NewPageFromBytes(buf, pageID)
	p.cache.put(pg)
	return pg, nil
}

// WritePage flushes a page to disk and clears its dirty flag.
func (p *Pager) WritePage(pg *Page) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	pg.WriteChecksum()
	offset := int64(pg.PageID()) * types.PageSize
	if _, err := p.file.WriteAt(pg.Bytes(), offset); err != nil {
		return fmt.Errorf("pager: write page %d: %w", pg.PageID(), err)
	}
	pg.ClearDirty()
	return nil
}

// FlushDirty writes all dirty cached pages to disk.
func (p *Pager) FlushDirty() error {
	for _, pg := range p.cache.dirtyPages() {
		if err := p.WritePage(pg); err != nil {
			return err
		}
	}
	return p.file.Sync()
}

// ── Root page management ──────────────────────────────────────────────────

func (p *Pager) SetRootPage(id uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rootPage = id
	return p.writeHeader()
}

func (p *Pager) RootPageID() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rootPage
}

func (p *Pager) PageCount() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pageCount
}

// Close flushes all dirty pages and closes the file.
func (p *Pager) Close() error {
	if err := p.FlushDirty(); err != nil {
		return err
	}
	return p.file.Close()
}