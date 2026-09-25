package pdfout

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

const (
	scaleSourceWidth  = 64
	scaleSourceHeight = 48
	flatRed           = 180
	flatGreen         = 90
	flatBlue          = 40
	jpegTolerance     = 4
	highQuality       = 90
	lowQuality        = 20
)

func TestScaleImage(t *testing.T) {
	src := gradientImage(scaleSourceWidth, scaleSourceHeight)
	down := ScaleImage(src, 16, 12)
	if got := down.Bounds(); got != image.Rect(0, 0, 16, 12) {
		t.Fatalf("downscale bounds = %v, want 16x12 at the origin", got)
	}
	if !bytes.Equal(down.Pix, ScaleImage(src, 16, 12).Pix) {
		t.Fatal("two scale calls returned different bytes")
	}
	up := ScaleImage(src, 128, 96)
	if got := up.Bounds(); got != image.Rect(0, 0, 128, 96) {
		t.Fatalf("upscale bounds = %v, want 128x96 at the origin", got)
	}
	tiny := ScaleImage(src, 0, -5)
	if got := tiny.Bounds(); got != image.Rect(0, 0, 1, 1) {
		t.Fatalf("clamped bounds = %v, want 1x1 at the origin", got)
	}
}

func TestEncodeDCT(t *testing.T) {
	flat := flatImage(32, 24, color.RGBA{R: flatRed, G: flatGreen, B: flatBlue, A: 255})
	encoded := mustDCT(t, flat, highQuality)
	if !bytes.Equal(encoded, mustDCT(t, flat, highQuality)) {
		t.Fatal("two DCT calls returned different bytes")
	}
	if !bytes.Equal(mustDCT(t, flat, maxJPEGQuality+1), mustDCT(t, flat, maxJPEGQuality)) {
		t.Fatal("quality above 100 did not clamp")
	}
	if !bytes.Equal(mustDCT(t, flat, 0), mustDCT(t, flat, minJPEGQuality)) {
		t.Fatal("quality below 1 did not clamp")
	}
	decoded, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.Bounds(); got != flat.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", got, flat.Bounds())
	}
	for y := range flat.Bounds().Dy() {
		for x := range flat.Bounds().Dx() {
			checkFlatPixel(t, decoded, x, y)
		}
	}
	noisy := gradientImage(scaleSourceWidth, scaleSourceHeight)
	low := mustDCT(t, noisy, lowQuality)
	high := mustDCT(t, noisy, highQuality)
	if len(low) >= len(high) {
		t.Fatalf("quality %d wrote %d bytes, quality %d wrote %d", lowQuality, len(low), highQuality, len(high))
	}
}

func TestEncodeFlateRGB(t *testing.T) {
	src := gradientImage(scaleSourceWidth, scaleSourceHeight)
	encoded := mustFlateRGB(t, src)
	if !bytes.Equal(encoded, mustFlateRGB(t, src)) {
		t.Fatal("two Flate calls returned different bytes")
	}
	if !bytes.Equal(inflateImage(t, encoded), gradientRGB(src)) {
		t.Fatal("Flate round trip did not match the packed RGB rows")
	}
	region := src.SubImage(image.Rect(3, 2, 19, 14))
	if !bytes.Equal(inflateImage(t, mustFlateRGB(t, region)), gradientRGB(region)) {
		t.Fatal("subimage round trip did not match")
	}
	small := mustFlateRGB(t, ScaleImage(src, 16, 12))
	if len(small) >= len(encoded) {
		t.Fatalf("16x12 wrote %d bytes, %dx%d wrote %d", len(small), scaleSourceWidth, scaleSourceHeight, len(encoded))
	}
}

func mustDCT(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	got, err := EncodeDCT(img, quality)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustFlateRGB(t *testing.T, img image.Image) []byte {
	t.Helper()
	got, err := EncodeFlateRGB(img)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func checkFlatPixel(t *testing.T, decoded image.Image, x, y int) {
	t.Helper()
	red, green, blue, _ := decoded.At(x, y).RGBA()
	if !nearFlat(int(red>>8), flatRed) || !nearFlat(int(green>>8), flatGreen) || !nearFlat(int(blue>>8), flatBlue) {
		t.Fatalf("pixel (%d,%d) = %d %d %d", x, y, red>>8, green>>8, blue>>8)
	}
}

func nearFlat(got, want int) bool {
	if got < want {
		return want-got <= jpegTolerance
	}
	return got-want <= jpegTolerance
}

// flatImage returns one solid color over the whole rectangle.
func flatImage(width, height int, fill color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, fill)
		}
	}
	return img
}

// gradientImage gives every pixel a value derived from its coordinates.
func gradientImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			offset := img.PixOffset(x, y)
			img.Pix[offset] = gradientRed(x, y)
			img.Pix[offset+1] = gradientGreen(x, y)
			img.Pix[offset+2] = gradientBlue(x, y)
			img.Pix[offset+3] = 255
		}
	}
	return img
}

// gradientRGB lists the expected packed RGB bytes for the same coordinates.
func gradientRGB(img image.Image) []byte {
	bounds := img.Bounds()
	out := make([]byte, 0, bounds.Dx()*bounds.Dy()*imageChannels)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out = append(out, gradientRed(x, y), gradientGreen(x, y), gradientBlue(x, y))
		}
	}
	return out
}

func gradientRed(x, y int) byte {
	return byte((x*5 + y*3) % 256)
}

func gradientGreen(x, y int) byte {
	return byte((x*11 + y*7) % 256)
}

func gradientBlue(x, y int) byte {
	return byte((x*13 + y*17) % 256)
}
