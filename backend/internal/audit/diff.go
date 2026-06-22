package audit

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode"
)

// Diff builds a before/after payload for a mutation, generically — any struct
// (e.g. a sqlc row or upsert-params) is turned into a snake_case map via JSON,
// so a new domain doesn't need hand-written per-column code. before may be nil
// (or a nil pointer) for a creation, in which case only "after" is included.
func Diff(before, after any) map[string]any {
	out := map[string]any{"after": toMap(after)}
	if !isNil(before) {
		out["before"] = toMap(before)
	}
	return out
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

// toMap marshals a value to JSON and back into a map, with top-level keys
// converted to snake_case. The db_types.JSONNull* wrappers marshal to their
// value or null, so nullable columns come out clean (not {Value,Valid}).
func toMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	out := make(map[string]any, len(raw))
	for k, val := range raw {
		out[toSnake(k)] = val
	}
	return out
}

// toSnake converts a Go field name (PascalCase, e.g. DbInstanceID) to snake_case
// (db_instance_id), inserting an underscore at lower→upper and acronym
// boundaries.
func toSnake(s string) string {
	rs := []rune(s)
	out := make([]rune, 0, len(rs)+4)
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))) {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			continue
		}
		out = append(out, r)
	}
	return strings.ToLower(string(out))
}
