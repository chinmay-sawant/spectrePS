package spectreps

import "context"

// PageImage is RGB8 pixels. Row 0 is the top. len(Pixels) == Height*Stride. Stride >= Width*3.
type PageImage struct {
	Width  int
	Height int
	Stride int
	Pixels []byte
}

// RasterizePage paints one zero-based page. The pixmap device arrives in a later tag.
func (in *Instance) RasterizePage(ctx context.Context, doc *Document, pageIndex int, opt RunOptions) (PageImage, error) {
	return PageImage{}, stubErr(ctx)
}
