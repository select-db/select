package toolkit

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"
)

func PrintJsonTesting(t *testing.T, label string, v any) {
	t.Helper()

	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal %s: %v", label, err)
	}

	re := regexp.MustCompile(`"([^"]+)"\s*:`)
	colored := re.ReplaceAllStringFunc(string(b), func(match string) string {
		return cyan + match[:len(match)-1] + reset + ":"
	})

	t.Logf("%s:\n%s", label, colored)
}

func PrintTesting(t *testing.T, label string, v any) {
	t.Helper()

	if v == nil {
		t.Logf("%s: <nil>", label)
		return
	}

	// Handle error types
	if err, ok := v.(error); ok {
		t.Logf("%s: error: %v", label, err)
		return
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	switch val.Kind() {
	case reflect.String:
		t.Logf("%s: %q", label, v)
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		PrintJsonTesting(t, label, v)
	case reflect.Ptr:
		if val.IsNil() {
			t.Logf("%s (%s): <nil>", label, typ.String())
		} else {
			t.Logf("%s (%s) ->", label, typ.String())
			PrintTesting(t, label, val.Elem().Interface())
		}
	default:
		t.Logf("%s (%s): %v", label, typ.String(), v)
	}
}
