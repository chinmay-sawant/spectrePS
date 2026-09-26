package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationGSEdges locks the pdfwrite input pairing: a PostScript input
// reaches the PDF open and fails there with exit 1, and a PDF input succeeds.
func TestValidationGSEdges(t *testing.T) {
	t.Run("postscript input", validationGSEdgesPostScript)
	t.Run("pdf input", validationGSEdgesPDF)
}

// validationGSEdgesPostScript proves the failure is the open error and not a
// usage error. The stderr line is compared with the library's own open error
// for the same bytes, so the gs mode is checked to route the input to the
// reader rather than only to print something.
func validationGSEdgesPostScript(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	out := filepath.Join(t.TempDir(), "out.pdf")
	args := []string{"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + out, src}
	code, stdout, stderr := callRun(t, args...)
	if code != exitJob {
		t.Fatalf("code = %d, want %d, stderr=%q", code, exitJob, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	_, openErr := in.OpenPDF(t.Context(), readPayload(t, src))
	if openErr == nil {
		t.Fatal("the PostScript input opened as a PDF")
	}
	if stderr != openErr.Error()+"\n" {
		t.Fatalf("stderr = %q, want the open error %q", stderr, openErr.Error()+"\n")
	}
	pageNoFile(t, out)
}

func validationGSEdgesPDF(t *testing.T) {
	t.Helper()
	src := gsPDF(t)
	out := filepath.Join(t.TempDir(), "out.pdf")
	args := []string{
		"gs", "-q", "-dBATCH", "-dNOPAUSE",
		"-sDEVICE=pdfwrite", "-sOutputFile=" + out, src,
	}
	want(t, args, exitOK, "", "")
	checkPDFImageOutput(t, out, 2)
}

// TestValidationGSOutputRules locks the percent rules on the one-file devices
// and the single-page pattern job.
func TestValidationGSOutputRules(t *testing.T) {
	t.Run("pdfimage24 percent", validationGSPercentPDFImage)
	t.Run("pdfwrite percent", validationGSPercentPDFWrite)
	t.Run("single page percent", validationGSSinglePagePattern)
}

func validationGSPercentPDFImage(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "page-%d.pdf")
	args := []string{"gs", "-sDEVICE=pdfimage24", "-sOutputFile=" + out, src}
	message := fmt.Sprintf("spectreps: -sOutputFile wants a plain path on -sDEVICE=pdfimage24, got %q\n", out)
	want(t, args, exitUsage, "", message)
	pageNoFile(t, filepath.Join(dir, "page-1.pdf"))
	pageNoFile(t, out)
}

func validationGSPercentPDFWrite(t *testing.T) {
	t.Helper()
	src := gsPDF(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "page-%d.pdf")
	args := []string{"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + out, src}
	message := fmt.Sprintf("spectreps: -sOutputFile wants a plain path on -sDEVICE=pdfwrite, got %q\n", out)
	want(t, args, exitUsage, "", message)
	pageNoFile(t, filepath.Join(dir, "page-1.pdf"))
	pageNoFile(t, out)
}

func validationGSSinglePagePattern(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "one.ps", []byte(pagePSRed))
	dir := t.TempDir()
	out := filepath.Join(dir, "page-%d.ppm")
	want(t, gsRasterArgs(out, src), exitOK, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-1.ppm")), 255, 0, 0)
	pageNoFile(t, filepath.Join(dir, "page-2.ppm"))
	pageNoFile(t, out)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d files, want 1", len(entries))
	}
}

// TestValidationGSParamCorners locks the empty and malformed value forms of
// the allowlisted switches, and the -g resolution rule on both devices.
func TestValidationGSParamCorners(t *testing.T) {
	t.Run("page list", validationGSPageListCorners)
	t.Run("first page empty", validationGSFirstPageEmpty)
	t.Run("last page", validationGSLastPageCorners)
	t.Run("device height", validationGSHeightCorners)
	t.Run("jpeg quality", validationGSJPEGQCorners)
	t.Run("grid size", validationGSGridSizeCorners)
}

func validationGSPageListCorners(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	out := filepath.Join(t.TempDir(), "list-%d.ppm")
	want(t, gsRasterArgs(out, src, "-sPageList"), exitUsage, "",
		"spectreps: -sPageList needs a list\n")
	want(t, gsRasterArgs(out, src, "-sPageList="), exitUsage, "",
		"spectreps: -sPageList needs a list\n")
	pageNoFile(t, filepath.Join(filepath.Dir(out), "list-1.ppm"))
}

func validationGSFirstPageEmpty(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	out := filepath.Join(t.TempDir(), "first-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dFirstPage="), exitUsage, "",
		"spectreps: -dFirstPage needs =N\n")
	pageNoFile(t, filepath.Join(filepath.Dir(out), "first-1.ppm"))
}

func validationGSLastPageCorners(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	out := filepath.Join(t.TempDir(), "last-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dLastPage=0"), exitUsage, "",
		"spectreps: -dLastPage wants a page number, got \"0\"\n")
	want(t, gsRasterArgs(out, src, "-dLastPage=x"), exitUsage, "",
		"spectreps: -dLastPage wants a page number, got \"x\"\n")
	pageNoFile(t, filepath.Join(filepath.Dir(out), "last-1.ppm"))
}

