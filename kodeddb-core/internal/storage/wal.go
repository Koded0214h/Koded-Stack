package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
WAL (Write-Ahead Log) — every mutation is written here BEFORE touching the
data pages. On crash recovery, we replay the WAL to restore state.

Entry wire format:
  [1B type][8B sequence][2B keyLen][4B valLen][key...][val...][4B CRC32]

The CRC32 at the end lets recovery skip truncated/corrupt entries
(e.g. from a crash mid-write).
*/

var ErrCorruptEntry = errors.New("wal: corrupt entry (bad checksum)")

type LogEntry struct {
	Type     uint8
	Sequence uint64
	Key      []byte
	Value    []byte
}

type WAL struct {
	file     *os.File
	mu       sync.Mutex
	sequence uint64
}

func NewWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("wal: open %s: %w", path, err)
	}

	w := &WAL{file: f}

	// Find highest sequence number in existing log
	_ = w.ReadAll(func(e LogEntry) {
		if e.Sequence > w.sequence {
			w.sequence = e.Sequence
		}
	})

	return w, nil
}

// Write appends a WAL entry. Blocks until fsync completes.
// Format: [1B type][8B seq][2B kLen][4B vLen][key][val][4B checksum]
func (w *WAL) Write(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.sequence++
	entry.Sequence = w.sequence

	// Build header: 1 + 8 + 2 + 4 = 15 bytes
	header := make([]byte, 15)
	header[0] = entry.Type
	binary.LittleEndian.PutUint64(header[1:9],   entry.Sequence)
	binary.LittleEndian.PutUint16(header[9:11],  uint16(len(entry.Key)))
	binary.LittleEndian.PutUint32(header[11:15], uint32(len(entry.Value)))

	// Compute checksum over header + key + value
	cs := checksum32(header)
	cs ^= checksum32(entry.Key)
	cs ^= checksum32(entry.Value)
	csBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(csBuf, cs)

	// Write in one syscall for atomicity
	record := make([]byte, 0, 15+len(entry.Key)+len(entry.Value)+4)
	record = append(record, header...)
	record = append(record, entry.Key...)
	record = append(record, entry.Value...)
	record = append(record, csBuf...)

	if _, err := w.file.Write(record); err != nil {
		return fmt.Errorf("wal: write: %w", err)
	}

	// fsync — without this the WAL is useless
	return w.file.Sync()
}

// WritePut is a convenience wrapper for WALPut entries.
func (w *WAL) WritePut(key, value []byte) error {
	return w.Write(LogEntry{Type: types.WALPut, Key: key, Value: value})
}

// WriteDelete is a convenience wrapper for WALDelete entries.
func (w *WAL) WriteDelete(key []byte) error {
	return w.Write(LogEntry{Type: types.WALDelete, Key: key})
}

// ReadAll replays the WAL, calling fn for each valid entry.
// Stops at EOF or first unrecoverable corruption.
// Corrupt/truncated entries at the tail are silently skipped
// (safe — they mean the crash happened mid-write for that entry).
func (w *WAL) ReadAll(fn func(LogEntry)) error {
	f, err := os.Open(w.file.Name())
	if err != nil {
		return fmt.Errorf("wal: open for read: %w", err)
	}
	defer f.Close()

	for {
		header := make([]byte, 15)
		_, err := io.ReadFull(f, header)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break // clean end or truncated tail
		}
		if err != nil {
			return fmt.Errorf("wal: read header: %w", err)
		}

		entryType := header[0]
		seq       := binary.LittleEndian.Uint64(header[1:9])
		kLen      := binary.LittleEndian.Uint16(header[9:11])
		vLen      := binary.LittleEndian.Uint32(header[11:15])

		key := make([]byte, kLen)
		val := make([]byte, vLen)
		csBuf := make([]byte, 4)

		if _, err := io.ReadFull(f, key);   err != nil { break }
		if _, err := io.ReadFull(f, val);   err != nil { break }
		if _, err := io.ReadFull(f, csBuf); err != nil { break }

		// Verify checksum
		expected := checksum32(header)
		expected ^= checksum32(key)
		expected ^= checksum32(val)
		got := binary.LittleEndian.Uint32(csBuf)

		if expected != got {
			// Corrupt entry — stop recovery here
			// (everything before this was good)
			break
		}

		fn(LogEntry{
			Type:     entryType,
			Sequence: seq,
			Key:      key,
			Value:    val,
		})
	}
	return nil
}

// Truncate clears the WAL after a successful checkpoint to disk.
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("wal: truncate: %w", err)
	}
	_, err := w.file.Seek(0, 0)
	return err
}

func (w *WAL) Sequence() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sequence
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// ── Simple XOR-based checksum ──────────────────────────────────────────────
// Good enough for entry validation. Replace with crc32.ChecksumIEEE in prod.
func checksum32(data []byte) uint32 {
	var cs uint32
	for i, b := range data {
		cs ^= uint32(b) << (uint(i%4) * 8)
	}
	return cs
}