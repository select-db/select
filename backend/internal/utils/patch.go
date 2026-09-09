package utils

import "backend/db/db_types"

// PatchNullStr merges a nullable text column: the payload's value when the key
// is present, SQL NULL when it is present and null, the existing value when it
// is absent. That three-way answer is why nullable columns keep a wrapper.
func PatchNullStr(payload map[string]any, key string, existing db_types.JSONNullString) db_types.JSONNullString {
	if _, has := payload[key]; has {
		if p := MapGetStringPtr(payload, key); p != nil {
			return db_types.NewJSONNullString(*p)
		}
		return db_types.JSONNullString{}
	}
	return existing
}

// PatchValue is the plain-typed merge: the payload's value when the key is
// present, the existing one when it is not.
//
// A NOT NULL column generates a plain Go type rather than a JSONNull wrapper,
// so it merges by value. The wrapper-typed helpers above stay for the columns
// that really are nullable, where "absent" and "NULL" are different answers.
func PatchValue[T any](payload map[string]any, key string, existing, payloadValue T) T {
	if _, has := payload[key]; has {
		return payloadValue
	}
	return existing
}

// PatchStrValueDefault is PatchValue for a NOT NULL text column carrying a DB
// default: a client that omits the key keeps what is stored, and a row holding
// the zero value falls back to the default rather than writing an empty string.
func PatchStrValueDefault(payload map[string]any, key, existing, def string) string {
	if _, has := payload[key]; has {
		return MapGetString(payload, key)
	}
	if existing != "" {
		return existing
	}
	return def
}
