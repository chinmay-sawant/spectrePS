package spectreps_test

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestImagePDFStable(t *testing.T) {
	in := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	pages, err := in.RunPostScript(t.Context(), []byte("0 0 moveto 10 0 lineto stroke"), opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d", len(pages))
	}
	first := imagePDFBytes(t, in, pages, 72)
	second := imagePDFBytes(t, in, pages, 72)
	if res := spectreps.CompareFiles(first, second); !res.Equal {
		t.Fatalf("CompareFiles = %+v", res)
	}
	if bytes.Contains(first, []byte("CreationDate")) || bytes.Contains(first, []byte("ModDate")) {
		t.Fatal("image PDF contains a date")
	}
	checkStableImage(t, first, pages[0])
	checkStableMismatch(t, in, pages[0], first)
}

func imagePDFBytes(t *testing.T, in *spectreps.Instance, pages []spectreps.PageImage, dpi float64) []byte {
	t.Helper()
	got, err := in.ImagePDF(t.Context(), pages, dpi)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func checkStableImage(t *testing.T, payload []byte, page spectreps.PageImage) {
	t.Helper()
	got := inflatePageImage(t, payload)
	want := tightPageImage(page)
	if !bytes.Equal(got, want) {
		t.Fatalf("image stream = %d bytes, want %d", len(got), len(want))
	}
}

func checkStableMismatch(t *testing.T, in *spectreps.Instance, page spectreps.PageImage, stable []byte) {
	t.Helper()
	changed := clonePageImage(page)
	changed.Pixels[0] ^= 0xFF
	out := imagePDFBytes(t, in, []spectreps.PageImage{changed}, 72)
	if res := spectreps.CompareFiles(stable, out); res.Equal {
		t.Fatal("one changed pixel did not change the bytes")
	}
}

func clonePageImage(page spectreps.PageImage) spectreps.PageImage {
	pixels := make([]byte, len(page.Pixels))
	copy(pixels, page.Pixels)
	page.Pixels = pixels
	return page
}

func tightPageImage(page spectreps.PageImage) []byte {
	out := make([]byte, 0, page.Width*page.Height*3)
	for row := range page.Height {
		base := row * page.Stride
		out = append(out, page.Pixels[base:base+page.Width*3]...)
	}
	return out
}

func inflatePageImage(t *testing.T, payload []byte) []byte {
	t.Helper()
	marker := []byte("/Subtype /Image")
	found := bytes.Index(payload, marker)
	if found < 0 {
		t.Fatal("image dictionary missing")
	}
	rest := payload[found:]
	start := bytes.Index(rest, []byte("stream\n"))
	if start < 0 {
		t.Fatal("image stream start missing")
	}
	body := rest[start+len("stream\n"):]
	end := bytes.Index(body, []byte("\nendstream"))
	if end < 0 {
		t.Fatal("image stream end missing")
	}
	reader, err := zlib.NewReader(bytes.NewReader(body[:end]))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return plain
}
