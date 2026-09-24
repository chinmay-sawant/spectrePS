package spectreps

// Version is the current tag.
func Version() string {
	return "0.0.1"
}

// Instance is one Spectre session. There is no process-wide singleton.
type Instance struct {
	closed bool
}

// New returns an instance. Close is safe when no job ran.
func New() (*Instance, error) {
	return &Instance{}, nil
}

// Close releases the instance. A second Close on the same instance returns nil.
func (in *Instance) Close() error {
	in.closed = true
	return nil
}

// RunOptions selects the page for later raster jobs.
// Zero PageWidthPt selects 612. Zero PageHeightPt selects 792. Zero ResolutionDPI selects 72.
// This phase does not apply those defaults. The fields exist so callers can pass them.
type RunOptions struct {
	PageWidthPt   float64
	PageHeightPt  float64
	ResolutionDPI int
}
