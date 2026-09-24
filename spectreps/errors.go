package spectreps

import (
	"fmt"

	"github.com/chinmay-sawant/spectrePS/internal/engine"
)

// ErrNotImplemented is returned by interpreter methods until their phase lands.
var ErrNotImplemented = engine.ErrNotImplemented

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
