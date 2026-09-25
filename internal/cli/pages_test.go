package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pagesPDF builds a classic PDF fixture with one content stream per page.
func pagesPDF(t *testing.T, contents ...string) []byte {
	t.Helper()
	kids := make([]string, 0, len(contents))
	for i := range contents {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+2*i))
	}
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
			strings.Join(kids, " "), len(contents))),
	}
	for i, content := range contents {
		pageDict := fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents %d 0 R /Resources << >> >>",
			4+2*i)
		objects = append(objects, []byte(pageDict), flateStream(t, content))
	}
	return classicXref(t, objects)
}

// pageCheckFill fails unless every RGB triple in a PPM body is one color.
func pageCheckFill(t *testing.T, body []byte, red, green, blue byte) {
	t.Helper()
	if len(body) == 0 || len(body)%3 != 0 {
		t.Fatalf("ppm body len %d", len(body))
	}
	for i := 0; i+2 < len(body); i += 3 {
		if body[i] != red || body[i+1] != green || body[i+2] != blue {
			t.Fatalf("pixel %d = %d,%d,%d, want %d,%d,%d",
				i/3, body[i], body[i+1], body[i+2], red, green, blue)
		}
	}
}

func pageNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s exists, want no file", path)
	}
}

const (
	pagePSRed   = "1 0 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill"
	pagePSGreen = "0 1 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill"
	pagePSBlue  = "0 0 1 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill"
)

const pagePSThreePages = pagePSRed + " showpage " + pagePSGreen + " showpage " + pagePSBlue + " showpage"

func TestRasterPDFPages(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "two.pdf")
	contents := []string{
		"1 0 0 rg 0 0 20 20 re f",
		"0 1 0 rg 0 0 20 20 re f",
	}
	if err := os.WriteFile(src, pagesPDF(t, contents...), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "page-%d.ppm")
	want(t, []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-2.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "page-3.ppm"))

	plain := filepath.Join(dir, "plain.ppm")
	wantCode(t, []string{"raster", "-o", plain, "-w", "20", "-h", "20", "-r", "72", src}, 2)
	pageNoFile(t, plain)
}

func TestPageSelection(t *testing.T) {
	t.Run("postscript", pageSelectionPS)
	t.Run("pdf", pageSelectionPDF)
	t.Run("range errors", pageSelectionRangeErrors)
	t.Run("postscript failure", pageSelectionPSFailure)
	t.Run("malformed", pageSelectionUsage)
}

