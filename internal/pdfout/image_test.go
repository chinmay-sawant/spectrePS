package pdfout

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	imageTestDPI    = 144
	paddingByte     = 0xEE
	imageStreamMark = "stream\n"
)

func TestImagePDF(t *testing.T) {
	images, packed := sampleImages(t)
	src := imagePDFBytes(t, images, imageTestDPI)
	if !bytes.Equal(src, imagePDFBytes(t, images, imageTestDPI)) {
		t.Fatal("two calls differ")
	}
	at72 := imagePDFBytes(t, images, defaultImageDPI)
	if !bytes.Equal(at72, imagePDFBytes(t, images, 0)) {
		t.Fatal("dpi 0 did not select 72")
	}
	checkImageHeader(t, src)
	checkImageTrailer(t, src)
	checkImageDict(t, src, len(images))
	checkImageDims(t, src, images)
	checkMediaBoxes(t, src, []string{
		"/MediaBox [0 0 2.5 1.5]",
		"/MediaBox [0 0 2 2]",
		"/MediaBox [0 0 1 1]",
	})
	checkMediaBoxes(t, at72, []string{
		"/MediaBox [0 0 5 3]",
		"/MediaBox [0 0 4 4]",
		"/MediaBox [0 0 2 2]",
	})
	checkImageStreams(t, src, packed)
	checkImageID(t, src, packed)
	checkImageContext(t)
	checkEmptyImages(t)
}

// sampleImages returns three images with stride padding plus the packed rows.
func sampleImages(t *testing.T) ([]graphics.Image, [][]byte) {
	t.Helper()
	sizes := []struct {
		width  int
		height int
		pad    int
	}{
		{width: 5, height: 3, pad: 3},
		{width: 4, height: 4, pad: 0},
		{width: 2, height: 2, pad: 1},
	}
	images := make([]graphics.Image, len(sizes))
	packed := make([][]byte, len(sizes))
	for i, size := range sizes {
		images[i], packed[i] = paddedImage(size.width, size.height, size.pad)
	}
	return images, packed
}

func paddedImage(width, height, pad int) (graphics.Image, []byte) {
	stride := width*imageChannels + pad
	pixels := make([]byte, height*stride)
	packed := make([]byte, 0, width*height*imageChannels)
	for row := range height {
		for col := range width * imageChannels {
			value := byte(row*width*imageChannels + col)
			pixels[row*stride+col] = value
			packed = append(packed, value)
		}
		for i := width * imageChannels; i < stride; i++ {
			pixels[row*stride+i] = paddingByte
		}
	}
	return graphics.Image{Width: width, Height: height, Stride: stride, Pixels: pixels}, packed
}

func imagePDFBytes(t *testing.T, pages []graphics.Image, dpi float64) []byte {
	t.Helper()
	got, err := WriteImages(t.Context(), pages, dpi)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func checkImageHeader(t *testing.T, src []byte) {
	t.Helper()
	if !bytes.HasPrefix(src, []byte(headerLine)) {
		t.Fatal("bad header")
	}
	if !bytes.HasSuffix(src, []byte("%%EOF\n")) {
		t.Fatal("bad trailer")
	}
}

func checkImageTrailer(t *testing.T, src []byte) {
	t.Helper()
	rejectDates(t, src)
	if bytes.Contains(src, []byte("/Info")) {
		t.Fatal("file contains /Info")
	}
}

func checkImageDict(t *testing.T, src []byte, count int) {
	t.Helper()
	keys := []string{
		"/Type /XObject",
		"/Subtype /Image",
		"/ColorSpace /DeviceRGB",
		"/BitsPerComponent 8",
		"/Filter /FlateDecode",
		"/XObject << /Im0 ",
		"cm /Im0 Do Q",
	}
	for _, key := range keys {
		if got := bytes.Count(src, []byte(key)); got != count {
			t.Fatalf("%s count = %d, want %d", key, got, count)
		}
	}
}

func checkImageDims(t *testing.T, src []byte, images []graphics.Image) {
	t.Helper()
	for _, img := range images {
		width := "/Width " + strconv.Itoa(img.Width)
		height := "/Height " + strconv.Itoa(img.Height)
		if !bytes.Contains(src, []byte(width)) || !bytes.Contains(src, []byte(height)) {
			t.Fatalf("missing %s or %s", width, height)
		}
	}
}

func checkMediaBoxes(t *testing.T, src []byte, boxes []string) {
	t.Helper()
	if got := bytes.Count(src, []byte("/MediaBox [")); got != len(boxes) {
		t.Fatalf("MediaBox count = %d, want %d", got, len(boxes))
	}
	for _, box := range boxes {
		if !bytes.Contains(src, []byte(box)) {
			t.Fatalf("missing %s", box)
		}
	}
}

func checkImageStreams(t *testing.T, src []byte, packed [][]byte) {
	t.Helper()
	decoded := decodedImages(t, src)
	if len(decoded) != len(packed) {
		t.Fatalf("image streams = %d, want %d", len(decoded), len(packed))
	}
	for i, want := range packed {
		if !bytes.Equal(decoded[i], want) {
			t.Fatalf("page %d stream = %d bytes, want %d", i, len(decoded[i]), len(want))
		}
	}
}

func decodedImages(t *testing.T, src []byte) [][]byte {
	t.Helper()
	marker := []byte("/Subtype /Image")
	var out [][]byte
	rest := src
	for {
		found := bytes.Index(rest, marker)
		if found < 0 {
			return out
		}
		rest = rest[found:]
		start := bytes.Index(rest, []byte(imageStreamMark))
		if start < 0 {
			t.Fatal("image stream start missing")
		}
		body := rest[start+len(imageStreamMark):]
		end := bytes.Index(body, []byte("\nendstream"))
		if end < 0 {
			t.Fatal("image stream end missing")
		}
		out = append(out, inflateImage(t, body[:end]))
		rest = body[end:]
	}
}

func inflateImage(t *testing.T, src []byte) []byte {
	t.Helper()
	reader, err := zlib.NewReader(bytes.NewReader(src))
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

func checkImageID(t *testing.T, src []byte, packed [][]byte) {
	t.Helper()
	parts := make([][]byte, len(packed))
	for i, page := range packed {
		parts[i] = mustFlate(t, page)
	}
	wantID(t, src, parts...)
}

func checkImageContext(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := WriteImages(ctx, nil, defaultImageDPI)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("canceled %v %#v", err, got)
	}
	defer func() {
		recovered := recover()
		if recovered != panicNilContext {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_, _ = WriteImages(nil, nil, defaultImageDPI) //nolint:staticcheck // nil context is the case under test
}

func checkEmptyImages(t *testing.T) {
	t.Helper()
	got := imagePDFBytes(t, nil, defaultImageDPI)
	if !bytes.Equal(got, imagePDFBytes(t, []graphics.Image{}, 0)) {
		t.Fatal("empty inputs differ")
	}
	file, err := pdf.Open(t.Context(), got)
	if err != nil {
		t.Fatal(err)
	}
	if file.PageCount() != 0 {
		t.Fatalf("pages %d", file.PageCount())
	}
	wantID(t, got)
	rejectDates(t, got)
}
