package pdfa

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The package had no benchmark before this file. Both preflights run inside
// every PDF/A rewrite and every tagged write, and PreflightUA2 runs a second
// content scanner over the same bytes at ua2_content.go:27, so both are on a
// product path.

// benchSample reads a checked-in input and skips when sampledata is absent,
// matching the skip rule in spectreps/bench_test.go.
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

// benchOpenSample opens a checked-in input as a pdf.File.
func benchOpenSample(b *testing.B, name string) *pdf.File {
	b.Helper()
	file, err := pdf.Open(b.Context(), benchSample(b, name))
	if err != nil {
		b.Fatal(err)
	}
	return file
}

// BenchmarkPreflight runs the PDF/A object loop over the compliant sample.
// The loop visits every object in the file, so the cost is the object count
// and not the page count.
func BenchmarkPreflight(b *testing.B) {
	file := benchOpenSample(b, "pdfa/compliant-a4.pdf")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := Preflight(b.Context(), file, Mode4); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPreflightUA2 runs the PDF/UA-2 preflight over the compliant sample.
// This is the second content scanner, separate from the one the content
// interpreter runs.
func BenchmarkPreflightUA2(b *testing.B) {
	file := benchOpenSample(b, "pdfua2/compliant-ua2.pdf")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := PreflightUA2(b.Context(), file); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkXMPPacket builds the two metadata packets a rewrite appends.
func BenchmarkXMPPacket(b *testing.B) {
	b.Run("pdfa4", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if len(XMP(Mode4)) == 0 {
				b.Fatal("empty XMP packet")
			}
		}
	})
	b.Run("ua2", func(b *testing.B) {
		meta := UA2Write(UA2Info{Title: "Bench", Lang: "en-US"}, false, true)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if len(UA2XMP(meta)) == 0 {
				b.Fatal("empty UA2 XMP packet")
			}
		}
	})
}
