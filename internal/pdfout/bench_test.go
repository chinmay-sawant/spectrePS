package pdfout

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

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
