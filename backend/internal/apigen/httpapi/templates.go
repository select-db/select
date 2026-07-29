package httpapi

import (
	"embed"
	"text/template"
)

// templateFS holds the code templates as editable files under templates/ (one
// per generated file), embedded at build time. Editing a template means editing
// its .tmpl file, not a Go string literal.
//
//go:embed templates
var templateFS embed.FS

// mustParse loads template <name> from templates/ and parses it with the
// generated-file header prepended. The templates are embedded compile-time
// constants, so a read/parse failure is a build error, not a runtime one.
func mustParse(name string) *template.Template {
	b, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic(err)
	}
	return template.Must(template.New(name).Parse(genHeader + string(b)))
}
