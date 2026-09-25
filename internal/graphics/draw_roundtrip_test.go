package graphics_test

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

// TestPixmapDrawImageRoundTrip writes one ImagePDF page, decodes its image
// stream, and stamps it back into a pixmap at 1:1. The pixels match the
// source image byte for byte.
func TestPixmapDrawImageRoundTrip(t *testing.T) {
	src := graphics.Image{Width: 5, Height: 3, Stride: 15, Pixels: roundTripPixels(5, 3)}
	got := roundTripPage(t, src)
	if got.Width != src.Width || got.Height != src.Height || got.Stride != src.Stride {
		t.Fatalf("geometry %dx%d stride %d, want %dx%d stride %d",
			got.Width, got.Height, got.Stride, src.Width, src.Height, src.Stride)
	}
	if !bytes.Equal(got.Pixels, src.Pixels) {
		t.Fatal("round trip pixels differ")
	}
}

// roundTripPage wraps src in an ImagePDF, decodes the stream, and stamps the
// decoded pixels back at 1:1.
func roundTripPage(t *testing.T, src graphics.Image) graphics.Image {
	t.Helper()
	payload, err := pdfout.WriteImages(t.Context(), []graphics.Image{src}, 72)
	if err != nil {
		t.Fatal(err)
	}
	file, err := pdf.Open(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	nums, err := file.ImageObjectNums()
	if err != nil {
		t.Fatal(err)
	}
	if len(nums) != 1 {
		t.Fatalf("image objects = %v, want one", nums)
	}
	pic, err := file.DecodeImage(nums[0])
	if err != nil {
		t.Fatal(err)
	}
	pixmap := graphics.NewPixmap(src.Width, src.Height)
	matrix := graphics.Matrix{
		A: float64(src.Width), B: 0, C: 0,
		D: float64(src.Height), E: 0, F: 0,
	}
	pixmap.DrawImage(pic, matrix, 1)
	pixmap.ShowPage()
	pages := pixmap.Pages()
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	return pages[0]
}

// roundTripPixels is one deterministic RGB page, row 0 first.
func roundTripPixels(width, height int) []byte {
	pixels := make([]byte, 0, width*height*3)
	for row := range height {
		for col := range width {
			pixels = append(pixels, byte(row*width*3+col), byte(col*7), byte(row*13))
		}
	}
	return pixels
}
