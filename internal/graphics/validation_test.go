package graphics

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// TestValidationPixmapStrokeFill locks stroke and fill edges: a negative
// width, a zero-length segment, a move-only path, winding against even-odd,
// and the color clamp.
func TestValidationPixmapStrokeFill(t *testing.T) {
	t.Run("negative width", func(t *testing.T) {
		segment := []Point{{X: 2, Y: 2, Move: true}, {X: 6, Y: 2}}
		positive := NewPixmap(8, 4)
		positive.Stroke(segment, 2, 0, 0, 0)
		negative := NewPixmap(8, 4)
		negative.Stroke(segment, -2, 0, 0, 0)
		if !bytes.Equal(shownPixmap(t, positive).Pixels, shownPixmap(t, negative).Pixels) {
			t.Fatal("negative width changed the stroke")
		}
	})
	t.Run("zero length segment", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		pixmap.Stroke([]Point{{X: 0.5, Y: 0.5, Move: true}, {X: 0.5, Y: 0.5}}, 1, 0, 0, 0)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 0, 1, 0, 0, 0)
		checkStampPixel(t, img, 1, 1, 255, 255, 255)
		checkStampPixel(t, img, 0, 0, 255, 255, 255)
		checkStampPixel(t, img, 1, 0, 255, 255, 255)
	})
	t.Run("move only path", func(t *testing.T) {
		pixmap := NewPixmap(4, 4)
		pixmap.Stroke([]Point{{X: 1, Y: 1, Move: true}}, 4, 0, 0, 0)
		pixmap.Fill([]Point{{X: 1, Y: 1, Move: true}}, 0, 0, 0, false)
		validationWhitePage(t, shownPixmap(t, pixmap))
	})
	t.Run("winding against even-odd", func(t *testing.T) {
		path := []Point{
			{X: 0, Y: 0, Move: true}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10},
			{X: 3, Y: 3, Move: true}, {X: 7, Y: 3}, {X: 7, Y: 7}, {X: 3, Y: 7},
		}
		nonzero := NewPixmap(10, 10)
		nonzero.Fill(path, 0, 0, 0, false)
		evenOdd := NewPixmap(10, 10)
		evenOdd.Fill(path, 0, 0, 0, true)
		nonzeroImg := shownPixmap(t, nonzero)
		evenOddImg := shownPixmap(t, evenOdd)
		checkStampPixel(t, nonzeroImg, 5, 4, 0, 0, 0)
		checkStampPixel(t, evenOddImg, 5, 4, 255, 255, 255)
		checkStampPixel(t, evenOddImg, 1, 1, 0, 0, 0)
	})
	t.Run("color clamp", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		square := []Point{{X: 0, Y: 0, Move: true}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}}
		pixmap.Fill(square, -1, 2, 0.5, false)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 0, 0, 0, 255, 128)
		checkStampPixel(t, img, 1, 1, 0, 255, 128)
	})
}

// TestValidationDrawImageEdges locks DrawImage on a nil image, zero-size
// bounds, a singular CTM, and an image that runs off the page.
func TestValidationDrawImageEdges(t *testing.T) {
	t.Run("nil image", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		pixmap.DrawImage(nil, Identity(), 1)
		validationWhitePage(t, shownPixmap(t, pixmap))
	})
	t.Run("zero size bounds", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		pixmap.DrawImage(image.NewRGBA(image.Rect(0, 0, 0, 0)), Identity(), 1)
		validationWhitePage(t, shownPixmap(t, pixmap))
	})
	t.Run("singular ctm", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		pixmap.DrawImage(solidStamp(1, 1, color.RGBA{R: 255, A: 255}), Matrix{}, 1)
		validationWhitePage(t, shownPixmap(t, pixmap))
	})
	t.Run("partly off page", func(t *testing.T) {
		pixmap := NewPixmap(4, 4)
		pic := solidStamp(4, 4, color.RGBA{R: 255, A: 255})
		pixmap.DrawImage(pic, Identity().Translate(0.5, 0.5), 4)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 2, 0, 255, 0, 0)
		checkStampPixel(t, img, 3, 1, 255, 0, 0)
		checkStampPixel(t, img, 0, 0, 255, 255, 255)
		checkStampPixel(t, img, 2, 2, 255, 255, 255)
	})
}

func validationWhitePage(t *testing.T, img Image) {
	t.Helper()
	for i, value := range img.Pixels {
		if value != whiteByte {
			t.Fatalf("pixels[%d] = %d, want white", i, value)
		}
	}
}
