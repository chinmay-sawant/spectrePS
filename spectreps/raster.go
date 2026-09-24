package spectreps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// RasterizePage paints one zero-based page.
func (in *Instance) RasterizePage(
	ctx context.Context,
	doc *Document,
	pageIndex int,
	opt RunOptions,
) (PageImage, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return zeroPageImage(), err
	}
	_ = in
	if pageOutOfRange(doc, pageIndex) {
		return zeroPageImage(), rasterRange()
	}
	width, height, scale, err := pageGeometry(opt)
	if err != nil {
		return zeroPageImage(), err
	}
	pixmap := graphics.NewPixmap(width, height)
	if err := doc.file.PaintPage(ctx, pageIndex, pixmap, scale); err != nil {
		return zeroPageImage(), asPDFJobError(err)
	}
	pixmap.ShowPage()
	return pageImages(pixmap.Pages())[0], nil
}

func pageOutOfRange(doc *Document, pageIndex int) bool {
	if doc == nil || doc.file == nil {
		return true
	}
	if pageIndex < 0 {
		return true
	}
	return pageIndex >= doc.file.PageCount()
}

func zeroPageImage() PageImage {
	return PageImage{
		Width:  0,
		Height: 0,
		Stride: 0,
		Pixels: nil,
	}
}

func rasterRange() JobError {
	return JobError{Op: "RasterizePage", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
}
