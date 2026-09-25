package graphics

import (
	"image"
	"image/color"
	"testing"
)

// TestPixmapDrawImageUnitSquare maps the image unit square over a 2 by 2
// pixmap at scale 2, so every device pixel samples one image pixel.
func TestPixmapDrawImageUnitSquare(t *testing.T) {
	pixmap := NewPixmap(2, 2)
	pic := stampImage(2, 2, []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255},
		{B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255},
	})
	pixmap.DrawImage(pic, Identity(), 2)
	img := shownPixmap(t, pixmap)
	checkStampPixel(t, img, 0, 0, 255, 0, 0)
	checkStampPixel(t, img, 1, 0, 0, 255, 0)
	checkStampPixel(t, img, 0, 1, 0, 0, 255)
	checkStampPixel(t, img, 1, 1, 255, 255, 255)
}

// TestPixmapDrawImageCTM checks that the current matrix places the image and
// that the scale multiplies it.
func TestPixmapDrawImageCTM(t *testing.T) {
	t.Run("translate", func(t *testing.T) {
		pixmap := NewPixmap(4, 4)
		pic := solidStamp(1, 1, color.RGBA{R: 255, A: 255})
		pixmap.DrawImage(pic, Identity().Translate(2, 1), 1)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 2, 2, 255, 0, 0)
		checkStampPixel(t, img, 1, 2, 255, 255, 255)
		checkStampPixel(t, img, 2, 1, 255, 255, 255)
		checkStampPixel(t, img, 3, 2, 255, 255, 255)
	})
	t.Run("scale", func(t *testing.T) {
		pixmap := NewPixmap(4, 4)
		pic := stampImage(2, 2, []color.RGBA{
			{R: 255, A: 255}, {G: 255, A: 255},
			{B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255},
		})
		pixmap.DrawImage(pic, Identity().Translate(0.5, 0.5), 2)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 1, 1, 255, 0, 0)
		checkStampPixel(t, img, 2, 1, 0, 255, 0)
		checkStampPixel(t, img, 1, 2, 0, 0, 255)
		checkStampPixel(t, img, 2, 2, 255, 255, 255)
		checkStampPixel(t, img, 0, 0, 255, 255, 255)
		checkStampPixel(t, img, 3, 3, 255, 255, 255)
	})
}

// TestPixmapDrawImageNearest scales a 2 by 2 image four times and checks the
// four 2 by 2 blocks. Every pixel in a block samples the same source pixel.
func TestPixmapDrawImageNearest(t *testing.T) {
	pixmap := NewPixmap(8, 8)
	pic := stampImage(2, 2, []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255},
		{B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255},
	})
	pixmap.DrawImage(pic, Identity(), 4)
	img := shownPixmap(t, pixmap)
	checkStampBlock(t, img, 0, 4, 1, 5, 255, 0, 0)
	checkStampBlock(t, img, 2, 4, 3, 5, 0, 255, 0)
	checkStampBlock(t, img, 0, 6, 1, 7, 0, 0, 255)
	checkStampBlock(t, img, 2, 6, 3, 7, 255, 255, 255)
	checkStampBlock(t, img, 0, 0, 7, 3, 255, 255, 255)
	checkStampBlock(t, img, 4, 4, 7, 7, 255, 255, 255)
}

// stampImage returns one image whose pixels are given in row-major order.
func stampImage(height, width int, pixels []color.RGBA) *image.RGBA {
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for idx, pixel := range pixels {
		pic.SetRGBA(idx%width, idx/width, pixel)
	}
	return pic
}

func solidStamp(height, width int, pixel color.RGBA) *image.RGBA {
	pixels := make([]color.RGBA, height*width)
	for idx := range pixels {
		pixels[idx] = pixel
	}
	return stampImage(height, width, pixels)
}

func shownPixmap(t *testing.T, pixmap *Pixmap) Image {
	t.Helper()
	pixmap.ShowPage()
	pages := pixmap.Pages()
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	return pages[0]
}

func checkStampPixel(t *testing.T, img Image, col, row int, red, green, blue byte) {
	t.Helper()
	idx := row*img.Stride + col*bytesPerPixel
	gotR := img.Pixels[idx]
	gotG := img.Pixels[idx+1]
	gotB := img.Pixels[idx+2]
	if gotR == red && gotG == green && gotB == blue {
		return
	}
	t.Fatalf(
		"pixel (%d,%d) = %d,%d,%d, want %d,%d,%d",
		col, row, gotR, gotG, gotB, red, green, blue,
	)
}

func checkStampBlock(
	t *testing.T,
	img Image,
	col0, row0, col1, row1 int,
	red, green, blue byte,
) {
	t.Helper()
	for row := row0; row <= row1; row++ {
		for col := col0; col <= col1; col++ {
			checkStampPixel(t, img, col, row, red, green, blue)
		}
	}
}