func pageSelectionPS(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "three.ps")
	if err := os.WriteFile(src, []byte(pagePSThreePages), 0o600); err != nil {
		t.Fatal(err)
	}

	// run accepts -pages and ignores it.
	want(t, []string{"run", "-pages", "2", src}, 0, "", "")

	out := filepath.Join(dir, "range-%d.ppm")
	want(t, []string{"raster", "-pages", "2-3", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "range-1.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "range-2.ppm")), 0, 0, 255)
	pageNoFile(t, filepath.Join(dir, "range-3.ppm"))

	single := filepath.Join(dir, "single-%d.ppm")
	want(t, []string{"raster", "-pages", "2", "-o", single, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "single-1.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "single-2.ppm"))

	open := filepath.Join(dir, "open-%d.ppm")
	want(t, []string{"raster", "-pages", "2-", "-o", open, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "open-1.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "open-2.ppm")), 0, 0, 255)

	start := filepath.Join(dir, "start-%d.ppm")
	want(t, []string{"raster", "-pages", "-2", "-o", start, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "start-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "start-2.ppm")), 0, 255, 0)

	want(t, []string{"inkcov", "-pages", "2", "-w", "20", "-h", "20", "-r", "72", src}, 0,
		"Page 1\n1.00000 0.00000 1.00000 RGB\n", "")
}

func pageSelectionPDF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "two.pdf")
	contents := []string{
		"1 0 0 rg 0 0 20 20 re f",
		"0 1 0 rg 0 0 20 20 re f",
	}
	if err := os.WriteFile(src, pagesPDF(t, contents...), 0o600); err != nil {
		t.Fatal(err)
	}

	single := filepath.Join(dir, "single-%d.ppm")
	want(t, []string{"raster", "-pages", "2", "-o", single, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "single-1.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "single-2.ppm"))

	// An end past the last page clamps to the last page.
	clamped := filepath.Join(dir, "clamp-%d.ppm")
	want(t, []string{"raster", "-pages", "1-9", "-o", clamped, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "clamp-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "clamp-2.ppm")), 0, 255, 0)

	want(t, []string{"bbox", "-pages", "2", "-w", "20", "-h", "20", "-r", "72", src}, 0,
		"%%BoundingBox: 0 0 20 20\n%%HiResBoundingBox: 0 0 20 20\n", "")
	want(t, []string{"inkcov", "-pages", "2", "-w", "20", "-h", "20", "-r", "72", src}, 0,
		"Page 1\n1.00000 0.00000 1.00000 RGB\n", "")

	img := filepath.Join(dir, "one.pdf")
	want(t, []string{"pdfimage", "-pages", "2", "-o", img, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	checkPDFImageOutput(t, img, 1)

	want(t, []string{"compare", "raster", "-pages", "1", "-w", "20", "-h", "20", "-r", "72", src, src}, 0, "", "")
}

const pageRangeErrorText = "Error: /rangecheck in pages\n"

// pageRasterArgs builds a raster command line for the 20 by 20 fixtures.
func pageRasterArgs(src, pages, out string) []string {
	return []string{
		"raster",
		"-pages", pages,
		"-o", out,
		"-w", "20",
		"-h", "20",
		"-r", "72",
		src,
	}
}

func pageSelectionRangeErrors(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "two.ps")
	if err := os.WriteFile(psPath, []byte(pagePSRed+" showpage "+pagePSGreen+" showpage"), 0o600); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(dir, "two.pdf")
	contents := []string{
		"1 0 0 rg 0 0 20 20 re f",
		"0 1 0 rg 0 0 20 20 re f",
	}
	if err := os.WriteFile(pdfPath, pagesPDF(t, contents...), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bad-%d.ppm")
	for _, src := range []string{psPath, pdfPath} {
		want(t, pageRasterArgs(src, "3", out), 1, "", pageRangeErrorText)
		want(t, pageRasterArgs(src, "0-2", out), 1, "", pageRangeErrorText)
		want(t, pageRasterArgs(src, "2-1", out), 1, "", pageRangeErrorText)
		inkcov := []string{"inkcov", "-pages", "3", "-w", "20", "-h", "20", "-r", "72", src}
		want(t, inkcov, 1, "", pageRangeErrorText)
	}
	pageNoFile(t, filepath.Join(dir, "bad-1.ppm"))
}

func pageSelectionPSFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.ps")
	if err := os.WriteFile(src, []byte(pagePSRed+" showpage save"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bad-%d.ppm")
	// Page 1 paints, but the range filter runs after RunPostScript, so the
	// failing page 2 still fails the command.
	want(t, pageRasterArgs(src, "1", out), 1, "", "Error: /undefined in save\n")
	pageNoFile(t, filepath.Join(dir, "bad-1.ppm"))
}

func pageSelectionUsage(t *testing.T) {
	src := writeTemp(t, "in.ps", []byte("1 2 add"))
	out := filepath.Join(t.TempDir(), "out.ppm")
	for _, value := range []string{"x", "1-2-3", "1.5"} {
		wantCode(t, []string{"raster", "-pages", value, "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 2)
	}
}

func TestCompareRasterPDF(t *testing.T) {
	dir := t.TempDir()
	two := filepath.Join(dir, "two.pdf")
	contents := []string{
		"1 0 0 rg 0 0 20 20 re f",
		"0 1 0 rg 0 0 20 20 re f",
	}
	if err := os.WriteFile(two, pagesPDF(t, contents...), 0o600); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(dir, "copy.pdf")
	twoBytes, err := os.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, twoBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	one := filepath.Join(dir, "one.pdf")
	if err := os.WriteFile(one, onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.pdf")
	otherContents := []string{
		"1 0 0 rg 0 0 20 20 re f",
		"0 0 1 rg 0 0 20 20 re f",
	}
	if err := os.WriteFile(other, pagesPDF(t, otherContents...), 0o600); err != nil {
		t.Fatal(err)
	}
	options := []string{"-w", "20", "-h", "20", "-r", "72"}

	want(t, pageCompareArgs(options, two, copyPath), 0, "", "")
	want(t, pageCompareArgs(options, two, one), 1, "mismatch length\n", "")
	want(t, pageCompareArgs(append([]string{"-pages", "1"}, options...), two, one), 0, "", "")
	want(t, pageCompareArgs(append([]string{"-pages", "1"}, options...), two, other), 0, "", "")

	code, stdout, stderr := callRun(t, pageCompareArgs(options, two, other)...)
	if code != 1 || stdout != "mismatch pixel 1\n" || stderr != "" {
		t.Fatalf("page 2 differs: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	ps := writeTemp(t, "page.ps", []byte(pagePSRed))
	want(t, pageCompareArgs(append([]string{"-pages", "1"}, options...), two, ps), 0, "", "")

	want(t, pageCompareArgs(append([]string{"-pages", "3"}, options...), one, one), 1, "", "Error: /rangecheck in pages\n")
}

func pageCompareArgs(options []string, left, right string) []string {
	args := []string{"compare", "raster"}
	args = append(args, options...)
	return append(args, left, right)
}
