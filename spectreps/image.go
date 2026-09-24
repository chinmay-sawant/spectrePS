package spectreps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

// ImagePDF writes a new PDF with one 24-bit RGB Flate image per page.
// dpi is the resolution the pages were painted at. Zero or less selects 72.
// A nil context panics. A canceled context returns ctx.Err().
func (in *Instance) ImagePDF(ctx context.Context, pages []PageImage, dpi float64) ([]byte, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	out, err := pdfout.WriteImages(ctx, graphicsImages(pages), dpi)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

func graphicsImages(pages []PageImage) []graphics.Image {
	out := make([]graphics.Image, len(pages))
	for i, page := range pages {
		out[i] = graphics.Image{
			Width:  page.Width,
			Height: page.Height,
			Stride: page.Stride,
			Pixels: page.Pixels,
		}
	}
	return out
}
