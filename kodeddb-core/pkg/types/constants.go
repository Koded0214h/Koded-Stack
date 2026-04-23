package types

const (
	// PageSize is the fundamental unit of disk I/O (4KB)
	PageSize = 4096

	// DbHeaderSize: Page 0 is reserved for database metadata
	DbHeaderSize = PageSize

	// KodedMagic: "KODED" in hex — validates file is a KodedDB file
	KodedMagic = 0x4B4F444544

	// WAL entry types
	WALPut    uint8 = 0
	WALDelete uint8 = 1

	// Page types
	PageTypeLeaf     uint8 = 0x01
	PageTypeInternal uint8 = 0x02
	PageTypeOverflow uint8 = 0x03
	PageTypeFree     uint8 = 0xFF

	// Page header offsets (fixed layout)
	// [0:4]  Checksum
	// [4:6]  SlotCount
	// [6:8]  FreeSpacePointer
	// [8:9]  PageType
	// [9:17] PageID (uint64)
	// [17:25] ParentID (uint64)
	// [25:PageHeaderSize] Reserved
	PageHeaderSize = 64

	// MemTable threshold — flush to disk when this many entries
	MemTableFlushThreshold = 4096

	// BTree order — max keys per node
	BTreeOrder = 100

	// Max key size in bytes
	MaxKeySize = 512

	// Max inline value size — larger values go to overflow pages
	MaxInlineValueSize = 512
)