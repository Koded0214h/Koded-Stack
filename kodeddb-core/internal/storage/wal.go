package storage

import (
	"encoding/binary"
	"os"
	"sync"
)

type LogEntry struct {
	Type  uint8  // 0 for Put, 1 for Delete
	Key   []byte
	Value []byte
}

type WAL struct {
	file *os.File
	mu   sync.Mutex
}

func NewWAL(path string) (*WAL, error) {
	// O_APPEND: always write at the end
	// O_SYNC: ask the OS to flush to disk immediately
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// Write appends a record to the log. 
// This is the "No-Vibe" version: raw bytes only.
func (w *WAL) Write(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Format: [Type(1B)][KeyLen(2B)][ValLen(4B)][Key...][Value...]
	header := make([]byte, 7)
	header[0] = entry.Type
	binary.LittleEndian.PutUint16(header[1:3], uint16(len(entry.Key)))
	binary.LittleEndian.PutUint32(header[3:7], uint32(len(entry.Value)))

	if _, err := w.file.Write(header); err != nil {
		return err
	}
	if _, err := w.file.Write(entry.Key); err != nil {
		return err
	}
	if _, err := w.file.Write(entry.Value); err != nil {
		return err
	}

	// This is the "Safety" line. Without this, the WAL is useless.
	return w.file.Sync()
}


func (w *WAL) ReadAll(fn func(LogEntry)) error {
	// We open a separate read-only file handle for recovery
	f, err := os.Open(w.file.Name())
	if err != nil {
		return err
	}
	defer f.Close()

	for {
		header := make([]byte, 7)
		_, err := f.Read(header)
		if err != nil {
			break // EOF or error
		}

		kLen := binary.LittleEndian.Uint16(header[1:3])
		vLen := binary.LittleEndian.Uint32(header[3:7])

		key := make([]byte, kLen)
		val := make([]byte, vLen)

		f.Read(key)
		f.Read(val)

		fn(LogEntry{
			Type:  header[0],
			Key:   key,
			Value: val,
		})
	}
	return nil
}