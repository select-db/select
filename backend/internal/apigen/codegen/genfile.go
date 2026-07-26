// Package codegen holds the primitives every apigen projection shares: the
// generated-file value type and the SQL->Go naming helpers.
package codegen

// GenFile is one generated source file (name + content).
type GenFile struct {
	Name    string
	Content string
}
