package ps

// Error is one PostScript error. Name has no slash. Op is the operator, or empty during scanning.
type Error struct {
	Name string
	Op   string
}

func (e *Error) Error() string {
	return e.Name
}

func errOf(name, op string) error {
	return &Error{Name: name, Op: op}
}
