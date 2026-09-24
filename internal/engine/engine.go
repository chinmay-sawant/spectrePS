package engine

import (
	"context"
	"errors"
)

// ErrNotImplemented is the job sentinel. Package spectreps re-exports this value.
var ErrNotImplemented = errors.New("spectreps: not implemented")

// Instance is one session. There is no process-wide singleton.
type Instance struct {
	closed bool
}

// New returns a session. Close is safe when no job ran.
func New() (*Instance, error) {
	return &Instance{closed: false}, nil
}

// Close releases the session. A second Close returns nil.
func (in *Instance) Close() error {
	if in.closed {
		return nil
	}
	in.closed = true
	return nil
}

// Ready checks the context before a job that this tag does not run yet.
// A nil context panics. A done context returns ctx.Err().
func (in *Instance) Ready(ctx context.Context) error {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrNotImplemented
}
