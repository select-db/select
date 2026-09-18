package validate

import (
	"strings"
	"testing"

	"backend/internal/api/query"
)

func roleSchema() Schema {
	return Schema{Fields: []Field{
		{Name: "id", Kind: query.KindUUID, Required: true},
		{Name: "name", Kind: query.KindText, Required: true},
		{Name: "note", Kind: query.KindText, Nullable: true},
		{Name: "effect", Kind: query.KindText, Enum: []string{"allow", "deny"}},
		{Name: "seats", Kind: query.KindInt},
		{Name: "active", Kind: query.KindBool},
		{Name: "seen_at", Kind: query.KindTime, Nullable: true},
		{Name: "role_id", Kind: query.KindUUID},
	}}
}

const goodUUID = "11111111-1111-1111-1111-111111111111"

// issueFor returns the message for a field, or "" if that field has no issue.
func issueFor(e *Error, field string) string {
	if e == nil {
		return ""
	}
	for _, is := range e.Issues {
		if is.Field == field {
			return is.Message
		}
	}
	return ""
}

func TestForCreateValid(t *testing.T) {
	body := map[string]any{
		"id":     goodUUID,
		"name":   "admin",
		"effect": "allow",
		"seats":  float64(5),
		"active": true,
	}
	if err := ForCreate(roleSchema(), body); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestForCreateRequiredMissing(t *testing.T) {
	// id and name are required on create; omitting both yields two issues.
	err := ForCreate(roleSchema(), map[string]any{})
	if err == nil {
		t.Fatal("expected required-field errors")
	}
	if m := issueFor(err, "id"); m != "is required" {
		t.Fatalf("id: got %q", m)
	}
	if m := issueFor(err, "name"); m != "is required" {
		t.Fatalf("name: got %q", m)
	}
}

func TestForUpdateSkipsRequired(t *testing.T) {
	// update is a partial patch: no field is required, present ones still checked.
	if err := ForUpdate(roleSchema(), map[string]any{"name": "renamed"}); err != nil {
		t.Fatalf("partial update should be valid, got %v", err)
	}
}

func TestTypeErrors(t *testing.T) {
	err := ForCreate(roleSchema(), map[string]any{
		"id":      "not-a-uuid",
		"name":    123,        // wrong type
		"seats":   float64(3), // ok
		"active":  "yes",      // wrong type
		"seen_at": "nope",     // bad date-time
		"role_id": goodUUID,   // ok
	})
	if err == nil {
		t.Fatal("expected type errors")
	}
	checks := map[string]string{
		"id":      "must be a valid uuid",
		"name":    "must be a string",
		"active":  "must be true or false",
		"seen_at": "must be an RFC3339 date-time string",
	}
	for field, want := range checks {
		if got := issueFor(err, field); got != want {
			t.Fatalf("%s: got %q want %q", field, got, want)
		}
	}
	// name was also required; it should report the type problem, not a duplicate.
	if n := len(err.Issues); n != 4 {
		t.Fatalf("expected 4 issues, got %d: %v", n, err.Issues)
	}
}

func TestIntegerRejectsFraction(t *testing.T) {
	err := ForCreate(roleSchema(), map[string]any{"id": goodUUID, "name": "n", "seats": float64(2.5)})
	if got := issueFor(err, "seats"); got != "must be an integer" {
		t.Fatalf("seats: got %q", got)
	}
}

func TestEnumViolation(t *testing.T) {
	err := ForCreate(roleSchema(), map[string]any{"id": goodUUID, "name": "n", "effect": "maybe"})
	got := issueFor(err, "effect")
	if !strings.Contains(got, "must be one of: allow, deny") {
		t.Fatalf("effect: got %q", got)
	}
}

func TestNullHandling(t *testing.T) {
	// nullable field may be null; non-nullable may not.
	err := ForCreate(roleSchema(), map[string]any{"id": goodUUID, "name": nil, "note": nil})
	if got := issueFor(err, "name"); got != "must not be null" {
		t.Fatalf("name null: got %q", got)
	}
	if got := issueFor(err, "note"); got != "" {
		t.Fatalf("nullable note should accept null, got %q", got)
	}
}

func TestStrictUnknownFieldWithSuggestion(t *testing.T) {
	err := ForCreate(roleSchema(), map[string]any{"id": goodUUID, "name": "n", "nam": "typo"})
	got := issueFor(err, "nam")
	if !strings.Contains(got, `unknown field "nam"`) || !strings.Contains(got, `did you mean "name"?`) {
		t.Fatalf("unknown field: got %q", got)
	}
}

func TestDeterministicOrder(t *testing.T) {
	// Two runs over the same unknown-field-heavy body must produce identical order.
	body := map[string]any{"id": goodUUID, "name": "n", "zzz": 1, "aaa": 2, "mmm": 3}
	a := ForCreate(roleSchema(), body).Error()
	b := ForCreate(roleSchema(), body).Error()
	if a != b {
		t.Fatalf("non-deterministic:\n%s\n%s", a, b)
	}
}
