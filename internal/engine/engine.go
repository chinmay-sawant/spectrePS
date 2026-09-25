package engine

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
