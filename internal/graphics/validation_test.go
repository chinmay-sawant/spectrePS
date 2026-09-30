package graphics

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
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

// TestValidationFillBoundingBox locks that bounding the fill scan by the path's
// own box does not change the result. The bound is an optimisation, so the test
// asserts the invariant it must preserve: every painted pixel lies inside the
// path's bounding box, and the path's own interior is painted. Comparing two
// pixmaps of different heights is not valid, because the row index is flipped
// against the height.
func TestValidationFillBoundingBox(t *testing.T) {
	path := []Point{
		{X: 2, Y: 2, Move: true}, {X: 6, Y: 2}, {X: 6, Y: 5}, {X: 2, Y: 5},
	}
	pixmap := NewPixmap(600, 700)
	pixmap.Fill(path, 0, 0, 0, false)
	img := shownPixmap(t, pixmap)

	painted := 0
	for row := range img.Height {
		for column := range img.Width {
			if !validationPixelPainted(img, row, column) {
				continue
			}
			painted++
			validationAssertInsidePathBox(t, img, row, column)
		}
	}
	if painted == 0 {
		t.Fatal("nothing was painted, so the bound skipped the path itself")
	}
}

func validationPixelPainted(img Image, row, column int) bool {
	index := row*img.Stride + column*bytesPerPixel
	return img.Pixels[index] != whiteByte ||
		img.Pixels[index+1] != whiteByte ||
		img.Pixels[index+2] != whiteByte
}

func validationAssertInsidePathBox(t *testing.T, img Image, row, column int) {
	t.Helper()
	// The pixel centre has to be inside the path's box, with one pixel of slack
	// for the boundary itself.
	centerColumn := float64(column) + pixelCenter
	centerRow := float64(img.Height-1-row) + pixelCenter
	if centerColumn < 1 || centerColumn > 7 || centerRow < 1 || centerRow > 6 {
		t.Fatalf("pixel (%d,%d) is painted and lies outside the path box", column, row)
	}
}

// TestValidationFillLargePathTerminates locks that a path with many points is
// bounded rather than walking every point for every pixel of the page. Before
// the bound, a 6,638-point path on a 612 by 792 page was 3.2 billion cross
// tests and read as a hang.
func TestValidationFillLargePathTerminates(t *testing.T) {
	path := make([]Point, 0, 7000)
	path = append(path, Point{X: 1, Y: 1, Move: true})
	for i := range 6998 {
		path = append(path, Point{X: 1 + float64(i%20)*0.1, Y: 1 + float64(i%30)*0.1})
	}
	pixmap := NewPixmap(612, 792)
	start := time.Now()
	pixmap.Fill(path, 0, 0, 0, false)
	elapsed := time.Since(start)
	img := shownPixmap(t, pixmap)
	if img.Width != 612 || img.Height != 792 {
		t.Fatalf("image %d by %d", img.Width, img.Height)
	}
	// Measured: 0.009s bounded, 17.3s unbounded, so this threshold is not
	// close to either result and does not depend on the machine being fast.
	if elapsed > fillBudget {
		t.Fatalf("fill took %s, want under %s: the scan is not bounded by the path box", elapsed, fillBudget)
	}
}

// fillBudget is the wall-clock ceiling for the large-path test. It exists to
// catch a fill scan that is unbounded again, not to measure performance.
const fillBudget = 5 * time.Second
