package spectreps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

// ImageColor selects the color space of the image streams in a bitmap PDF.
type ImageColor int

const (
	// ImageColorRGB stores 24-bit RGB, 3 bytes per pixel, with /DeviceRGB.
	ImageColorRGB ImageColor = iota
	// ImageColorGray stores 8-bit gray, 1 byte per pixel, with /DeviceGray.
	ImageColorGray
	// ImageColorCMYK stores 32-bit CMYK, 4 bytes per pixel, with /DeviceCMYK.
	ImageColorCMYK
)

// ImagePDF writes a new PDF with one 24-bit RGB Flate image per page.
// It is ImagePDFColor with ImageColorRGB. dpi is the resolution the pages were
// painted at. Zero or less selects 72. A nil context panics. A canceled
// context returns ctx.Err().
func (in *Instance) ImagePDF(ctx context.Context, pages []PageImage, dpi float64) ([]byte, error) {
	return in.ImagePDFColor(ctx, pages, dpi, ImageColorRGB)
}

// ImagePDFColor writes a new PDF with one Flate image per page in color.
// ImageColorGray stores 8-bit gray and ImageColorCMYK stores 32-bit CMYK.
// The DeviceGray and DeviceCMYK conversions are in documentation/devices.md.
// dpi is the resolution the pages were painted at. Zero or less selects 72.
// A nil context panics. A canceled context returns ctx.Err().
func (in *Instance) ImagePDFColor(
	ctx context.Context,
	pages []PageImage,
	dpi float64,
	color ImageColor,
) ([]byte, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	out, err := pdfout.WriteImagesColor(ctx, graphicsImages(pages), dpi, imageColorSpace(color))
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

// imageColorSpace maps the public color to the writer color space. Any value
// other than the named constants selects RGB.
func imageColorSpace(color ImageColor) pdfout.ImageColorSpace {
	switch color {
	case ImageColorGray:
		return pdfout.ImageGray
	case ImageColorCMYK:
		return pdfout.ImageCMYK
	case ImageColorRGB:
		return pdfout.ImageRGB
	default:
		return pdfout.ImageRGB
	}
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
