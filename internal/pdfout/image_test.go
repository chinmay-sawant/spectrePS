package pdfout

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"math"
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

func imagePDFBytesColor(t *testing.T, pages []graphics.Image, dpi float64, space ImageColorSpace) []byte {
	t.Helper()
	got, err := WriteImagesColor(t.Context(), pages, dpi, space)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestImageColorSpaces(t *testing.T) {
	images, packed := sampleImages(t)
	rgb := imagePDFBytes(t, images, imageTestDPI)
	if !bytes.Equal(rgb, imagePDFBytesColor(t, images, imageTestDPI, ImageRGB)) {
		t.Fatal("ImageRGB bytes differ from WriteImages")
	}
	checkImageDictSpace(t, rgb, "/DeviceRGB", len(images))
	gray := imagePDFBytesColor(t, images, imageTestDPI, ImageGray)
	checkImageDictSpace(t, gray, "/DeviceGray", len(images))
	checkColorStreams(t, gray, packed, ImageGray)
	cmyk := imagePDFBytesColor(t, images, imageTestDPI, ImageCMYK)
	checkImageDictSpace(t, cmyk, "/DeviceCMYK", len(images))
	checkColorStreams(t, cmyk, packed, ImageCMYK)
	checkRedConversion(t)
	checkImageColorContext(t)
}

func checkImageDictSpace(t *testing.T, src []byte, name string, count int) {
	t.Helper()
	if got := bytes.Count(src, []byte("/ColorSpace "+name)); got != count {
		t.Fatalf("%s count = %d, want %d", name, got, count)
	}
	if got := bytes.Count(src, []byte("/BitsPerComponent 8")); got != count {
		t.Fatalf("BitsPerComponent count = %d, want %d", got, count)
	}
}

func checkColorStreams(t *testing.T, src []byte, packed [][]byte, space ImageColorSpace) {
	t.Helper()
	decoded := decodedImages(t, src)
	if len(decoded) != len(packed) {
		t.Fatalf("image streams = %d, want %d", len(decoded), len(packed))
	}
	for i, page := range packed {
		want := convertedSamples(page, space)
		if !bytes.Equal(decoded[i], want) {
			t.Fatalf("page %d stream = %d bytes, want %d", i, len(decoded[i]), len(want))
		}
	}
}

// convertedSamples recomputes the frozen policy from the packed RGB bytes.
func convertedSamples(packed []byte, space ImageColorSpace) []byte {
	switch space {
	case ImageGray:
		out := make([]byte, 0, len(packed)/imageChannels)
		for at := 0; at < len(packed); at += imageChannels {
			out = append(out, wantLuma(packed[at], packed[at+1], packed[at+2]))
		}
		return out
	case ImageCMYK:
		out := make([]byte, 0, len(packed)/imageChannels*cmykChannels)
		for at := 0; at < len(packed); at += imageChannels {
			c, m, y, k := wantCMYK(packed[at], packed[at+1], packed[at+2])
			out = append(out, c, m, y, k)
		}
		return out
	case ImageRGB:
		return packed
	default:
		return packed
	}
}

func wantLuma(red, green, blue byte) byte {
	return byte(math.Round(0.299*float64(red) + 0.587*float64(green) + 0.114*float64(blue)))
}

func wantCMYK(red, green, blue byte) (byte, byte, byte, byte) {
	redF := float64(red) / 255
	greenF := float64(green) / 255
	blueF := float64(blue) / 255
	black := 1 - math.Max(redF, math.Max(greenF, blueF))
	if black >= 1 {
		return 0, 0, 0, byte(math.Round(black * 255))
	}
	scale := func(value float64) byte { return byte(math.Round(value * 255)) }
	return scale((1 - redF - black) / (1 - black)),
		scale((1 - greenF - black) / (1 - black)),
		scale((1 - blueF - black) / (1 - black)),
		scale(black)
}

// checkRedConversion is the worked example in documentation/devices.md.
func checkRedConversion(t *testing.T) {
	t.Helper()
	red := []graphics.Image{{Width: 1, Height: 1, Stride: imageChannels, Pixels: []byte{255, 0, 0}}}
	gray := decodedImages(t, imagePDFBytesColor(t, red, defaultImageDPI, ImageGray))
	if len(gray) != 1 || !bytes.Equal(gray[0], []byte{76}) {
		t.Fatalf("red gray = %v, want [76]", gray)
	}
	cmyk := decodedImages(t, imagePDFBytesColor(t, red, defaultImageDPI, ImageCMYK))
	if len(cmyk) != 1 || !bytes.Equal(cmyk[0], []byte{0, 255, 255, 0}) {
		t.Fatalf("red cmyk = %v, want [0 255 255 0]", cmyk)
	}
}

func checkImageColorContext(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := WriteImagesColor(ctx, nil, defaultImageDPI, ImageGray)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("canceled %v %#v", err, got)
	}
	defer func() {
		recovered := recover()
		if recovered != panicNilContext {
			t.Fatalf("panic %v", recovered)
		}
	}()
	//nolint:staticcheck // nil context is the case under test
	_, _ = WriteImagesColor(nil, nil, defaultImageDPI, ImageCMYK)
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
