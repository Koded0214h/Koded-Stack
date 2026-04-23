package storage

import (
	"encoding/binary"
	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
Page Layout Structure:
- Checksum (4B)
- SlotCount (2B)
- FreeSpacePointer (2B) -> Offset where the next record should be written
- Slot Array (2B each)  -> Offsets to the records
- ... Free Space ...
- Records (Variable)
*/

type Page struct {
	data []byte // The raw 4096 bytes
}

func NewPage() *Page {
	p := &Page{data: make([]byte, types.PageSize)}
	// Initialize FreeSpacePointer to the end of the page
	p.setFreeSpacePointer(uint16(types.PageSize))
	return p
}

// setFreeSpacePointer writes the 2-byte offset to bytes [6:8]
func (p *Page) setFreeSpacePointer(offset uint16) {
	binary.LittleEndian.PutUint16(p.data[6:8], offset)
}

func (p *Page) getFreeSpacePointer() uint16 {
	return binary.LittleEndian.Uint16(p.data[6:8])
}

func (p *Page) getSlotCount() uint16 {
	return binary.LittleEndian.Uint16(p.data[4:6])
}

func (p *Page) setSlotCount(count uint16) {
	binary.LittleEndian.PutUint16(p.data[4:6], count)
}