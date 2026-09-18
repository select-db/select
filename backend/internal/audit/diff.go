package audit

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode"
)

// Diff builds a before/after payload from any structs (sqlc rows/params) via
// JSON, so a new domain needs no per-column code. before nil → only "after".
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

// toMap JSON-round-trips into a snake_case map. JSONNull* wrappers marshal to
// value-or-null, so nullable columns come out clean (not {Value,Valid}).
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

// toSnake: DbInstanceID -> db_instance_id (underscore at lower→upper and
// acronym boundaries).
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
