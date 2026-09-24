package pdf

// Error is one PDF job error.
// Op is the operator or construct name without a slash.
// Name is the error name without a slash.
type Error struct {
	Op   string
	Name string
}

// Error returns the error name. The public API adds the slash form.
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.Name
}

// NewError returns a job error with the given operator and name.
func NewError(opName, errName string) error {
	return &Error{Op: opName, Name: errName}
}
