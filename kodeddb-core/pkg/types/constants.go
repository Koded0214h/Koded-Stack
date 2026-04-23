package types

const (
    // PageSize is the fundamental unit of disk I/O
    PageSize = 4096 
    
    // DbHeaderSize: We reserve the first page (Page 0) for metadata
    DbHeaderSize = PageSize
    
    // MagicNumber: To verify the file is actually a KodedDB file
    KodedMagic = 0x4B4F444544 // "KODED" in hex
)