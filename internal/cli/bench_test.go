package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
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
