package spectreps_test

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// allocRuns is the number of measured runs per function. AllocsPerRun is
// deterministic under GOMAXPROCS=1, so the counts gate make test.
const allocRuns = 100

// The allocation ceiling per function. These are accepted baseline numbers,
// recorded in documentation/performance.md.
const (
	allocMeasureBox       = 0
	allocMeasureInk       = 0
	allocMeasureInkAmount = 0
	allocCompareRaster    = 0
	allocCompareFiles     = 0
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
