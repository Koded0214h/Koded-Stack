package types

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// ── Data types ────────────────────────────────────────────────────────────────

type DataType uint8

const (
	TypeInt   DataType = 0x01 // int64
	TypeFloat DataType = 0x02 // float64
	TypeText  DataType = 0x03 // UTF-8 string
	TypeBool  DataType = 0x04 // bool
	TypeBytes DataType = 0x05 // raw []byte
	TypeSemi  DataType = 0x06 // semistructured — any shape per row
	TypeNull  DataType = 0xFF // NULL sentinel
)

func (d DataType) String() string {
	switch d {
	case TypeInt:   return "INT"
	case TypeFloat: return "FLOAT"
	case TypeText:  return "TEXT"
	case TypeBool:  return "BOOL"
	case TypeBytes: return "BYTES"
	case TypeSemi:  return "SEMI"
	case TypeNull:  return "NULL"
	default:        return fmt.Sprintf("UNKNOWN(%d)", d)
	}
}

func ParseDataType(s string) (DataType, error) {
	switch s {
	case "INT", "INTEGER", "BIGINT": return TypeInt, nil
	case "FLOAT", "REAL", "DOUBLE":  return TypeFloat, nil
	case "TEXT", "VARCHAR", "STRING":return TypeText, nil
	case "BOOL", "BOOLEAN":          return TypeBool, nil
	case "BYTES", "BLOB":            return TypeBytes, nil
	case "SEMI", "JSON", "DYNAMIC":  return TypeSemi, nil
	default:
		return 0, fmt.Errorf("unknown data type: %q", s)
	}
}

// ── Value — a typed cell value ────────────────────────────────────────────────

type Value struct {
	Type    DataType
	IntVal  int64
	FloatVal float64
	TextVal  string
	BoolVal  bool
	BytesVal []byte
	SemiVal  map[string]Value // semistructured fields
	IsNull   bool
}

var NullValue = Value{Type: TypeNull, IsNull: true}

func IntValue(v int64) Value   { return Value{Type: TypeInt,   IntVal: v} }
func FloatValue(v float64) Value { return Value{Type: TypeFloat, FloatVal: v} }
func TextValue(v string) Value  { return Value{Type: TypeText,  TextVal: v} }
func BoolValue(v bool) Value   { return Value{Type: TypeBool,  BoolVal: v} }
func BytesValue(v []byte) Value { return Value{Type: TypeBytes, BytesVal: v} }
func SemiValue(m map[string]Value) Value { return Value{Type: TypeSemi, SemiVal: m} }

func (v Value) String() string {
	if v.IsNull { return "NULL" }
	switch v.Type {
	case TypeInt:   return strconv.FormatInt(v.IntVal, 10)
	case TypeFloat: return strconv.FormatFloat(v.FloatVal, 'f', -1, 64)
	case TypeText:  return v.TextVal
	case TypeBool:
		if v.BoolVal { return "true" }
		return "false"
	case TypeBytes: return fmt.Sprintf("<bytes:%d>", len(v.BytesVal))
	case TypeSemi:  return fmt.Sprintf("<semi:%d fields>", len(v.SemiVal))
	default:        return "?"
	}
}

// Compare returns -1, 0, 1 for ordering. Types must match.
func (v Value) Compare(other Value) (int, error) {
	if v.IsNull && other.IsNull { return 0, nil }
	if v.IsNull  { return -1, nil }
	if other.IsNull { return 1, nil }
	if v.Type != other.Type {
		return 0, fmt.Errorf("type mismatch: %s vs %s", v.Type, other.Type)
	}
	switch v.Type {
	case TypeInt:
		if v.IntVal < other.IntVal { return -1, nil }
		if v.IntVal > other.IntVal { return 1, nil }
		return 0, nil
	case TypeFloat:
		if v.FloatVal < other.FloatVal { return -1, nil }
		if v.FloatVal > other.FloatVal { return 1, nil }
		return 0, nil
	case TypeText:
		if v.TextVal < other.TextVal { return -1, nil }
		if v.TextVal > other.TextVal { return 1, nil }
		return 0, nil
	case TypeBool:
		if v.BoolVal == other.BoolVal { return 0, nil }
		if !v.BoolVal { return -1, nil }
		return 1, nil
	default:
		return 0, fmt.Errorf("type %s not comparable", v.Type)
	}
}

