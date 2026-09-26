package pdfout

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	benchWhatPDF = "compress/whatisthis.pdf"
	benchPathPDF = "compress/path.pdf"
)

// benchSample reads a checked-in input and skips when sampledata is absent.
func benchSample(b *testing.B, name string) []byte {
	b.Helper()
	path := filepath.Join("..", "..", "sampledata", filepath.FromSlash(name))
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		b.Skipf("sampledata/%s is absent", name)
	}
	if err != nil {
		b.Fatal(err)
	}
	return src
}

// benchSampleFile opens a checked-in input as a pdf.File.
func benchSampleFile(b *testing.B, name string) *pdf.File {
	b.Helper()
	src := benchSample(b, name)
	file, err := pdf.Open(b.Context(), src)
	if err != nil {
		b.Fatal(err)
	}
	return file
}

// benchImage builds a gradient RGBA image for the scale and encode kernels.
func benchImage(width, height int) *image.RGBA {
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			pic.SetRGBA(x, y, color.RGBA{
				R: byte(x),
				G: byte(y),
				B: byte(x ^ y),
				A: 255,
			})
		}
	}
	return pic
}

// BenchmarkWriteCopy writes the pass-through copy of a path-only file and an
// image file.
func BenchmarkWriteCopy(b *testing.B) {
	cases := []struct {
		name string
		rel  string
	}{
		{name: "path", rel: benchPathPDF},
		{name: "image", rel: benchWhatPDF},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			file := benchSampleFile(b, tc.rel)
			src := benchSample(b, tc.rel)
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := WriteCopy(b.Context(), file, CopyOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLevel5ImageReEncode decodes and re-encodes the image streams as
// DCT at the hard cap.
func BenchmarkLevel5ImageReEncode(b *testing.B) {
	src := benchSample(b, benchWhatPDF)
	file, err := pdf.Open(b.Context(), src)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := LevelOverrides(b.Context(), file, MaxCompressionLevel); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScaleImage resamples a 1600 by 1200 image to the level 5 cap with
// CatmullRom.
func BenchmarkScaleImage(b *testing.B) {
	pic := benchImage(1600, 1200)
	const width, height = 842, 632
	b.SetBytes(int64(width * height))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = ScaleImage(pic, width, height)
	}
}

// BenchmarkEncodeDCT encodes an 842 by 632 image as baseline JPEG at quality
// 40, the level 5 settings.
func BenchmarkEncodeDCT(b *testing.B) {
	pic := benchImage(842, 632)
	b.SetBytes(int64(842 * 632 * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := EncodeDCT(pic, 40); err != nil {
			b.Fatal(err)
		}
	}
}

// benchGraphicsImage paints one page through the device and returns the shown
// image, which is the shape the image writers take. The packer benchmarks need
// a real graphics.Image and the page costs nothing once it is painted.
func benchGraphicsImage(b *testing.B, side int) graphics.Image {
	b.Helper()
	pixmap := graphics.NewPixmap(side, side)
	for i := range side {
		pixmap.Stroke([]graphics.Point{
			{X: 0, Y: float64(i), Move: true},
			{X: float64(side), Y: float64(i)},
		}, 1, 0, 0, 0)
	}
	pixmap.ShowPage()
	return pixmap.Pages()[0]
}

// BenchmarkWriteImagesColor packs one page through each branch of packSamples.
// The image PDF benchmark in spectreps covers ImageColorRGB only, so the gray
// and CMYK per pixel float loops have never been timed.
func BenchmarkWriteImagesColor(b *testing.B) {
	pages := []graphics.Image{benchGraphicsImage(b, 200)}
	for _, testCase := range []struct {
		name  string
		space ImageColorSpace
	}{
		{name: "rgb", space: ImageRGB},
		{name: "gray", space: ImageGray},
		{name: "cmyk", space: ImageCMYK},
	} {
		b.Run(testCase.name, func(b *testing.B) {
			b.SetBytes(int64(200 * 200 * 3))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := WriteImagesColor(b.Context(), pages, 72, testCase.space); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEncodeFlateRGB encodes an 842 by 632 image as a Flate stream, the
// same geometry BenchmarkEncodeDCT uses so the two encoders compare. The
// encoder reads through the image.Image interface per pixel and writes one
// zlib row per scanline.
func BenchmarkEncodeFlateRGB(b *testing.B) {
	pic := benchImage(842, 632)
	b.SetBytes(int64(842 * 632 * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := EncodeFlateRGB(pic); err != nil {
			b.Fatal(err)
		}
	}
}
