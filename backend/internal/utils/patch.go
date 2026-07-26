package utils

import "backend/db/db_types"

// PatchStr returns the payload string at key if present, otherwise existing.
func PatchStr(payload map[string]any, key string, existing db_types.JSONNullString) db_types.JSONNullString {
	if _, has := payload[key]; has {
		return db_types.NewJSONNullString(MapGetString(payload, key))
	}
	return existing
}

// PatchNullStr is like PatchStr but treats a nil value as SQL NULL.
func PatchNullStr(payload map[string]any, key string, existing db_types.JSONNullString) db_types.JSONNullString {
	if _, has := payload[key]; has {
		if p := MapGetStringPtr(payload, key); p != nil {
			return db_types.NewJSONNullString(*p)
		}
		return db_types.JSONNullString{}
	}
	return existing
}

// PatchStrDefault is like PatchStr but falls back to def when neither the
// payload carries the key nor existing holds a value — for a NOT NULL column
// with a DB default, so a newly-created row never writes SQL NULL.
func PatchStrDefault(payload map[string]any, key string, existing db_types.JSONNullString, def string) db_types.JSONNullString {
	if _, has := payload[key]; has {
		return db_types.NewJSONNullString(MapGetString(payload, key))
	}
	if existing.Valid {
		return existing
	}
	return db_types.NewJSONNullString(def)
}

// PatchUUID returns payloadValue if key is present in payload, otherwise existing.
func PatchUUID(payload map[string]any, key string, existing db_types.JSONNullUUID, payloadValue db_types.JSONNullUUID) db_types.JSONNullUUID {
	if _, has := payload[key]; has {
		return payloadValue
	}
	return existing
}
