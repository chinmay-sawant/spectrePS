package pdf

import (
	"image"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestPaintClip proves W intersects the current path into later fills and
// strokes. A pixel inside the clip keeps the mark and a pixel outside it does
// not.
func TestPaintClip(t *testing.T) {
	checkClipFill(t)
	checkClipStroke(t)
	checkClipImage(t)
}

func checkClipFill(t *testing.T) {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	src := "0 0 5 20 re W n 1 0 0 rg 0 0 20 20 re f"
	if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 4, 10, 255, 0, 0)
	wantPixel(t, img, 5, 10, whiteByte, whiteByte, whiteByte)
}

func checkClipStroke(t *testing.T) {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	src := "0 0 5 20 re W n 4 w 0 0 m 20 0 l S"
	if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 4, 19, 0, 0, 0)
	wantPixel(t, img, 5, 19, whiteByte, whiteByte, whiteByte)
	wantPixel(t, img, 4, 17, whiteByte, whiteByte, whiteByte)
}

func checkClipImage(t *testing.T) {
	t.Helper()
	file := doPage(t, "0 0 1 1 re W n 2 0 0 2 0 0 cm /Im0 Do",
		"<< /XObject << /Im0 5 0 R >> >>", rgbImageBody(t))
	pixmap := graphics.NewPixmap(2, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 0, 1, 0, 0, 255)
	wantPixel(t, img, 1, 0, whiteByte, whiteByte, whiteByte)
}

// TestPaintClipEvenOdd proves W* uses the even-odd rule, so the hole of two
// nested rectangles is outside the clip.
func TestPaintClipEvenOdd(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	src := "0 0 15 15 re 5 5 5 5 re W* n 1 0 0 rg 0 0 20 20 re f"
	if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 2, 10, 255, 0, 0)
	wantPixel(t, img, 7, 10, whiteByte, whiteByte, whiteByte)
	wantPixel(t, img, 17, 10, whiteByte, whiteByte, whiteByte)
}

// TestPaintClipRestore proves q saves the clip and Q restores it, and that an
// inner q/Q pair does not drop an outer clip.
func TestPaintClipRestore(t *testing.T) {
	t.Run("restore clears", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		src := "q 0 0 5 20 re W n Q 1 0 0 rg 0 0 20 20 re f"
		if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 15, 10, 255, 0, 0)
	})
	t.Run("nested", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		src := "0 0 5 20 re W n q 0 0 5 10 re W n Q 1 0 0 rg 0 0 20 20 re f"
		if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 2, 5, 255, 0, 0)
		wantPixel(t, img, 15, 10, whiteByte, whiteByte, whiteByte)
	})
}

// TestPaintClipRejected proves a marker without graphics.ClipMarker refuses W
// with undefined, which keeps the rewrite recorder clip-free.
func TestPaintClipRejected(t *testing.T) {
	rec := &refusingMarker{}
	err := Paint(t.Context(), []byte("0 0 5 5 re W n"), rec, 1)
	wantJobErr(t, err, "W", nameUndefined)
	err = Paint(t.Context(), []byte("0 0 5 5 re W* n"), rec, 1)
	wantJobErr(t, err, "W*", nameUndefined)
}

// refusingMarker implements graphics.Marker only.
type refusingMarker struct {
	strokes int
}

func (rec *refusingMarker) Stroke(_ []graphics.Point, _, _, _, _ float64) {
	rec.strokes++
}

func (rec *refusingMarker) Fill(_ []graphics.Point, _, _, _ float64, _ bool) {}

func (rec *refusingMarker) DrawImage(_ image.Image, _ graphics.Matrix, _ float64) {}
