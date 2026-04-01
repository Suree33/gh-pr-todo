package types

// BUG: consider attaching source language metadata if output ever needs it.
// Represents a TODO comment found in a diff.
type TODO struct {
	Filename string
	// The line number in the file.
	Line int
	// The whole comment line.
	Comment string
	// TODO, FIXME, HACK, NOTE, etc.
	Type string
}
