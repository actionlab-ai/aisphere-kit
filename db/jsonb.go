package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONB is a generic column type that marshals/unmarshals any Go value to/from
// a JSON column. It works with both Postgres JSONB and MySQL JSON columns.
//
// Usage with GORM:
//
//	type Doc struct {
//	    ID    string `gorm:"primaryKey"`
//	    Blob  db.JSONB[Payload] `gorm:"type:jsonb"`
//	}
//
// JSONB is safe to embed by value; do not embed by pointer unless you want
// the column to be NULL-able. An empty JSONB[T]{} marshals to the JSON
// literal `null`.
type JSONB[T any] struct {
	Data T
	// Null indicates whether the column value was SQL NULL on scan.
	// Set to true by Scan when the database returns NULL. Setting
	// Null=true before insert/updated causes GORM to write NULL.
	Null bool
}

// Value implements driver.Valuer.
func (j JSONB[T]) Value() (driver.Value, error) {
	if j.Null {
		return nil, nil
	}
	b, err := json.Marshal(j.Data)
	if err != nil {
		return nil, fmt.Errorf("jsonb marshal: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (j *JSONB[T]) Scan(src any) error {
	if src == nil {
		j.Null = true
		var zero T
		j.Data = zero
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("jsonb: unsupported scan source %T", src)
	}
	if len(b) == 0 {
		j.Null = true
		var zero T
		j.Data = zero
		return nil
	}
	j.Null = false
	if err := json.Unmarshal(b, &j.Data); err != nil {
		return fmt.Errorf("jsonb unmarshal: %w", err)
	}
	return nil
}

// MarshalJSON returns the underlying value's JSON encoding.
func (j JSONB[T]) MarshalJSON() ([]byte, error) {
	if j.Null {
		return []byte("null"), nil
	}
	return json.Marshal(j.Data)
}

// UnmarshalJSON delegates to the underlying value.
func (j *JSONB[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		j.Null = true
		var zero T
		j.Data = zero
		return nil
	}
	j.Null = false
	return json.Unmarshal(b, &j.Data)
}

// ErrJSONBNull is returned by callers that explicitly want to reject NULL.
var ErrJSONBNull = errors.New("jsonb: column is null")

// MustGet returns the underlying value or ErrJSONBNull when the column was NULL.
func (j JSONB[T]) MustGet() (T, error) {
	if j.Null {
		var zero T
		return zero, ErrJSONBNull
	}
	return j.Data, nil
}