func validationGSHeightCorners(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "height.ppm")
	heightArgs := func(tokens ...string) []string {
		args := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-dDEVICEWIDTHPOINTS=20"}
		args = append(args, tokens...)
		return append(args, src)
	}
	want(t, heightArgs("-dDEVICEHEIGHTPOINTS"), exitUsage, "",
		"spectreps: -dDEVICEHEIGHTPOINTS needs =N\n")
	want(t, heightArgs("-dDEVICEHEIGHTPOINTS=x"), exitUsage, "",
		"spectreps: -dDEVICEHEIGHTPOINTS wants a positive size, got \"x\"\n")
	want(t, heightArgs("-dDEVICEHEIGHTPOINTS=0"), exitUsage, "",
		"spectreps: -dDEVICEHEIGHTPOINTS wants a positive size, got \"0\"\n")
	pageNoFile(t, out)

	// A missing value and a missing switch differ: the bare token exits 2,
	// and the omitted switch keeps the 792-point default.
	widthOnly := filepath.Join(dir, "width-only.ppm")
	args := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + widthOnly, "-dDEVICEWIDTHPOINTS=20", src}
	want(t, args, exitOK, "", "")
	gsCheckPPMHeader(t, readPayload(t, widthOnly), 20, 792)
}

func validationGSJPEGQCorners(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	dir := t.TempDir()
	args := func(path string, extra ...string) []string {
		all := []string{"gs", "-sDEVICE=jpeg", "-sOutputFile=" + path}
		all = append(all, extra...)
		return append(all, "-g20x20", src)
	}
	want(t, args(filepath.Join(dir, "missing.jpg"), "-dJPEGQ"), exitUsage, "",
		"spectreps: -dJPEGQ needs =N\n")
	want(t, args(filepath.Join(dir, "empty.jpg"), "-dJPEGQ="), exitUsage, "",
		"spectreps: -dJPEGQ needs =N\n")
	pageNoFile(t, filepath.Join(dir, "missing.jpg"))
	pageNoFile(t, filepath.Join(dir, "empty.jpg"))

	// The omitted switch keeps the default quality of 75.
	byDefault := filepath.Join(dir, "default.jpg")
	q75 := filepath.Join(dir, "q75.jpg")
	want(t, args(byDefault), exitOK, "", "")
	want(t, args(q75, "-dJPEGQ=75"), exitOK, "", "")
	if !bytes.Equal(readPayload(t, byDefault), readPayload(t, q75)) {
		t.Fatal("the default JPEG quality is not 75")
	}

	// The quality reaches the encoder, so the clamped pairs cannot match by
	// accident of a dropped -jpegq flag.
	minQuality := filepath.Join(dir, "q1.jpg")
	maxQuality := filepath.Join(dir, "q100.jpg")
	want(t, args(minQuality, "-dJPEGQ=1"), exitOK, "", "")
	want(t, args(maxQuality, "-dJPEGQ=100"), exitOK, "", "")
	if bytes.Equal(readPayload(t, minQuality), readPayload(t, maxQuality)) {
		t.Fatal("-dJPEGQ did not change the JPEG bytes")
	}

	for _, testCase := range []struct {
		name string
		arg  string
		ref  string
	}{
		{"zero clamps to 1", "-dJPEGQ=0", minQuality},
		{"negative clamps to 1", "-dJPEGQ=-7", minQuality},
		{"over clamps to 100", "-dJPEGQ=101", maxQuality},
		{"far over clamps to 100", "-dJPEGQ=1000", maxQuality},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			out := filepath.Join(dir, "clamp.jpg")
			want(t, args(out, testCase.arg), exitOK, "", "")
			if !bytes.Equal(readPayload(t, out), readPayload(t, testCase.ref)) {
				t.Fatalf("%s did not clamp to the reference bytes", testCase.arg)
			}
		})
	}
}

func validationGSGridSizeCorners(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "grid.ppm")
	gridArgs := func(grid string) []string {
		return []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, grid, src}
	}
	want(t, gridArgs("-g0x0"), exitUsage, "",
		"spectreps: -g wants WxH, got \"0x0\"\n")
	want(t, gridArgs("-g-10x20"), exitUsage, "",
		"spectreps: -g wants WxH, got \"-10x20\"\n")
	want(t, gridArgs("-g20x-20"), exitUsage, "",
		"spectreps: -g wants WxH, got \"20x-20\"\n")
	pageNoFile(t, out)

	args := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-g144x72", "-r72", src}
	want(t, args, exitOK, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 144, 72)

	pdfOut := filepath.Join(dir, "grid.pdf")
	args = []string{"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + pdfOut, "-g20x20", gsPDF(t)}
	want(t, args, exitUsage, "", "spectreps: -g has no place on -sDEVICE=pdfwrite\n")
	pageNoFile(t, pdfOut)
}

// TestValidationGSRejectedDevices adds the grayscale, alpha, color-space, and
// EPS names to the reject set the grammar names.
func TestValidationGSRejectedDevices(t *testing.T) {
	src := gsSolidPS(t)
	for _, name := range []string{"jpeggray", "pgmraw", "pngalpha", "pamcmyk32", "eps2write"} {
		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.dat")
			args := []string{"gs", "-sDEVICE=" + name, "-sOutputFile=" + out, src}
			want(t, args, exitUsage, "", "spectreps: -sDEVICE="+name+" is not an accepted device\n")
			pageNoFile(t, out)
		})
	}
}
