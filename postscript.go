package spectreps

import "context"

func stubErr(ctx context.Context) error {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrNotImplemented
}

// RunPostScript runs a PostScript program. The interpreter arrives in a later tag.
func (in *Instance) RunPostScript(ctx context.Context, src []byte, opt RunOptions) ([]PageImage, error) {
	return nil, stubErr(ctx)
}
