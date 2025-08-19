package types

// TODO represents a TODO comment found in a diff
type TODO struct {
	Filename string
	Line     int
	Comment  string
	Type     string // TODO, FIXME, HACK, NOTE, etc.
}