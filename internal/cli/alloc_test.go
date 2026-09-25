package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// allocCLIRuns is the number of measured rewrite runs. AllocsPerRun is
// deterministic under GOMAXPROCS=1, so the count gates make test.
const allocCLIRuns = 20

// allocCLILevel2 is the accepted allocation ceiling for one level 2 rewrite of
// the small image fixture below. The isolated perf branch measured 701; the
// merged tree adds one allocation in the image decode path, so the budget is
// recorded against the integrated tree.
const allocCLILevel2 = 702

// TestPerformanceAllocs locks the level 2 copy allocation count. Timing never
// gates make test; this deterministic count does.
func TestPerformanceAllocs(t *testing.T) {
	input, output := allocCLIFixture(t)
	args := []string{"rewrite", "-level", "2", "-o", output, input}
	// Pause the collector so the zlib writer pool stays populated across the
	// measured runs; the allocator count is then exact.
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	got := testing.AllocsPerRun(allocCLIRuns, func() {
		if code := Run(args, io.Discard, io.Discard); code != exitOK {
			t.Fatalf("Run(%v) = %d, want 0", args, code)
		}
	})
	if got != allocCLILevel2 {
		t.Errorf("level 2 rewrite allocs = %v, want %v", got, allocCLILevel2)
	}
}

// allocCLIFixture writes a small one-page image PDF and returns its path and
// an output path.
func allocCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	pages, err := in.RunPostScript(
		t.Context(),
		[]byte("0 0 moveto 20 0 lineto stroke"),
		spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := in.ImagePDF(t.Context(), pages, 72)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(input, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return input, filepath.Join(dir, "out.pdf")
}
