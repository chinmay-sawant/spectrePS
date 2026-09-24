package spectreps

import (
	"errors"
	"fmt"
)

// ErrNotImplemented is returned by interpreter methods until their phase lands.
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
// Example with a position: Error: /stackunderflow in add at box.ps:3:5
// When Filename is empty the position is unknown and the " at file:line:col" tail is omitted.
func (e JobError) Error() string {
	text := "Error: /" + e.Msg + " in " + e.Op
	if e.Filename == "" {
		return text
	}
	return fmt.Sprintf("%s at %s:%d:%d", text, e.Filename, e.Line, e.Column)
}
