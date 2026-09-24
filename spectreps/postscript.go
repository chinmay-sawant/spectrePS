package spectreps

import "context"

// RunPostScript runs a PostScript program. The interpreter arrives in a later tag.
func (in *Instance) RunPostScript(ctx context.Context, src []byte, opt RunOptions) ([]PageImage, error) {
	_ = src
	_ = opt

	return nil, in.impl.Ready(ctx)
}
