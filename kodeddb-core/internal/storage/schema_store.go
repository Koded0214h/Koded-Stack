package storage

import (
	"fmt"
	"sync"

	"github.com/Koded0214h/kodeddb-core/pkg/types"
)

/*
SchemaStore persists table schemas inside the same engine as user data.
Schemas are stored as regular key-value pairs under a reserved prefix:

  Key:   __schema__:tableName
  Value: encoded TableSchema bytes

This means schemas survive crashes via WAL replay, just like user data.
No separate catalog file needed.
*/

const schemaPrefix = "__schema__:"

// Storer is the subset of Engine methods SchemaStore needs.
// This avoids a circular import between storage and engine packages.
type Storer interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Scan(fn func(key, value []byte) bool) error
}

type SchemaStore struct {
	mu      sync.RWMutex
	engine  Storer
	cache   map[string]*types.TableSchema // tableName → schema (in-memory cache)
}

func NewSchemaStore(engine Storer) (*SchemaStore, error) {
	ss := &SchemaStore{
		engine: engine,
		cache:  make(map[string]*types.TableSchema),
	}
	// Load all existing schemas into cache on startup
	if err := ss.loadAll(); err != nil {
		return nil, fmt.Errorf("schema store: load: %w", err)
	}
	return ss, nil
}

// ── Schema CRUD ───────────────────────────────────────────────────────────────

// CreateTable persists a new table schema.
func (ss *SchemaStore) CreateTable(schema *types.TableSchema) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, exists := ss.cache[schema.Name]; exists {
		return fmt.Errorf("table %q already exists", schema.Name)
	}

	// Validate: must have exactly one primary key
	pkCount := 0
	for _, c := range schema.Columns {
		if c.PrimaryKey { pkCount++ }
	}
	if pkCount == 0 {
		return fmt.Errorf("table %q: no primary key defined", schema.Name)
	}
	if pkCount > 1 {
		return fmt.Errorf("table %q: composite primary keys not supported yet", schema.Name)
	}

	encoded := types.EncodeSchema(schema)
	key      := []byte(schemaPrefix + schema.Name)

	if err := ss.engine.Put(key, encoded); err != nil {
		return fmt.Errorf("schema store: persist %q: %w", schema.Name, err)
	}

	ss.cache[schema.Name] = schema
	return nil
}

// GetTable returns the schema for a table by name.
func (ss *SchemaStore) GetTable(name string) (*types.TableSchema, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if schema, ok := ss.cache[name]; ok {
		return schema, nil
	}
	return nil, fmt.Errorf("table %q does not exist", name)
}

// DropTable removes a table schema.
func (ss *SchemaStore) DropTable(name string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.cache[name]; !ok {
		return fmt.Errorf("table %q does not exist", name)
	}

	key := []byte(schemaPrefix + name)
	if err := ss.engine.Delete(key); err != nil {
		return fmt.Errorf("schema store: drop %q: %w", name, err)
	}

	delete(ss.cache, name)
	return nil
}

// ListTables returns all table names.
func (ss *SchemaStore) ListTables() []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	names := make([]string, 0, len(ss.cache))
	for name := range ss.cache {
		names = append(names, name)
	}
	return names
}

// TableExists returns true if the table is defined.
func (ss *SchemaStore) TableExists(name string) bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	_, ok := ss.cache[name]
	return ok
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (ss *SchemaStore) loadAll() error {
	prefix := []byte(schemaPrefix)
	return ss.engine.Scan(func(key, value []byte) bool {
		if len(key) <= len(prefix) {
			return true
		}
		// Only process schema keys
		for i, b := range prefix {
			if key[i] != b { return true }
		}
		schema, err := types.DecodeSchema(value)
		if err != nil {
			// Log and skip corrupt schema
			fmt.Printf("[schema_store] warn: corrupt schema for key %q: %v\n", key, err)
			return true
		}
		ss.cache[schema.Name] = schema
		return true
	})
}