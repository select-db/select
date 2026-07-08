package utils

import (
	"encoding/json"
	"strconv"
)

// MapGetIntOr returns the value at key coerced to int, or def when missing or
// not numeric. JSON-decoded numbers arrive as float64, so that case is handled
// explicitly.
func MapGetIntOr(m map[string]interface{}, key string, def int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return def
}

// Returns the value at key as a string, or "" if missing or not a string.
func MapGetString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Returns the value at key as *string, or nil if missing/nil.
func MapGetStringPtr(m map[string]interface{}, key string) *string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return &t
	case nil:
		return nil
	default:
		b, _ := json.Marshal(v)
		s := string(b)
		return &s
	}
}
