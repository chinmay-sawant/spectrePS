package spectreps

import (
	"context"
	"errors"
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/ps"
)

const (
	defaultPageWidth  = 612
	defaultPageHeight = 792
	defaultDPI        = 72
	pointsPerInch     = 72
	maxPageSide       = 20000
	maxPagePixels     = 40000000
)

// RunPostScript runs a PostScript program and returns one image per page.
func (in *Instance) RunPostScript(ctx context.Context, src []byte, opt RunOptions) ([]PageImage, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	width, height, scale, err := pageGeometry(opt)
	if err != nil {
		return nil, err
	}
	pixmap := graphics.NewPixmap(width, height)
	interp := ps.NewInterp()
	interp.UsePixmap(pixmap, scale)
	if err := interp.Run(ctx, src); err != nil {
		return nil, asJobError(err)
	}
	return pageImages(pixmap.Pages()), nil
}

func pageGeometry(opt RunOptions) (int, int, float64, error) {
	width := opt.PageWidthPt
	height := opt.PageHeightPt
	dpi := opt.ResolutionDPI
	if width == 0 {
		width = defaultPageWidth
	}
	if height == 0 {
		height = defaultPageHeight
	}
	if dpi == 0 {
		dpi = defaultDPI
	}
	if width < 0 || height < 0 || dpi < 0 {
		return 0, 0, 0, rasterLimit()
	}
	scale := float64(dpi) / pointsPerInch
	pw := int(math.Round(width * scale))
	ph := int(math.Round(height * scale))
	if pageOverCap(pw, ph) {
		return 0, 0, 0, rasterLimit()
	}
	return pw, ph, scale, nil
}

func pageOverCap(width, height int) bool {
	if width <= 0 || height <= 0 || width > maxPageSide || height > maxPageSide {
		return true
	}
	return int64(width)*int64(height) > maxPagePixels
}

func rasterLimit() JobError {
	return JobError{Op: "raster", Msg: "limitcheck", Filename: "", Line: 0, Column: 0}
}

func asJobError(err error) error {
	var psErr *ps.Error
	if errors.As(err, &psErr) {
		return JobError{Op: psErr.Op, Msg: psErr.Name, Filename: "", Line: 0, Column: 0}
	}
	return err
}

func pageImages(pages []graphics.Image) []PageImage {
	out := make([]PageImage, len(pages))
	for i, page := range pages {
		out[i] = PageImage{
			Width:  page.Width,
			Height: page.Height,
			Stride: page.Stride,
			Pixels: page.Pixels,
		}
	}
	return out
}
