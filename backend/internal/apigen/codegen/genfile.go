// Package codegen holds the primitives every apigen projection shares: the
// generated-file value type, the SQL->Go naming helpers, and the template
// renderer.
package codegen

// GenFile is one generated source file (name + content).
type GenFile struct {
	Name    string
	Content string
}
