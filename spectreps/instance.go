package spectreps

import "github.com/chinmay-sawant/spectrePS/internal/engine"

// Version is the current tag.
func Version() string {
	return "0.0.4"
}

// Instance is one Spectre session. There is no process-wide singleton.
type Instance struct {
	impl *engine.Instance
}

// New returns an instance. Close is safe when no job ran.
func New() (*Instance, error) {
	impl, err := engine.New()
	if err != nil {
		return nil, err
	}
	return &Instance{impl: impl}, nil
}

// Close releases the instance. A second Close on the same instance returns nil.
func (in *Instance) Close() error {
	return in.impl.Close()
}