func (v Value) Equal(other Value) bool {
	cmp, err := v.Compare(other)
	return err == nil && cmp == 0
}

// ── Value serialization ───────────────────────────────────────────────────────
// Format: [1B type][payload...]
//   INT:   [8B little-endian int64]
//   FLOAT: [8B little-endian IEEE754 float64]
//   TEXT:  [4B len][utf8 bytes]
//   BOOL:  [1B: 0x00=false 0x01=true]
//   BYTES: [4B len][bytes]
//   NULL:  (no payload)
//   SEMI:  [4B fieldCount] repeated: [4B kLen][key][encoded value]

func EncodeValue(v Value) []byte {
	if v.IsNull {
		return []byte{byte(TypeNull)}
	}
	switch v.Type {
	case TypeInt:
		b := make([]byte, 9)
		b[0] = byte(TypeInt)
		binary.LittleEndian.PutUint64(b[1:], uint64(v.IntVal))
		return b
	case TypeFloat:
		b := make([]byte, 9)
		b[0] = byte(TypeFloat)
		binary.LittleEndian.PutUint64(b[1:], math.Float64bits(v.FloatVal))
		return b
	case TypeText:
		tb := []byte(v.TextVal)
		b := make([]byte, 5+len(tb))
		b[0] = byte(TypeText)
		binary.LittleEndian.PutUint32(b[1:5], uint32(len(tb)))
		copy(b[5:], tb)
		return b
	case TypeBool:
		if v.BoolVal { return []byte{byte(TypeBool), 0x01} }
		return []byte{byte(TypeBool), 0x00}
	case TypeBytes:
		b := make([]byte, 5+len(v.BytesVal))
		b[0] = byte(TypeBytes)
		binary.LittleEndian.PutUint32(b[1:5], uint32(len(v.BytesVal)))
		copy(b[5:], v.BytesVal)
		return b
	case TypeSemi:
		return encodeSemi(v.SemiVal)
	default:
		return []byte{byte(TypeNull)}
	}
}

func DecodeValue(data []byte) (Value, int, error) {
	if len(data) == 0 {
		return NullValue, 0, fmt.Errorf("empty value bytes")
	}
	t := DataType(data[0])
	switch t {
	case TypeNull:
		return NullValue, 1, nil
	case TypeInt:
		if len(data) < 9 { return NullValue, 0, fmt.Errorf("short INT") }
		v := int64(binary.LittleEndian.Uint64(data[1:9]))
		return IntValue(v), 9, nil
	case TypeFloat:
		if len(data) < 9 { return NullValue, 0, fmt.Errorf("short FLOAT") }
		bits := binary.LittleEndian.Uint64(data[1:9])
		return FloatValue(math.Float64frombits(bits)), 9, nil
	case TypeText:
		if len(data) < 5 { return NullValue, 0, fmt.Errorf("short TEXT header") }
		l := int(binary.LittleEndian.Uint32(data[1:5]))
		if len(data) < 5+l { return NullValue, 0, fmt.Errorf("short TEXT body") }
		return TextValue(string(data[5 : 5+l])), 5 + l, nil
	case TypeBool:
		if len(data) < 2 { return NullValue, 0, fmt.Errorf("short BOOL") }
		return BoolValue(data[1] == 0x01), 2, nil
	case TypeBytes:
		if len(data) < 5 { return NullValue, 0, fmt.Errorf("short BYTES header") }
		l := int(binary.LittleEndian.Uint32(data[1:5]))
		if len(data) < 5+l { return NullValue, 0, fmt.Errorf("short BYTES body") }
		b := make([]byte, l)
		copy(b, data[5:5+l])
		return BytesValue(b), 5 + l, nil
	case TypeSemi:
		v, n, err := decodeSemi(data)
		return v, n, err
	default:
		return NullValue, 0, fmt.Errorf("unknown type byte %x", data[0])
	}
}

