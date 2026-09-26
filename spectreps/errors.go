package spectreps

import (
	"errors"
	"fmt"
)

// ErrNotImplemented is the legacy job sentinel. No method returns it: every
// job either runs or returns a JobError, and the CLI maps a JobError to exit
// 1, never this value. It stays exported for callers that still compare
// against it.
var ErrNotImplemented = errors.New("spectreps: not implemented")

// JobError is one interpreter error.
// Op is the operator name without a slash. Msg is the error name without a slash.
// The slash appears only in Error.
type JobError struct {
	Op       string
	Msg      string
	Filename string
	Line     int
	Column   int
}

// Error prints one line.
// With a filename: Error: /stackunderflow in add at box.ps:3:5
// An empty Filename omits the " at file:line:col" tail.
func (e JobError) Error() string {
	text := "Error: /" + e.Msg + " in " + e.Op
	if e.Filename == "" {
		return text
	}
	return fmt.Sprintf("%s at %s:%d:%d", text, e.Filename, e.Line, e.Column)
}
