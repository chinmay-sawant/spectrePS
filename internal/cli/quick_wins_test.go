package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestQuickWins is the v0.0.3 acceptance walk through Run. It covers PDF
// pages in raster, -pages ranges with emitted-page numbering, PDF inputs in
// compare raster, TIFF raster output, and the pdfimage color spaces.
func TestQuickWins(t *testing.T) {
	t.Run("raster every PDF page", quickWinsRasterPDFPages)
	t.Run("pages range numbering", quickWinsPageRanges)
	t.Run("compare raster PDF inputs", quickWinsCompareRasterPDF)
	t.Run("tiff output", quickWinsTIFFOutput)
	t.Run("pdfimage colorspaces", quickWinsPDFImageColorspaces)
}

// quickWinsTwoPagePDF writes a two-page PDF whose first page is red and whose
// second page is the given RGB fill.
func quickWinsTwoPagePDF(t *testing.T, second string) string {
	t.Helper()
	contents := []string{"1 0 0 rg 0 0 20 20 re f", second}
	return writeTemp(t, "two.pdf", pagesPDF(t, contents...))
}

// quickWinsRasterArgs builds a 20 by 20 raster command line.
func quickWinsRasterArgs(pages, out, src string) []string {
	return []string{"raster", "-pages", pages, "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
}

// quickWinsCompareArgs builds a 20 by 20 compare raster command line.
func quickWinsCompareArgs(options []string, left, right string) []string {
	args := []string{"compare", "raster"}
	args = append(args, options...)
	return append(args, left, right)
}

func quickWinsRasterPDFPages(t *testing.T) {
	src := quickWinsTwoPagePDF(t, "0 1 0 rg 0 0 20 20 re f")
	dir := t.TempDir()
	out := filepath.Join(dir, "page-%d.ppm")
	want(t, []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-2.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "page-3.ppm"))
}

func quickWinsPageRanges(t *testing.T) {
	src := quickWinsTwoPagePDF(t, "0 1 0 rg 0 0 20 20 re f")
	dir := t.TempDir()

	// Page 2 is renumbered from 1 in the output path.
	single := filepath.Join(dir, "single-%d.ppm")
	want(t, quickWinsRasterArgs("2", single, src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "single-1.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "single-2.ppm"))

	full := filepath.Join(dir, "full-%d.ppm")
	want(t, quickWinsRasterArgs("1-2", full, src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "full-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "full-2.ppm")), 0, 255, 0)

	bad := filepath.Join(dir, "bad-%d.ppm")
	want(t, quickWinsRasterArgs("3", bad, src), 1, "", "Error: /rangecheck in pages\n")
	pageNoFile(t, filepath.Join(dir, "bad-1.ppm"))
}

func quickWinsCompareRasterPDF(t *testing.T) {
	two := quickWinsTwoPagePDF(t, "0 1 0 rg 0 0 20 20 re f")
	blue := quickWinsTwoPagePDF(t, "0 0 1 rg 0 0 20 20 re f")
	one := writeTemp(t, "one.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	copyPath := filepath.Join(t.TempDir(), "copy.pdf")
	raw, err := os.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	options := []string{"-w", "20", "-h", "20", "-r", "72"}

	want(t, quickWinsCompareArgs(options, two, copyPath), 0, "", "")
	want(t, quickWinsCompareArgs(options, two, one), 1, "mismatch length\n", "")
	want(t, quickWinsCompareArgs(options, two, blue), 1, "mismatch pixel 1\n", "")
	want(t, quickWinsCompareArgs(append([]string{"-pages", "1"}, options...), two, one), 0, "", "")
	rangeErr := quickWinsCompareArgs(append([]string{"-pages", "3"}, options...), two, one)
	want(t, rangeErr, 1, "", "Error: /rangecheck in pages\n")
}

func quickWinsTIFFOutput(t *testing.T) {
	src := writeTemp(t, "solid.ps", []byte(
		"0.25 0.5 0.75 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill"))
	dir := t.TempDir()
	ppm := filepath.Join(dir, "page.ppm")
	want(t, []string{"raster", "-o", ppm, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	body := ppmBody(t, ppm)

	for _, tt := range []struct {
		name   string
		flag   string
		suffix string
	}{
		{"deflate-tif", "deflate", ".tif"},
		{"none-tif", "none", ".tif"},
		{"deflate-tiff", "deflate", ".tiff"},
		{"none-tiff", "none", ".tiff"},
	} {
		out := filepath.Join(dir, tt.name+tt.suffix)
		args := []string{"raster", "-tiffcompress", tt.flag, "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
		want(t, args, 0, "", "")
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		// checkTIFF decodes with tiff.Decode and compares pixels. TIFF bytes
		// are not an equality oracle.
		checkTIFF(t, tt.name, raw, body)
	}
}

func quickWinsPDFImageColorspaces(t *testing.T) {
	src := writeTemp(t, "red.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	dir := t.TempDir()
	for _, tt := range []struct {
		space      string
		deviceName string
		stream     []byte
	}{
		{"rgb", "/DeviceRGB", bytes.Repeat([]byte{255, 0, 0}, 400)},
		{"gray", "/DeviceGray", bytes.Repeat([]byte{76}, 400)},
		{"cmyk", "/DeviceCMYK", bytes.Repeat([]byte{0, 255, 255, 0}, 400)},
	} {
		out := filepath.Join(dir, tt.space+".pdf")
		args := []string{"pdfimage", "-colorspace", tt.space, "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
		want(t, args, 0, "", "")
		checkPDFImageOutput(t, out, 1)
		payload := readPayload(t, out)
		if !bytes.Contains(payload, []byte("/ColorSpace "+tt.deviceName)) {
			t.Fatalf("-colorspace %s wrote no %s", tt.space, tt.deviceName)
		}
		streams := imageStreamBodies(t, payload)
		if len(streams) != 1 {
			t.Fatalf("-colorspace %s wrote %d image streams, want 1", tt.space, len(streams))
		}
		if !bytes.Equal(streams[0], tt.stream) {
			t.Fatalf("-colorspace %s stream = %d bytes, want %d", tt.space, len(streams[0]), len(tt.stream))
		}
	}
}
