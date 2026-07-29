package codegen

import (
	"strings"
	"text/template"
)

// Render executes a code template into a string. The apigen templates are
// compile-time constants, so an execution failure is a programming error, not a
// runtime condition — Render panics rather than returning an error. Callers run
// the result through go/format.Source themselves (each emitter reports format
// errors against its own file name).
func Render(t *template.Template, data any) string {
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		panic(err)
	}
	return b.String()
}
