package syncer

import (
	"embed"
	"text/template"
)

// templateFS holds the code templates as editable files under templates/,
// embedded at build time. Editing a template means editing its .tmpl file, not a
// Go string literal. Go templates (.go.tmpl) get the generated-file header
// prepended; SQL templates (.sql.tmpl, sqlc query files) do not.
//
//go:embed templates
var templateFS embed.FS

func mustParse(name string) *template.Template {
	return template.Must(template.New(name).Parse(genHeader + readTemplate(name)))
}

func mustParseSQL(name string) *template.Template {
	return template.Must(template.New(name).Parse(readTemplate(name)))
}

func readTemplate(name string) string {
	b, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic(err)
	}
	return string(b)
}
