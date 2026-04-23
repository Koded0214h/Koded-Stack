package storage

import (
    "os"
    "fmt"
    "github.com/Koded0214h/kodeddb-core/pkg/types"
)

type Pager struct {
    file     *os.File
    fileSize int64
}

// NewPager opens the database file and calculates how many pages we have
func NewPager(fileName string) (*Pager, error) {
    // os.O_RDWR: Read/Write
    // os.O_CREATE: Create if doesn't exist
    f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0666)
    if err != nil {
        return nil, err
    }

    info, _ := f.Stat()
    
    return &Pager{
        file:     f,
        fileSize: info.Size(),
    }, nil
}

// ReadPage fetches a specific 4KB block from the disk
func (p *Pager) ReadPage(pageId uint64) ([]byte, error) {
    offset := int64(pageId * types.PageSize)
    
    // Ensure we aren't reading past the end of the file
    if offset >= p.fileSize {
        return nil, fmt.Errorf("page %d out of bounds", pageId)
    }

    page := make([]byte, types.PageSize)
    _, err := p.file.ReadAt(page, offset)
    return page, err
}

// WritePage flushes a 4KB block to a specific location on disk
func (p *Pager) WritePage(pageId uint64, data []byte) error {
    if len(data) != types.PageSize {
        return fmt.Errorf("invalid page size: expected %d, got %d", types.PageSize, len(data))
    }

    offset := int64(pageId * types.PageSize)
    _, err := p.file.WriteAt(data, offset)
    return err
}