func encodeSemi(m map[string]Value) []byte {
	buf := []byte{byte(TypeSemi)}
	count := make([]byte, 4)
	binary.LittleEndian.PutUint32(count, uint32(len(m)))
	buf = append(buf, count...)
	for k, v := range m {
		kb := []byte(k)
		klen := make([]byte, 4)
		binary.LittleEndian.PutUint32(klen, uint32(len(kb)))
		buf = append(buf, klen...)
		buf = append(buf, kb...)
		buf = append(buf, EncodeValue(v)...)
	}
	return buf
}

func decodeSemi(data []byte) (Value, int, error) {
	if len(data) < 5 { return NullValue, 0, fmt.Errorf("short SEMI") }
	count := int(binary.LittleEndian.Uint32(data[1:5]))
	off := 5
	m := make(map[string]Value, count)
	for i := 0; i < count; i++ {
		if off+4 > len(data) { return NullValue, 0, fmt.Errorf("short SEMI key len") }
		kLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+kLen > len(data) { return NullValue, 0, fmt.Errorf("short SEMI key") }
		k := string(data[off : off+kLen])
		off += kLen
		v, n, err := DecodeValue(data[off:])
		if err != nil { return NullValue, 0, fmt.Errorf("SEMI value: %w", err) }
		m[k] = v
		off += n
	}
	return SemiValue(m), off, nil
}

// ── Column definition ─────────────────────────────────────────────────────────

type Column struct {
	Name       string
	Type       DataType
	PrimaryKey bool
	Nullable   bool
	Default    *Value // nil = no default
}

// ── Table schema ──────────────────────────────────────────────────────────────

type TableSchema struct {
	Name    string
	Columns []Column
}

func (t *TableSchema) PrimaryKeyCol() (Column, bool) {
	for _, c := range t.Columns {
		if c.PrimaryKey {
			return c, true
		}
	}
	return Column{}, false
}

func (t *TableSchema) ColByName(name string) (Column, int, bool) {
	for i, c := range t.Columns {
		if c.Name == name {
			return c, i, true
		}
	}
	return Column{}, -1, false
}

func (t *TableSchema) ColNames() []string {
	names := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = c.Name
	}
	return names
}

// ── Row — a map of column name → Value ───────────────────────────────────────

type Row struct {
	Schema *TableSchema
	Values map[string]Value
}

func NewRow(schema *TableSchema) *Row {
	return &Row{Schema: schema, Values: make(map[string]Value, len(schema.Columns))}
}

func (r *Row) Set(col string, val Value) { r.Values[col] = val }

func (r *Row) Get(col string) (Value, bool) {
	v, ok := r.Values[col]
	return v, ok
}

// PrimaryKeyBytes returns the serialized primary key for use as BTree key.
// Format: tableName + ":" + encoded_pk_value
func (r *Row) PrimaryKeyBytes() ([]byte, error) {
	pkCol, ok := r.Schema.PrimaryKeyCol()
	if !ok {
		return nil, fmt.Errorf("table %s has no primary key", r.Schema.Name)
	}
	pkVal, ok := r.Values[pkCol.Name]
	if !ok {
		return nil, fmt.Errorf("row missing primary key column %q", pkCol.Name)
	}
	prefix := []byte(r.Schema.Name + ":")
	encoded := EncodeValue(pkVal)
	key := make([]byte, len(prefix)+len(encoded))
	copy(key, prefix)
	copy(key[len(prefix):], encoded)
	return key, nil
}

