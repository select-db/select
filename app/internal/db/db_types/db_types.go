package db_types

import (
	"database/sql"
	"encoding/json"
)

// JSONPayload is the generated type for `JSON` columns (e.g. mutation_commit.payload).
// It is an alias for any so the mutation flow can hold either the raw payload or a
// decoded DTO in the same field. Pinned via a sqlc column override because newer sqlc
// defaults `JSON` columns to json.RawMessage, which is not assignment-compatible.
type JSONPayload = any

/**
*	String
 */
type JSONNullString struct {
	sql.NullString
}

func (ns JSONNullString) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.String)
	}
	return json.Marshal(nil)
}
func (ns JSONNullString) Or(defaultVal string) string {
	if ns.Valid {
		return ns.String
	}
	return defaultVal
}
func NewJSONNullString(s string) JSONNullString {
	return JSONNullString{
		NullString: sql.NullString{
			String: s,
			Valid:  true,
		},
	}
}
func NewJSONNullStringFromPtr(s *string) JSONNullString {
	if s == nil {
		return JSONNullString{}
	}
	return NewJSONNullString(*s)
}
func (ns JSONNullString) Ptr() *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

