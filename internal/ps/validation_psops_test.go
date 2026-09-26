package ps

import (
	"math"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestValidationPSOps proves the PostScript Level 2 operators the CUPS corpus
// needs: bind, the rectangle paints, arc and arcn, stringwidth, setlinecap,
// the clip set, dtransform, the degree trigonometry, and the interpreter
// identification operators.
func TestValidationPSOps(t *testing.T) {
	t.Run("bind", validationBind)
	t.Run("rectstroke", validationRectStroke)
	t.Run("rectfill", validationRectFill)
	t.Run("arc", validationArc)
	t.Run("arcn", validationArcN)
	t.Run("stringwidth", validationStringWidth)
	t.Run("setlinecap", validationSetLineCap)
	t.Run("clip", validationClip)
	t.Run("clippath pathbbox initclip", validationClipPath)
	t.Run("dtransform", validationDTransform)
	t.Run("cos sin", validationTrig)
	t.Run("string and cvs destination", validationStringCvs)
	t.Run("interpreter info", validationInterpreterInfo)
}

func validationBind(t *testing.T) {
	t.Helper()
	assertIntValues(t, assertRun(t, "{ 1 2 add } bind exec"), 3)
	assertIntValues(t, assertRun(t, "{} bind exec"))
	// bind returns the procedure unchanged: a name still sees the definition
	// current at execution time, which is the documented reason bind is a
	// pass-through in this interpreter.
	assertIntValues(t, assertRun(t, "/x 1 def { x 2 mul } bind /x 2 def exec"), 4)
	got := assertRun(t, "{ 1 } bind")
	if len(got) != 1 || got[0].Kind != KindArray || !got[0].Exec {
		t.Fatalf("{ 1 } bind stack = %+v, want one executable array", got)
	}
	psopsWantError(t, "bind", "stackunderflow")
	psopsWantError(t, "1 bind", "typecheck")
	psopsWantError(t, "[1 2] bind", "typecheck")
	psopsWantError(t, "(a) bind", "typecheck")
	psopsWantError(t, "<<>> bind", "typecheck")
}

func validationRectStroke(t *testing.T) {
	t.Helper()
	_, pages := psopsRun(t, "0 0 0 setrgbcolor 5 5 10 10 rectstroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 10, 10)
	validationWantWhite(t, pages[0], 2, 2)

	_, pages = psopsRun(t, "0 0 0 setrgbcolor 15 15 -10 -10 rectstroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 10, 0, 0, 0)

	_, pages = psopsRun(t, "1 0 0 setrgbcolor 5 5 10 10 rectstroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 10, 255, 0, 0)

	_, pages = psopsRun(t, "0 0 0 setrgbcolor 3 setlinewidth 5 5 10 10 rectstroke", 20, 20)
	validationWantPixel(t, pages[0], 4, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 2, 10)

	// rectstroke does not alter the current path.
	_, pages = psopsRun(t,
		"0 0 0 setrgbcolor 0 0 moveto 10 0 lineto 5 5 10 10 rectstroke stroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 0, 0, 0, 0)
	assertFloats(t, "7 7 moveto 5 5 10 10 rectstroke currentpoint", 7, 7)

	psopsWantError(t, "rectstroke", "stackunderflow")
	psopsWantError(t, "1 2 3 rectstroke", "stackunderflow")
	psopsWantError(t, "1 2 3 (x) rectstroke", "typecheck")
}

func validationRectFill(t *testing.T) {
	t.Helper()
	_, pages := psopsRun(t, "0 0 0 setrgbcolor 5 5 10 10 rectfill", 20, 20)
	validationWantPixel(t, pages[0], 10, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 2, 2)

	_, pages = psopsRun(t, "1 0 0 setrgbcolor 15 15 -10 -10 rectfill", 20, 20)
	validationWantPixel(t, pages[0], 10, 10, 255, 0, 0)

	// rectfill does not alter the current path.
	_, pages = psopsRun(t,
		"0 0 0 setrgbcolor 0 0 moveto 10 0 lineto 5 5 10 10 rectfill stroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 0, 0, 0, 0)

	psopsWantError(t, "rectfill", "stackunderflow")
	psopsWantError(t, "1 2 3 rectfill", "stackunderflow")
	psopsWantError(t, "1 2 3 (x) rectfill", "typecheck")
}

func validationArc(t *testing.T) {
	t.Helper()
	_, pages := psopsRun(t, "0 0 0 setrgbcolor 10 10 5 0 360 arc closepath fill", 20, 20)
	validationWantPixel(t, pages[0], 10, 10, 0, 0, 0)
	validationWantPixel(t, pages[0], 10, 14, 0, 0, 0)
	validationWantWhite(t, pages[0], 1, 1)
	validationWantWhite(t, pages[0], 10, 16)

	// With a current point, arc connects it to the arc start with a line.
	_, pages = psopsRun(t, "0 0 0 setrgbcolor 0 0 moveto 10 10 5 0 90 arc stroke", 20, 20)
	validationWantPixel(t, pages[0], 3, 2, 0, 0, 0)
	validationWantPixel(t, pages[0], 10, 14, 0, 0, 0)

	// Without a current point, arc starts its own subpath, so no connector.
	_, pages = psopsRun(t, "0 0 0 setrgbcolor 10 10 5 0 90 arc stroke", 20, 20)
	validationWantWhite(t, pages[0], 3, 2)
	validationWantPixel(t, pages[0], 10, 14, 0, 0, 0)

	// A full turn from equal angles is a point at the start angle.
	got := assertRun(t, "10 10 5 30 30 arc currentpoint")
	psopsWantPoint(t, got, 10+5*math.Cos(30*math.Pi/halfTurnDegrees),
		10+5*math.Sin(30*math.Pi/halfTurnDegrees))

	// An angle below the start walks counterclockwise past zero.
	got = assertRun(t, "10 10 5 340 20 arc currentpoint")
	psopsWantPoint(t, got, 10+5*math.Cos(20*math.Pi/halfTurnDegrees),
		10+5*math.Sin(20*math.Pi/halfTurnDegrees))

	psopsWantError(t, "arc", "stackunderflow")
	psopsWantError(t, "10 10 5 0 arc", "stackunderflow")
	psopsWantError(t, "10 10 (r) 0 90 arc", "typecheck")
	psopsWantError(t, "10 10 -1 0 90 arc", "rangecheck")
}

func validationArcN(t *testing.T) {
	t.Helper()
	// arcn from 0 degrees clockwise to 90 takes the long way through the
	// bottom of the circle.
	_, pages := psopsRun(t, "0 0 0 setrgbcolor 10 10 5 0 90 arcn stroke", 20, 20)
	validationWantPixel(t, pages[0], 13, 6, 0, 0, 0)
	validationWantWhite(t, pages[0], 13, 13)

	got := assertRun(t, "10 10 5 90 0 arcn currentpoint")
	psopsWantPoint(t, got, 15, 10)

	psopsWantError(t, "arcn", "stackunderflow")
	psopsWantError(t, "10 10 -2 0 90 arcn", "rangecheck")
	psopsWantError(t, "10 10 5 0 (x) arcn", "typecheck")
}

func validationStringWidth(t *testing.T) {
	t.Helper()
	assertFloats(t, "/Helvetica findfont 1000 scalefont setfont (A) stringwidth", 667, 0)
	assertFloats(t, "/Helvetica findfont 1000 scalefont setfont ( ) stringwidth", 278, 0)
	assertFloats(t, "/Helvetica findfont 1000 scalefont setfont (AB) stringwidth", 1334, 0)
	assertFloats(t, "/Times-Roman findfont 1000 scalefont setfont (A) stringwidth", 722, 0)
	assertFloats(t, "/Courier findfont 1000 scalefont setfont (A) stringwidth", 600, 0)
	// No current point is needed, and the current point is not moved.
	assertFloats(t,
		"/Helvetica findfont 1000 scalefont setfont 7 7 moveto (A) stringwidth pop "+
			"pop currentpoint",
		7, 7)
	// A code with no StandardEncoding name advances nothing.
	assertFloats(t, "/Helvetica findfont 1000 scalefont setfont (\000) stringwidth", 0, 0)

	psopsWantError(t, "stringwidth", "stackunderflow")
	psopsWantError(t, "1 stringwidth", "typecheck")
	psopsWantError(t, "(A) stringwidth", "invalidfont")
}

func validationSetLineCap(t *testing.T) {
	t.Helper()
	interp, _ := psopsRun(t, "1 setlinecap", 10, 10)
	if gsFor(interp).lineCap != 1 {
		t.Fatalf("lineCap = %d, want 1", gsFor(interp).lineCap)
	}
	interp, _ = psopsRun(t, "0 setlinecap 2 setlinecap", 10, 10)
	if gsFor(interp).lineCap != 2 {
		t.Fatalf("lineCap = %d, want 2", gsFor(interp).lineCap)
	}
	// gsave and grestore save the line cap.
	interp, _ = psopsRun(t, "1 setlinecap gsave 0 setlinecap grestore", 10, 10)
	if gsFor(interp).lineCap != 1 {
		t.Fatalf("lineCap after grestore = %d, want 1", gsFor(interp).lineCap)
	}

	psopsWantError(t, "setlinecap", "stackunderflow")
	psopsWantError(t, "1.0 setlinecap", "typecheck")
	psopsWantError(t, "-1 setlinecap", "rangecheck")
	psopsWantError(t, "3 setlinecap", "rangecheck")
}

func validationClip(t *testing.T) {
	t.Helper()
	clip := "5 5 moveto 15 5 lineto 15 15 lineto 5 15 lineto closepath"
	// clip on a fill: the fill crosses the page, the clip cuts it.
	_, pages := psopsRun(t,
		"0 0 0 setrgbcolor "+clip+" clip 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill",
		20, 20)
	validationWantPixel(t, pages[0], 10, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 2, 2)

	// clip leaves the current path in place, so the path can be stroked after.
	_, pages = psopsRun(t, "0 0 0 setrgbcolor "+clip+" clip stroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 10, 0, 0, 0)

	// An empty path clips everything away.
	_, pages = psopsRun(t, "0 0 0 setrgbcolor newpath clip 0 0 20 20 rectfill", 20, 20)
	validationWantWhite(t, pages[0], 10, 10)

	// gsave and grestore carry the clip.
	_, pages = psopsRun(t,
		"gsave 0 0 0 setrgbcolor "+clip+" clip 1 0 0 setrgbcolor 0 0 20 20 rectfill grestore "+
			"0 0 1 setrgbcolor 0 0 20 20 rectfill", 20, 20)
	validationWantPixel(t, pages[0], 10, 10, 0, 0, 255)
	validationWantPixel(t, pages[0], 2, 2, 0, 0, 255)

	// initclip drops the stored clip.
	_, pages = psopsRun(t,
		"0 0 0 setrgbcolor "+clip+" clip initclip 0 0 20 20 rectfill", 20, 20)
	validationWantPixel(t, pages[0], 2, 2, 0, 0, 0)

	// clip needs no operand and never errors on an empty stack.
	_, pages = psopsRun(t, "clip 0 0 20 20 rectfill", 20, 20)
	validationWantWhite(t, pages[0], 10, 10)
}

func validationClipPath(t *testing.T) {
	t.Helper()
	// With no pixmap the device page is the letter default.
	assertFloats(t, "initclip newpath clippath pathbbox", 0, 0, 612, 792)

	// pathbbox reports the current path in user space, through the inverse CTM.
	assertFloats(t, "newpath pathbbox", 0, 0, 0, 0)
	assertFloats(t, "2 3 moveto 10 3 lineto 10 7 lineto pathbbox", 2, 3, 10, 7)
	assertFloats(t, "2 0 0 2 1 1 concat 0 0 moveto 10 0 lineto pathbbox", 0, 0, 10, 0)

	// The default clip path is the device page rectangle.
	interp, _ := psopsRun(t, "initclip newpath clippath pathbbox", 20, 20)
	got := interp.Operand()
	psopsWantPoint(t, got[0:2], 0, 0)
	psopsWantPoint(t, got[2:4], 20, 20)

	// A stored clip is still the clip after newpath, so clippath returns it.
	interp, _ = psopsRun(t,
		"5 5 moveto 15 5 lineto 15 15 lineto 5 15 lineto closepath clip newpath clippath pathbbox",
		20, 20)
	got = interp.Operand()
	psopsWantPoint(t, got[0:2], 5, 5)
	psopsWantPoint(t, got[2:4], 15, 15)
}

func validationDTransform(t *testing.T) {
	t.Helper()
	assertFloats(t, "72 72 dtransform", 72, 72)
	assertFloats(t, "2 3 scale 1 1 dtransform", 2, 3)
	assertFloats(t, "1 2 translate 1 0 dtransform", 1, 0)
	// rotate 90 turns the x distance into the y distance; the cosine is a
	// rounding-sized 6.1e-17, so compare with a tolerance.
	psopsWantFloatsNear(t, "90 rotate 72 0 dtransform", 0, 72)
	psopsWantError(t, "dtransform", "stackunderflow")
	psopsWantError(t, "1 (y) dtransform", "typecheck")
}

func psopsWantFloatsNear(t *testing.T, src string, want ...float64) {
	t.Helper()
	got := assertRun(t, src)
	if len(got) != len(want) {
		t.Fatalf("Run(%q) stack len = %d, want %d", src, len(got), len(want))
	}
	const tolerance = 1e-9
	for idx, obj := range got {
		value, ok := scalar(obj)
		if !ok || math.Abs(value-want[idx]) > tolerance {
			t.Fatalf("Run(%q) stack[%d] = %+v, want %v", src, idx, obj, want[idx])
		}
	}
}

func validationTrig(t *testing.T) {
	t.Helper()
	assertFloats(t, "0 cos", 1)
	assertFloats(t, "90 sin", 1)
	assertFloats(t, "180 cos", -1)
	assertFloats(t, "270 sin", -1)
	psopsWantError(t, "sin", "stackunderflow")
	psopsWantError(t, "cos", "stackunderflow")
	psopsWantError(t, "(a) cos", "typecheck")
	psopsWantError(t, "true sin", "typecheck")
}

func validationStringCvs(t *testing.T) {
	t.Helper()
	// string allocates a zero-filled string of the popped length.
	got := assertRun(t, "3 string")
	if len(got) != 1 || got[0].Kind != KindString || string(got[0].Str.Bytes) != "\x00\x00\x00" {
		t.Fatalf("3 string stack = %+v, want three zero bytes", got)
	}
	psopsWantError(t, "string", "stackunderflow")
	psopsWantError(t, "-1 string", "rangecheck")
	psopsWantError(t, "1.0 string", "typecheck")

	// cvs with a destination string writes into it and returns the written part.
	assertFloats(t, "42 10 string cvs length", 2)
	validationWantString(t, "42 10 string cvs", "42")
	assertFloats(t, "0.5 10 string cvs length", 3)
	// The destination is padded with spaces.
	got = assertRun(t, "42 6 string cvs")
	if len(got) != 1 || string(got[0].Str.Bytes) != "42" {
		t.Fatalf("42 6 string cvs stack = %+v, want 42", got)
	}
	// A destination that is too short is rangecheck.
	psopsWantError(t, "12345 2 string cvs", "rangecheck")
	// The allocating form still answers with --nostringval-- for composites.
	validationWantString(t, "(abc) cvs", "--nostringval--")
}

func validationInterpreterInfo(t *testing.T) {
	t.Helper()
	got := assertRun(t, "languagelevel")
	if len(got) != 1 || got[0].Kind != KindInt || got[0].Int != languageLevel {
		t.Fatalf("languagelevel stack = %+v, want int %d", got, languageLevel)
	}
	psopsWantStrings(t, "version", "product")
	psopsWantInts(t, "revision", "serialnumber")
}

// psopsWantStrings proves each operator name pushes one non-empty string.
func psopsWantStrings(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		got := assertRun(t, name)
		if len(got) != 1 || got[0].Kind != KindString || len(got[0].Str.Bytes) == 0 {
			t.Fatalf("%s stack = %+v, want a non-empty string", name, got)
		}
	}
}

// psopsWantInts proves each operator name pushes one int.
func psopsWantInts(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		got := assertRun(t, name)
		if len(got) != 1 || got[0].Kind != KindInt {
			t.Fatalf("%s stack = %+v, want an int", name, got)
		}
	}
}

// psopsRun runs src against a fresh pixmap. It returns the interpreter so a
// test can read the graphics state, and the finished pages.
func psopsRun(t *testing.T, src string, width, height int) (*Interp, []graphics.Image) {
	t.Helper()
	interp := NewInterp()
	pixmap := graphics.NewPixmap(width, height)
	interp.UsePixmap(pixmap, 1)
	if err := interp.Run(t.Context(), []byte(src)); err != nil {
		t.Fatalf("Run(%q) error = %v", src, err)
	}
	return interp, pixmap.Pages()
}

func psopsWantError(t *testing.T, src, name string) {
	t.Helper()
	interp := NewInterp()
	err := interp.Run(t.Context(), []byte(src))
	got, ok := psErrorName(err)
	if !ok || got != name {
		t.Fatalf("Run(%q) error = %v, want %s", src, err, name)
	}
}

// psopsWantPoint compares the two objects at the start of got with one user
// point, allowing the rounding a degree-to-radian conversion introduces.
func psopsWantPoint(t *testing.T, got []Object, wantX, wantY float64) {
	t.Helper()
	if len(got) < 2 {
		t.Fatalf("stack = %+v, want two numbers", got)
	}
	values := make([]float64, 2)
	for i := range values {
		value, ok := scalar(got[i])
		if !ok {
			t.Fatalf("stack[%d] = %+v, want a number", i, got[i])
		}
		values[i] = value
	}
	const tolerance = 1e-9
	if math.Abs(values[0]-wantX) > tolerance || math.Abs(values[1]-wantY) > tolerance {
		t.Fatalf("point = %v,%v, want %v,%v", values[0], values[1], wantX, wantY)
	}
}