// Encode serializes the full row to bytes for BTree value storage.
// Format: [4B colCount] repeated: [4B nameLen][name][encoded value]
func (r *Row) Encode() ([]byte, error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(r.Values)))
	for name, val := range r.Values {
		nb := []byte(name)
		nlen := make([]byte, 4)
		binary.LittleEndian.PutUint32(nlen, uint32(len(nb)))
		buf = append(buf, nlen...)
		buf = append(buf, nb...)
		buf = append(buf, EncodeValue(val)...)
	}
	return buf, nil
}

// DecodeRow deserializes bytes back into a Row.
func DecodeRow(schema *TableSchema, data []byte) (*Row, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("short row data")
	}
	count := int(binary.LittleEndian.Uint32(data[0:4]))
	row := NewRow(schema)
	off := 4
	for i := 0; i < count; i++ {
		if off+4 > len(data) { return nil, fmt.Errorf("short col name len") }
		nLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+nLen > len(data) { return nil, fmt.Errorf("short col name") }
		name := string(data[off : off+nLen])
		off += nLen
		val, n, err := DecodeValue(data[off:])
		if err != nil { return nil, fmt.Errorf("col %q: %w", name, err) }
		row.Values[name] = val
		off += n
	}
	return row, nil
}

// ── Schema serialization (for schema_store) ───────────────────────────────────
// Format: [4B nameLen][name][4B colCount] repeated cols:
//   [4B nameLen][name][1B type][1B flags: bit0=pk bit1=nullable]

func EncodeSchema(s *TableSchema) []byte {
	nb := []byte(s.Name)
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(nb)))
	buf = append(buf, nb...)
	cc := make([]byte, 4)
	binary.LittleEndian.PutUint32(cc, uint32(len(s.Columns)))
	buf = append(buf, cc...)
	for _, c := range s.Columns {
		cb := []byte(c.Name)
		cl := make([]byte, 4)
		binary.LittleEndian.PutUint32(cl, uint32(len(cb)))
		buf = append(buf, cl...)
		buf = append(buf, cb...)
		buf = append(buf, byte(c.Type))
		var flags uint8
		if c.PrimaryKey { flags |= 0x01 }
		if c.Nullable   { flags |= 0x02 }
		buf = append(buf, flags)
	}
	return buf
}

func DecodeSchema(data []byte) (*TableSchema, error) {
	if len(data) < 4 { return nil, fmt.Errorf("short schema") }
	nLen := int(binary.LittleEndian.Uint32(data[0:4]))
	if len(data) < 4+nLen+4 { return nil, fmt.Errorf("short schema name") }
	name := string(data[4 : 4+nLen])
	off := 4 + nLen
	colCount := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	cols := make([]Column, 0, colCount)
	for i := 0; i < colCount; i++ {
		if off+4 > len(data) { return nil, fmt.Errorf("short col") }
		cLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+cLen+2 > len(data) { return nil, fmt.Errorf("short col name") }
		cName := string(data[off : off+cLen])
		off += cLen
		dt   := DataType(data[off])
		flags := data[off+1]
		off += 2
		cols = append(cols, Column{
			Name:       cName,
			Type:       dt,
			PrimaryKey: flags&0x01 != 0,
			Nullable:   flags&0x02 != 0,
		})
	}
	return &TableSchema{Name: name, Columns: cols}, nil
}

func (v Value) EqualTo(other Value) bool {
	if v.Type != other.Type { return false }
	if v.IsNull && other.IsNull { return true }
	if v.IsNull || other.IsNull { return false }
	if v.Type == TypeBytes {
		if len(v.BytesVal) != len(other.BytesVal) { return false }
		for i := range v.BytesVal { if v.BytesVal[i] != other.BytesVal[i] { return false } }
		return true
	}
	cmp, err := v.Compare(other)
	return err == nil && cmp == 0
}