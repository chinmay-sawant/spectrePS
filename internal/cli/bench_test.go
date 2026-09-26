package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// benchCLIStrokeProgram is the synthetic PostScript input the raster and
// pdfimage benchmarks run. It paints 2000 horizontal strokes on a 200 by 200
// point page.
const benchCLIStrokeProgram = "0 1 1999 { /y exch def 0 y moveto 200 y lineto 1 setlinewidth stroke } for"

// benchCLISample returns a checked-in input path, or skips when sampledata is
// absent.
func benchCLISample(b *testing.B, name string) string {
	b.Helper()
	path := filepath.Join("..", "..", "sampledata", filepath.FromSlash(name))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.Skipf("sampledata/%s is absent", name)
	}
	return path
}

// benchCLIProgram writes the stroke program under the benchmark temp dir.
func benchCLIProgram(b *testing.B) string {
	b.Helper()
	path := filepath.Join(b.TempDir(), "stroke.ps")
	if err := os.WriteFile(path, []byte(benchCLIStrokeProgram), 0o600); err != nil {
		b.Fatal(err)
	}
	return path
}

// benchCLIRun calls Run with the args and fails on a nonzero exit code.
func benchCLIRun(b *testing.B, args ...string) {
	b.Helper()
	if code := Run(args, io.Discard, io.Discard); code != exitOK {
		b.Fatalf("Run(%v) = %d, want 0", args, code)
	}
}

// BenchmarkCLIVersion measures one version command in process.
func BenchmarkCLIVersion(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchCLIRun(b, "version")
	}
}

// BenchmarkCLIRaster rasterizes the path PDF at 72 and 300 dpi.
func BenchmarkCLIRaster(b *testing.B) {
	src := benchCLISample(b, "compress/path.pdf")
	cases := []struct {
		name string
		dpi  string
	}{
		{name: "72", dpi: "72"},
		{name: "300", dpi: "300"},
	}
	for _, testCase := range cases {
		b.Run(testCase.name, func(b *testing.B) {
			out := filepath.Join(b.TempDir(), "out.ppm")
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchCLIRun(b, "raster", "-r", testCase.dpi, "-o", out, src)
			}
		})
	}
}

// BenchmarkCLIRewrite rewrites the image PDF at levels 2 and 5.
func BenchmarkCLIRewrite(b *testing.B) {
	src := benchCLISample(b, "compress/whatisthis.pdf")
	cases := []struct {
		name  string
		level string
	}{
		{name: "level2", level: "2"},
		{name: "level5", level: "5"},
	}
	for _, testCase := range cases {
		b.Run(testCase.name, func(b *testing.B) {
			out := filepath.Join(b.TempDir(), "out.pdf")
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchCLIRun(b, "rewrite", "-level", testCase.level, "-o", out, src)
			}
		})
	}
}

// BenchmarkCLIPDFImage packs the stroke program as an image PDF.
func BenchmarkCLIPDFImage(b *testing.B) {
	src := benchCLIProgram(b)
	out := filepath.Join(b.TempDir(), "out.pdf")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchCLIRun(b, "pdfimage", "-w", "200", "-h", "200", "-o", out, src)
	}
}

// BenchmarkCLIText extracts the two-line text PDF.
func BenchmarkCLIText(b *testing.B) {
	src := benchCLISample(b, "fixtures/text.pdf")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchCLIRun(b, "text", src)
	}
}

// BenchmarkCLIGS routes one gs argv line to the pdfwrite job.
func BenchmarkCLIGS(b *testing.B) {
	src := benchCLISample(b, "fixtures/gs-argv-input.pdf")
	out := filepath.Join(b.TempDir(), "out.pdf")
	args := []string{
		"gs", "-q", "-dBATCH", "-dNOPAUSE",
		"-sDEVICE=pdfwrite", "-sOutputFile=" + out, src,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchCLIRun(b, args...)
	}
}

// benchFormatPage paints one 200 by 200 page with the same stroke program the
// raster benchmark uses. Every encoder benchmark shares it, so the difference
// between the four lines is the encoder and not the raster. The CLI raster
// benchmark writes PPM, so without these the PNG, JPEG, and TIFF paths have
// never been timed.
func benchFormatPage(b *testing.B) spectreps.PageImage {
	b.Helper()
	in, err := spectreps.New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = in.Close() })
	pages, err := in.RunPostScript(
		b.Context(),
		[]byte(benchCLIStrokeProgram),
		spectreps.RunOptions{PageWidthPt: 200, PageHeightPt: 200, ResolutionDPI: 72},
	)
	if err != nil {
		b.Fatal(err)
	}
	return pages[0]
}

// BenchmarkEncodePPM measures the PPM writer. It allocates the body and then a
// second full copy when it prepends the header, so its B/op is at least twice
// the page size.
func BenchmarkEncodePPM(b *testing.B) {
	img := benchFormatPage(b)
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = encodePPM(img)
	}
}

// BenchmarkEncodePNG and BenchmarkEncodeJPEG measure the two compressed raster
// formats. Each pays the W*H*4 RGBA conversion in rgbaFromPage first.
func BenchmarkEncodePNG(b *testing.B) {
	img := benchFormatPage(b)
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := encodePNG(img); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeJPEG measures the JPEG writer at the default quality the
// raster command uses.
func BenchmarkEncodeJPEG(b *testing.B) {
	img := benchFormatPage(b)
	b.SetBytes(int64(img.Width * img.Height * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := encodeJPEG(img, 85); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeTIFF measures the TIFF writer with and without Deflate, so
// the compression cost is separated from the container cost.
func BenchmarkEncodeTIFF(b *testing.B) {
	img := benchFormatPage(b)
	for _, testCase := range []struct {
		name     string
		encoding tiffEncoding
	}{
		{name: "deflate", encoding: tiffDeflate},
		{name: "none", encoding: tiffNone},
	} {
		b.Run(testCase.name, func(b *testing.B) {
			b.SetBytes(int64(img.Width * img.Height * 3))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := encodeTIFF(img, testCase.encoding); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
