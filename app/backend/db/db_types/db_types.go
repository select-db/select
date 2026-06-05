package db_types

import (
	"database/sql"
	"encoding/json"
)

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

