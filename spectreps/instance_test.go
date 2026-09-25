package spectreps_test

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationMultiInstance runs two instances from two New calls in
// interleaved and sequential order. A process-wide singleton would show
// cross-talk between the two instances.
func TestValidationMultiInstance(t *testing.T) {
	first := newInst(t)
	second := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	red := []byte("1 0 0 setrgbcolor 0 0 moveto 10 0 lineto stroke")
	green := []byte("0 1 0 setrgbcolor 0 0 moveto 10 0 lineto stroke")

	firstRed := validationInstancePages(t, first, red, opt)
	secondGreen := validationInstancePages(t, second, green, opt)
	// Interleaved: each instance runs the other instance's program next.
	firstGreen := validationInstancePages(t, first, green, opt)
	secondRed := validationInstancePages(t, second, red, opt)
	// Sequential: each instance repeats its first program.
	firstRedAgain := validationInstancePages(t, first, red, opt)
	secondGreenAgain := validationInstancePages(t, second, green, opt)

	validationSamePage(t, "first red", firstRed[0], firstRedAgain[0])
	validationSamePage(t, "second green", secondGreen[0], secondGreenAgain[0])
	validationSamePage(t, "first green", firstGreen[0], secondGreen[0])
	validationSamePage(t, "second red", secondRed[0], firstRed[0])
	if spectreps.CompareRaster(firstRed[0], firstGreen[0]).Equal {
		t.Fatal("first instance painted red and green the same")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second first.Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second second.Close() error = %v", err)
	}
}

func validationInstancePages(
	t *testing.T,
	in *spectreps.Instance,
	src []byte,
	opt spectreps.RunOptions,
) []spectreps.PageImage {
	t.Helper()
	pages, err := in.RunPostScript(t.Context(), src, opt)
	if err != nil {
		t.Fatalf("RunPostScript(%q) error = %v", src, err)
	}
	if len(pages) != 1 {
		t.Fatalf("RunPostScript(%q) pages = %d, want 1", src, len(pages))
	}
	return pages
}

func validationSamePage(t *testing.T, name string, left, right spectreps.PageImage) {
	t.Helper()
	if res := spectreps.CompareRaster(left, right); !res.Equal {
		t.Fatalf("%s raster = %+v, want equal", name, res)
	}
}
