package syncer

// PayloadHasDeletedAt returns true if the payload has a non-null deleted_at (server sent a soft-deleted row).
func PayloadHasDeletedAt(payload map[string]any) bool {
	v, ok := payload["deleted_at"]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return t != ""
	case float64:
		return t != 0
	default:
		return true
	}
}
