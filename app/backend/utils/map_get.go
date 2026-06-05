package utils

import "encoding/json"

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
