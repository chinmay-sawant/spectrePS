package spectreps_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// The checked-in inputs. No benchmark reaches the network, and a missing
// sampledata file skips the benchmark instead of failing the package.
const (
	benchPathPDF = "compress/path.pdf"
	benchWhatPDF = "compress/whatisthis.pdf"
	benchTextPDF = "fixtures/text.pdf"

	benchPagePoints = 200
)

// benchStrokeProgram is the synthetic PostScript input: 2000 horizontal
// strokes across a 200 by 200 point page at 0.5 width.
const benchStrokeProgram = "0 1 1999 { /y exch def 0 y moveto 200 y lineto 1 setlinewidth stroke } for"

// benchRunOptions returns the page geometry the raster jobs share. The
// benchmarks paint at 72 dpi, the point-to-pixel scale of 1.
func benchRunOptions() spectreps.RunOptions {
	return spectreps.RunOptions{
		PageWidthPt:   benchPagePoints,
		PageHeightPt:  benchPagePoints,
		ResolutionDPI: 72,
	}
}

// benchRead reads one file under sampledata and skips when the tree is absent.
func benchRead(b *testing.B, name string) []byte {
	b.Helper()
	path := filepath.Join("..", "sampledata", filepath.FromSlash(name))
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		b.Skipf("sampledata/%s is absent", name)
	}
	if err != nil {
		b.Fatal(err)
	}
	return src
}

// benchInstance returns one instance for a benchmark that reuses it.
func benchInstance(b *testing.B) *spectreps.Instance {
	b.Helper()
	in, err := spectreps.New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = in.Close() })
	return in
}

// benchOpen returns an instance and the open document for one input.
func benchOpen(b *testing.B, src []byte) (*spectreps.Instance, *spectreps.Document) {
	b.Helper()
	in := benchInstance(b)
	doc, err := in.OpenPDF(b.Context(), src)
	if err != nil {
		b.Fatal(err)
	}
	return in, doc
}

// benchPaint runs the stroke program and returns the page pixels.
func benchPaint(b *testing.B) []spectreps.PageImage {
	b.Helper()
	pages, err := benchInstance(b).RunPostScript(
		b.Context(),
		[]byte(benchStrokeProgram),
		benchRunOptions(),
	)
	if err != nil {
		b.Fatal(err)
	}
	return pages
}

