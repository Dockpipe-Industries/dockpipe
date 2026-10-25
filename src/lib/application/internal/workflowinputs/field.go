// Package workflowinputs discovers typed workflow fields and resolves their environment bindings.
package workflowinputs

// Field describes authored type metadata shared by execution and catalog projection.
// Presentation-specific JSON shapes and inferred UI controls belong to the catalog.
type Field struct {
	FieldName    string
	EnvName      string
	Type         string
	ElementType  string
	Description  string
	DefaultValue string
	Attributes   map[string]string
	Children     []Field
}

// Discovery retains usable fields and diagnostics from tolerant source discovery.
// Callers decide whether diagnostics should prevent use of partially discovered fields.
type Discovery struct {
	Fields      []Field
	Diagnostics []error
}
