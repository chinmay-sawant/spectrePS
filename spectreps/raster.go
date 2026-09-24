package spectreps

import "context"

// RasterizePage paints one zero-based page. The pixmap device arrives in a later tag.
func (in *Instance) RasterizePage(
	ctx context.Context,
	doc *Document,
	pageIndex int,
	opt RunOptions,
) (PageImage, error) {
	_ = doc
	_ = pageIndex
	_ = opt

	return PageImage{
		Width:  0,
		Height: 0,
		Stride: 0,
		Pixels: nil,
	}, in.impl.Ready(ctx)
}