// benchmarkRunPostScript runs the synthetic program for one page.
func BenchmarkRunPostScript(b *testing.B) {
	in := benchInstance(b)
	src := []byte(benchStrokeProgram)
	opt := benchRunOptions()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := in.RunPostScript(b.Context(), src, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOpenPDF parses a classic xref file and an xref stream file.
func BenchmarkOpenPDF(b *testing.B) {
	cases := []struct {
		name string
		rel  string
	}{
		{name: "classic", rel: benchPathPDF},
		{name: "xref-stream", rel: benchWhatPDF},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			src := benchRead(b, tc.rel)
			in := benchInstance(b)
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := in.OpenPDF(b.Context(), src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRasterizePage paints page 0 of the path PDF at 72 dpi.
func BenchmarkRasterizePage(b *testing.B) {
	src := benchRead(b, benchPathPDF)
	in, doc := benchOpen(b, src)
	opt := benchRunOptions()
	width, height := benchPixels(opt)
	b.SetBytes(int64(width * height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := in.RasterizePage(b.Context(), doc, 0, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// benchPixels returns the page size in pixels without painting.
func benchPixels(opt spectreps.RunOptions) (int, int) {
	scale := float64(opt.ResolutionDPI) / 72
	width := int(float64(opt.PageWidthPt)*scale + 0.5)
	height := int(float64(opt.PageHeightPt)*scale + 0.5)
	return width, height
}

// benchRewrite rewrites one open document at the requested options.
func benchRewrite(b *testing.B, rel string, opt spectreps.RewriteOptions) {
	b.Helper()
	src := benchRead(b, rel)
	in, doc := benchOpen(b, src)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := in.RewritePDF(b.Context(), doc, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRewriteLevel0 re-emits the path subset of the path PDF.
func BenchmarkRewriteLevel0(b *testing.B) {
	benchRewrite(b, benchPathPDF, spectreps.DefaultRewriteOptions())
}

// BenchmarkRewriteLevel1 flates every content stream of the image PDF.
func BenchmarkRewriteLevel1(b *testing.B) {
	benchRewrite(b, benchWhatPDF, spectreps.RewriteOptions{Level: 1})
}

// BenchmarkRewriteLevel2 also re-encodes Flate, raw, and CCITT images.
func BenchmarkRewriteLevel2(b *testing.B) {
	benchRewrite(b, benchWhatPDF, spectreps.RewriteOptions{Level: 2})
}

// BenchmarkRewriteLevel3 re-encodes images as DCT at the medium cap.
func BenchmarkRewriteLevel3(b *testing.B) {
	benchRewrite(b, benchWhatPDF, spectreps.RewriteOptions{Level: 3})
}

// BenchmarkRewriteLevel4 re-encodes images as DCT at the strong cap.
func BenchmarkRewriteLevel4(b *testing.B) {
	benchRewrite(b, benchWhatPDF, spectreps.RewriteOptions{Level: 4})
}

// BenchmarkRewriteLevel5 re-encodes images as DCT at the hard cap.
func BenchmarkRewriteLevel5(b *testing.B) {
	benchRewrite(b, benchWhatPDF, spectreps.RewriteOptions{Level: 5})
}

// BenchmarkExtractText extracts the two-line text PDF.
func BenchmarkExtractText(b *testing.B) {
	src := benchRead(b, benchTextPDF)
	in, doc := benchOpen(b, src)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := in.ExtractText(b.Context(), doc, 0); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkImagePDF packs one stroke page as an image PDF in RGB.
func BenchmarkImagePDF(b *testing.B) {
	pages := benchPaint(b)
	in := benchInstance(b)
	ctx := b.Context()
	b.SetBytes(int64(pages[0].Width * pages[0].Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := in.ImagePDF(ctx, pages, 72); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMeasureBox measures the marked edges of one stroke page.
func BenchmarkMeasureBox(b *testing.B) {
	img := benchPaint(b)[0]
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok := spectreps.MeasureBox(img, 72); !ok {
			b.Fatal("blank page")
		}
	}
}

// BenchmarkMeasureInk measures the marked channel fractions of one page.
func BenchmarkMeasureInk(b *testing.B) {
	img := benchPaint(b)[0]
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = spectreps.MeasureInk(img)
	}
}

// BenchmarkMeasureInkAmount measures the weighted ink of one page.
func BenchmarkMeasureInkAmount(b *testing.B) {
	img := benchPaint(b)[0]
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = spectreps.MeasureInkAmount(img)
	}
}

// BenchmarkCompareRaster compares two equal stroke pages.
func BenchmarkCompareRaster(b *testing.B) {
	left := benchPaint(b)[0]
	right := left
	right.Pixels = bytes.Clone(left.Pixels)
	b.SetBytes(int64(left.Width * left.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if res := spectreps.CompareRaster(left, right); !res.Equal {
			b.Fatalf("compare = %+v", res)
		}
	}
}

// BenchmarkCompareFiles compares two equal copies of the image PDF bytes.
func BenchmarkCompareFiles(b *testing.B) {
	src := benchRead(b, benchWhatPDF)
	left := src
	right := bytes.Clone(src)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if res := spectreps.CompareFiles(left, right); !res.Equal {
			b.Fatalf("compare = %+v", res)
		}
	}
}

// BenchmarkReuseInstance rasterizes every page with one instance and one open
// document, the shape an embedded consumer keeps.
func BenchmarkReuseInstance(b *testing.B) {
	src := benchRead(b, benchPathPDF)
	in, doc := benchOpen(b, src)
	opt := benchRunOptions()
	pages := doc.PageCount()
	if pages == 0 {
		b.Fatal("no pages")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if _, err := in.RasterizePage(b.Context(), doc, i%pages, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPerCallInstance pays a New, an OpenPDF, one page, and a Close per
// iteration, the shape a consumer pays per call.
func BenchmarkPerCallInstance(b *testing.B) {
	src := benchRead(b, benchPathPDF)
	opt := benchRunOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		in, err := spectreps.New()
		if err != nil {
			b.Fatal(err)
		}
		doc, err := in.OpenPDF(b.Context(), src)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := in.RasterizePage(b.Context(), doc, 0, opt); err != nil {
			b.Fatal(err)
		}
		if err := in.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
