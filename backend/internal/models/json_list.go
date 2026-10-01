// backend/internal/models/json_list.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONList stores a slice in a Postgres jsonb column. A nil list is written
// as [] and read back as an empty, non-nil list, and always serialises as
// [] (never null), so API consumers can iterate without a nil check.
type JSONList[T any] []T

// Value implements driver.Valuer. The JSON is handed over as text, which the
// jsonb column type accepts.
func (l JSONList[T]) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]T(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (l *JSONList[T]) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = JSONList[T]{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("JSONList: unsupported scan type %T", src)
	}
	out := []T{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("JSONList: %w", err)
	}
	if out == nil {
		out = []T{}
	}
	*l = out
	return nil
}

// MarshalJSON implements json.Marshaler. Marshalling the underlying slice
// type avoids recursing into this method.
func (l JSONList[T]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(l))
}
