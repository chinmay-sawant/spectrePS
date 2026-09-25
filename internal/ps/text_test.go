package ps

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestShowPS proves that findfont, scalefont, setfont, and show drive the
// standard 14 metrics from the PostScript front end.
func TestShowPS(t *testing.T) {
	t.Parallel()
	checkFontLookup(t)
	checkShowAdvance(t)
	checkShowDevice(t)
}

func checkFontLookup(t *testing.T) {
	t.Helper()
	got := mustRun(t, "/Helvetica findfont /FontName get").Operand()
	if len(got) != 1 || got[0].Kind != KindName || got[0].Name != "Helvetica" {
		t.Fatalf("findfont = %v", got)
	}
	assertFloats(t, "/Courier findfont 12 scalefont /FontSize get", 12)
	assertErrName(t, "/Nope findfont", errInvalidFont)
	assertErrName(t, "12 12 scalefont", errInvalidFont)
	assertErrName(t, "<< /A 1 >> 12 scalefont", errInvalidFont)
}

func checkShowAdvance(t *testing.T) {
	t.Helper()
	assertFloats(t, "0 0 moveto /Courier findfont 10 scalefont setfont (A) show currentpoint", 6, 0)
	assertFloats(t,
		"10 0 translate 0 0 moveto /Courier findfont 10 scalefont setfont (A) show currentpoint",
		6, 0)
	assertFloats(t,
		"2 2 scale 0 0 moveto /Courier findfont 10 scalefont setfont (A) show currentpoint",
		6, 0)
	// gsave and grestore save the current font.
	assertFloats(t,
		"/Courier findfont 10 scalefont setfont gsave /Helvetica findfont 12 scalefont setfont grestore "+
			"0 0 moveto (A) show currentpoint",
		6, 0)
	// A show with no font and no current point is invalidfont and nocurrentpoint.
	assertErrName(t, "0 0 moveto (A) show", errInvalidFont)
	assertErrName(t, "(A) show", errInvalidFont)
	assertErrName(t, "/Courier findfont 10 scalefont setfont (A) show", errNoCurrentPt)
}

func checkShowDevice(t *testing.T) {
	t.Helper()
	interp := NewInterp()
	registerFlowOps(interp)
	interp.UsePixmap(graphics.NewPixmap(20, 20), 1)
	err := interp.Run(t.Context(), []byte("/Helvetica findfont 12 scalefont setfont 0 0 moveto (A) show"))
	got, ok := psErrorName(err)
	if !ok || got != errInvalidFont {
		t.Fatalf("show on a device error = %v, want invalidfont", err)
	}
}
