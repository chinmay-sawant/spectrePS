package pdfout

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

const (
	minScaleSize   = 1
	minJPEGQuality = 1
	maxJPEGQuality = 100
	colorByteShift = 8
)

// ScaleImage resamples img to width by height with the CatmullRom kernel.
// Width and height below 1 clamp to 1. The result is opaque RGBA at the origin.
func ScaleImage(img image.Image, width, height int) *image.RGBA {
	width = max(width, minScaleSize)
	height = max(height, minScaleSize)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// EncodeDCT encodes img as baseline JPEG at quality, clamped to 1 through 100.
func EncodeDCT(img image.Image, quality int) ([]byte, error) {
	quality = min(max(quality, minJPEGQuality), maxJPEGQuality)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeFlateRGB writes tightly packed RGB rows, top row first, wrapped in zlib.
// Row width is bounds.Dx()*3, so stride padding in the source is dropped.
func EncodeFlateRGB(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	row := make([]byte, bounds.Dx()*imageChannels)
	return withFlateWriter(func(writer *zlib.Writer) error {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				red, green, blue, _ := img.At(x, y).RGBA()
				at := (x - bounds.Min.X) * imageChannels
				row[at] = byte(red >> colorByteShift)
				row[at+1] = byte(green >> colorByteShift)
				row[at+2] = byte(blue >> colorByteShift)
			}
			if _, err := writer.Write(row); err != nil {
				return err
			}
		}
		return nil
	})
}
