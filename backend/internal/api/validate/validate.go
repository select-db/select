// Package validate is the write-body validator the generated REST create/update
// handlers call before a write reaches the syncer's Apply. It is the write-side
// counterpart of the query package: hand-written and entity-agnostic, driven by
// a per-resource Schema the apigen handler emitter generates from the same field
// IR that produces the OpenAPI request schema — so the API, its docs, and this
// validator never disagree on what a body may contain.
//
// It checks shape only: presence of required fields, per-kind types, closed
// enum membership, and (strict) rejection of unknown fields — collecting every
// problem into one Error, like a zod parse. Relational checks that need the
// database (foreign-key existence, cross-workspace scope, last-writer-wins) stay
// inside Apply; this runs first so the common client mistakes get a precise,
// field-level message instead of an opaque write failure.
package validate

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"backend/internal/api/query"
)

// Field is one writable field's validation spec, generated per resource.
type Field struct {
	Name     string
	Kind     query.Kind
	Required bool     // must be present and non-null on create
	Nullable bool     // may be explicitly null (SQL NULL)
	Enum     []string // closed value set (@app.values); empty = unconstrained
}

// Schema is a resource's writable shape.
type Schema struct {
	Fields []Field
}

// Issue is one field-level problem, in the response's fields array.
type Issue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error aggregates every issue found in one body (a zod-style parse: all at
// once, not first-fail). Nil means the body is valid.
type Error struct {
	Issues []Issue
}

func (e *Error) Error() string {
	parts := make([]string, len(e.Issues))
	for i, is := range e.Issues {
		parts[i] = is.Field + ": " + is.Message
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// ForCreate validates a create body: required fields must be present and every
// present field must satisfy its type/enum. The client supplies the id.
func ForCreate(s Schema, body map[string]any) *Error { return check(s, body, true) }

// ForUpdate validates an update (partial patch) body: nothing is required, but
// every present field must satisfy its type/enum. The id is set from the path by
// the handler before validation, so it is a known, valid field here.
func ForUpdate(s Schema, body map[string]any) *Error { return check(s, body, false) }

func check(s Schema, body map[string]any, create bool) *Error {
	var issues []Issue

	known := make(map[string]Field, len(s.Fields))
	names := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		known[f.Name] = f
		names = append(names, f.Name)
	}

	// Strict: reject fields the resource doesn't accept, with a "did you mean"
	// hint (reusing the read path's suggester). Iterate in sorted order so the
	// issue list is deterministic regardless of map iteration.
	unknown := make([]string, 0)
	for k := range body {
		if _, ok := known[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		msg := fmt.Sprintf("unknown field %q", k)
		if sug := query.Suggest(names, k); sug != "" {
			msg += fmt.Sprintf("; did you mean %q?", sug)
		}
		issues = append(issues, Issue{Field: k, Message: msg})
	}

	for _, f := range s.Fields {
		raw, present := body[f.Name]
		if !present {
			if create && f.Required {
				issues = append(issues, Issue{Field: f.Name, Message: "is required"})
			}
			continue
		}
		if raw == nil {
			if !f.Nullable {
				issues = append(issues, Issue{Field: f.Name, Message: "must not be null"})
			}
			continue
		}
		if msg := checkValue(f, raw); msg != "" {
			issues = append(issues, Issue{Field: f.Name, Message: msg})
		}
	}

	if len(issues) == 0 {
		return nil
	}
	return &Error{Issues: issues}
}

// checkValue returns "" if raw is a valid value for the field, else the reason.
// JSON decodes numbers as float64 and objects/arrays as map/slice, which is why
// the integer and json checks look the way they do.
func checkValue(f Field, raw any) string {
	switch f.Kind {
	case query.KindText, query.KindUUID, query.KindInet:
		s, ok := raw.(string)
		if !ok {
			return "must be a string"
		}
		switch f.Kind {
		case query.KindUUID:
			if _, err := uuid.Parse(s); err != nil {
				return "must be a valid uuid"
			}
		case query.KindInet:
			if net.ParseIP(s) == nil {
				return "must be a valid IP address"
			}
		}
		return enumCheck(f, s)
	case query.KindInt:
		n, ok := raw.(float64)
		if !ok || n != float64(int64(n)) {
			return "must be an integer"
		}
	case query.KindBool:
		if _, ok := raw.(bool); !ok {
			return "must be true or false"
		}
	case query.KindTime:
		s, ok := raw.(string)
		if !ok {
			return "must be an RFC3339 date-time string"
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return "must be an RFC3339 date-time string"
		}
	case query.KindJSON:
		// Any JSON value is acceptable for a json column.
	}
	return ""
}

// enumCheck validates a string value against a closed set, listing the allowed
// values on failure.
func enumCheck(f Field, s string) string {
	if len(f.Enum) == 0 {
		return ""
	}
	for _, v := range f.Enum {
		if s == v {
			return ""
		}
	}
	return "must be one of: " + strings.Join(f.Enum, ", ")
}
