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

// allocRuns is the number of measured runs per function. AllocsPerRun is
// deterministic under GOMAXPROCS=1, so the counts gate make test.
const allocRuns = 100

// The allocation ceiling per function. These are accepted baseline numbers,
// recorded in documentation/performance.md. The first five are the v0.0.4 set
// and the rest are the v0.0.5 coverage extension.
const (
	allocMeasureBox       = 0
	allocMeasureInk       = 0
	allocMeasureInkAmount = 0
	allocCompareRaster    = 0
	allocCompareFiles     = 0

	allocDocumentInfo    = 7
	allocPreflightUA2    = 4512
	allocWritePostScript = 5628
	allocRewriteTagged   = 12090
	allocRewritePDFA     = 147
)

// allocCase is one locked allocation count.
type allocCase struct {
	name  string
	limit float64
	call  func()
}

// TestPerformanceAllocs locks the allocation counts of the measure and
// compare jobs. Timing never gates make test; these deterministic counts do.
func TestPerformanceAllocs(t *testing.T) {
	in := newInst(t)
	pages, err := in.RunPostScript(t.Context(), []byte(benchStrokeProgram), benchRunOptions())
	if err != nil {
		t.Fatal(err)
	}
	img := pages[0]
	other := img
	other.Pixels = bytes.Clone(img.Pixels)
	left := bytes.Repeat([]byte("spectreps"), 512)
	right := bytes.Clone(left)

	cases := []allocCase{
		{name: "MeasureBox", limit: allocMeasureBox, call: func() {
			_, _ = spectreps.MeasureBox(img, 72)
		}},
		{name: "MeasureInk", limit: allocMeasureInk, call: func() {
			_ = spectreps.MeasureInk(img)
		}},
		{name: "MeasureInkAmount", limit: allocMeasureInkAmount, call: func() {
			_ = spectreps.MeasureInkAmount(img)
		}},
		{name: "CompareRaster", limit: allocCompareRaster, call: func() {
			_ = spectreps.CompareRaster(img, other)
		}},
		{name: "CompareFiles", limit: allocCompareFiles, call: func() {
			_ = spectreps.CompareFiles(left, right)
		}},
	}
	for _, tc := range cases {
		got := testing.AllocsPerRun(allocRuns, tc.call)
		if got != tc.limit {
			t.Errorf("%s allocs = %v, want %v", tc.name, got, tc.limit)
		}
	}
}

// TestJobAllocs locks the allocation counts of the jobs the v0.0.5 coverage
// extension gave a benchmark. AllocsPerRun and -benchmem do not always agree,
// because AllocsPerRun warms the zlib writer pool before it measures and a
// benchmark does not, so the ceilings here are the AllocsPerRun values. The
// benchmark values are in documentation/performance.md beside them.
func TestJobAllocs(t *testing.T) {
	ua2Src := allocRead(t, "pdfua2/compliant-ua2.pdf")
	pathSrc := allocRead(t, "compress/path.pdf")

	ua2In := newInst(t)
	ua2Doc, err := ua2In.OpenPDF(t.Context(), ua2Src)
	if err != nil {
		t.Fatal(err)
	}

	pathIn := newInst(t)
	pathDoc, err := pathIn.OpenPDF(t.Context(), pathSrc)
	if err != nil {
		t.Fatal(err)
	}

	tagged := spectreps.RewriteOptions{Tag: true, Title: "Alloc", Lang: "en-US"}
	archival := spectreps.RewriteOptions{PDFA: spectreps.PDFA4}

	cases := []allocCase{
		{name: "DocumentInfo", limit: allocDocumentInfo, call: func() {
			_, _ = pathDoc.Info()
		}},
		{name: "PreflightUA2", limit: allocPreflightUA2, call: func() {
			_ = ua2In.PreflightUA2(t.Context(), ua2Doc)
		}},
		{name: "WritePostScript", limit: allocWritePostScript, call: func() {
			_, _ = pathIn.WritePostScript(t.Context(), pathDoc, spectreps.PostScriptOptions{})
		}},
		{name: "RewritePDF tagged", limit: allocRewriteTagged, call: func() {
			_, _ = pathIn.RewritePDF(t.Context(), pathDoc, tagged)
		}},
		{name: "RewritePDF PDF/A-4", limit: allocRewritePDFA, call: func() {
			_, _ = pathIn.RewritePDF(t.Context(), pathDoc, archival)
		}},
	}
	for _, tc := range cases {
		got := testing.AllocsPerRun(allocRuns, tc.call)
		if got != tc.limit {
			t.Errorf("%s allocs = %v, want %v", tc.name, got, tc.limit)
		}
	}
}

// allocRead reads one file under sampledata and skips the test when it is
// absent, the rule the benchmarks use.
func allocRead(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "sampledata", filepath.FromSlash(name))
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("sampledata/%s is absent", name)
	}
	if err != nil {
		t.Fatal(err)
	}
	return src
}
