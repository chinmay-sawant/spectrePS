package ps

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestValidationStackExtra locks index, roll with a negative shift, clear,
// counttomark, and copy, including their error paths.
func TestValidationStackExtra(t *testing.T) {
	t.Run("index", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 3 1 index"), 1, 2, 3, 2)
		assertIntValues(t, assertRun(t, "1 2 3 2 index"), 1, 2, 3, 1)
		assertPSError(t, "1 2 -1 index", "rangecheck")
		assertPSError(t, "1 2 1.0 index", "typecheck")
		assertPSError(t, "1 2 5 index", "stackunderflow")
	})
	t.Run("roll", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 3 3 1 roll"), 3, 1, 2)
		assertIntValues(t, assertRun(t, "1 2 3 3 -1 roll"), 2, 3, 1)
		assertIntValues(t, assertRun(t, "1 2 3 3 0 roll"), 1, 2, 3)
		assertIntValues(t, assertRun(t, "1 2 0 5 roll"), 1, 2)
		assertPSError(t, "1 2 -1 1 roll", "rangecheck")
		assertPSError(t, "1 2 1.0 1 roll", "typecheck")
		assertPSError(t, "1 2 5 1 roll", "stackunderflow")
	})
	t.Run("clear", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 3 clear"))
		assertIntValues(t, assertRun(t, "clear"))
	})
	t.Run("counttomark", validationCountToMark)
	t.Run("copy", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 3 2 copy"), 1, 2, 3, 2, 3)
		assertIntValues(t, assertRun(t, "1 2 0 copy"), 1, 2)
		assertPSError(t, "1 2 3.5 copy", "typecheck")
		assertPSError(t, "1 2 -1 copy", "rangecheck")
		assertPSError(t, "1 2 5 copy", "stackunderflow")
	})
}

func validationCountToMark(t *testing.T) {
	t.Helper()
	got := assertRun(t, "1 2 mark 3 4 counttomark")
	validationWantKinds(t, got, KindInt, KindInt, KindMark, KindInt, KindInt, KindInt)
	if got[5].Int != 2 {
		t.Fatalf("count = %d, want 2", got[5].Int)
	}
	got = assertRun(t, "1 mark counttomark")
	validationWantKinds(t, got, KindInt, KindMark, KindInt)
	if got[0].Int != 1 || got[2].Int != 0 {
		t.Fatalf("1 mark counttomark stack = %+v, want 1 mark 0", got)
	}
	assertPSError(t, "1 2 counttomark", "unmatchedmark")
}

func validationWantKinds(t *testing.T, got []Object, want ...Kind) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("stack len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i, kind := range want {
		if got[i].Kind != kind {
			t.Fatalf("stack[%d].Kind = %v, want %v", i, got[i].Kind, kind)
		}
	}
}

// TestValidationMathOps locks the math set and its real and integer edges.
func TestValidationMathOps(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "7 3 sub"), 4)
		assertIntValues(t, assertRun(t, "6 3 mul"), 18)
		assertIntValues(t, assertRun(t, "7 2 idiv"), 3)
		assertIntValues(t, assertRun(t, "-7 2 idiv"), -3)
		assertIntValues(t, assertRun(t, "7 3 mod"), 1)
		assertIntValues(t, assertRun(t, "-7 3 mod"), -1)
		assertIntValues(t, assertRun(t, "5 neg"), -5)
		assertIntValues(t, assertRun(t, "-5 neg"), 5)
		assertIntValues(t, assertRun(t, "-5 abs"), 5)
		assertIntValues(t, assertRun(t, "2 ceiling"), 2)
		assertIntValues(t, assertRun(t, "2 floor"), 2)
		assertIntValues(t, assertRun(t, "2 round"), 2)
		assertIntValues(t, assertRun(t, "-2147483648 -1 mod"), 0)
	})
	t.Run("real", func(t *testing.T) {
		assertReal(t, assertRun(t, "2.5 1 sub"), 1.5)
		assertReal(t, assertRun(t, "1.5 2 mul"), 3)
		assertReal(t, assertRun(t, "2.5 neg"), -2.5)
		assertReal(t, assertRun(t, "-2.5 abs"), 2.5)
		assertReal(t, assertRun(t, "1.2 ceiling"), 2)
		assertReal(t, assertRun(t, "1.8 floor"), 1)
		assertReal(t, assertRun(t, "1.5 round"), 2)
		assertReal(t, assertRun(t, "2.5 round"), 3)
		assertReal(t, assertRun(t, "-1.5 round"), -1)
	})
	t.Run("errors", func(t *testing.T) {
		assertPSError(t, "1 0 idiv", "undefinedresult")
		assertPSError(t, "1 0 mod", "undefinedresult")
		assertPSError(t, "1.5 1 idiv", "typecheck")
		assertPSError(t, "1.5 1 mod", "typecheck")
		assertPSError(t, "-2147483648 neg", "rangecheck")
		assertPSError(t, "-2147483648 abs", "rangecheck")
		assertPSError(t, "-2147483648 -1 idiv", "rangecheck")
	})
}

// TestValidationCompareLogic locks comparison and logic in boolean and
// bitwise forms.
func TestValidationCompareLogic(t *testing.T) {
	t.Run("eq and ne", func(t *testing.T) {
		assertBool(t, "1 1.0 eq", true)
		assertBool(t, "1.0 1 eq", true)
		assertBool(t, "1 1.5 eq", false)
		assertBool(t, "1.0 1.0 eq", true)
		assertBool(t, "1 2 ne", true)
		assertBool(t, "1.0 1 ne", false)
	})
	t.Run("order", func(t *testing.T) {
		assertBool(t, "2 1 gt", true)
		assertBool(t, "1 2 gt", false)
		assertBool(t, "2 2 ge", true)
		assertBool(t, "1 1.0 ge", true)
		assertBool(t, "1 2 lt", true)
		assertBool(t, "2 1 lt", false)
		assertBool(t, "2 2 le", true)
		assertBool(t, "2.5 2 le", false)
	})
	t.Run("boolean logic", func(t *testing.T) {
		assertBool(t, "true false and", false)
		assertBool(t, "true true and", true) //nolint:dupword // the same boolean on both operands
		assertBool(t, "true false or", true)
		assertBool(t, "false false or", false) //nolint:dupword // the same boolean on both operands
		assertBool(t, "true false xor", true)
		assertBool(t, "true true xor", false) //nolint:dupword // the same boolean on both operands
		assertBool(t, "true not", false)
		assertBool(t, "false not", true)
	})
	t.Run("bitwise logic", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "12 10 and"), 8)
		assertIntValues(t, assertRun(t, "4 1 or"), 5)
		assertIntValues(t, assertRun(t, "12 10 xor"), 6)
		assertIntValues(t, assertRun(t, "5 not"), -6)
		assertIntValues(t, assertRun(t, "0 not"), -1)
	})
	t.Run("typecheck", func(t *testing.T) {
		assertPSError(t, "true 1 and", "typecheck")
		assertPSError(t, "1.0 1 or", "typecheck")
		assertPSError(t, "1.0 not", "typecheck")
		assertPSError(t, "null not", "typecheck")
		assertPSError(t, "2 true gt", "typecheck")
	})
}

// TestValidationTypeOps locks the type and conversion set with null.
func TestValidationTypeOps(t *testing.T) {
	t.Run("type", validationTypeNames)
	t.Run("xcheck", validationXCheck)
	t.Run("cvi and cvr", validationNumberConversions)
	t.Run("cvs and cvn", validationStringConversions)
	t.Run("cvx and cvlit", validationAccess)
	t.Run("length get put", validationLengthGetPut)
	t.Run("null", validationNull)
}

func validationTypeNames(t *testing.T) {
	t.Helper()
	cases := []struct {
		src  string
		want string
	}{
		{"1 type", "integertype"},
		{"1.5 type", "realtype"},
		{"true type", "booleantype"},
		{"null type", "nulltype"},
		{"/a type", "nametype"},
		{"(a) type", "stringtype"},
		{"[1] type", "arraytype"},
		{"<<>> type", "dicttype"},
		{"mark type", "marktype"},
		{"{1} type", "arraytype"},
	}
	for _, tt := range cases {
		got := assertRun(t, tt.src)
		if len(got) != 1 {
			t.Fatalf("Run(%q) stack = %+v, want one object", tt.src, got)
		}
		validationWantName(t, got[0], tt.want, false)
	}
}

func validationXCheck(t *testing.T) {
	t.Helper()
	assertBool(t, "(abc) xcheck", false)
	assertBool(t, "/a xcheck", false)
	assertBool(t, "{1} xcheck", true)
	assertBool(t, "(abc) cvx xcheck", true)
	assertBool(t, "/a cvx xcheck", true)
	assertBool(t, "1 xcheck", false)
	assertBool(t, "1.5 xcheck", false)
	assertBool(t, "true xcheck", false)
	assertBool(t, "null xcheck", false)
	assertBool(t, "mark xcheck", false)
	assertBool(t, "<<>> xcheck", false)
}

func validationNumberConversions(t *testing.T) {
	t.Helper()
	assertIntValues(t, assertRun(t, "3 cvi"), 3)
	assertIntValues(t, assertRun(t, "3.9 cvi"), 3)
	assertIntValues(t, assertRun(t, "-3.9 cvi"), -3)
	assertIntValues(t, assertRun(t, "( 42 ) cvi"), 42)
	assertIntValues(t, assertRun(t, "(4.9) cvi"), 4)
	assertPSError(t, "(abc) cvi", "syntaxerror")
	assertPSError(t, "true cvi", "typecheck")
	assertPSError(t, "(2147483648) cvi", "rangecheck")
	assertReal(t, assertRun(t, "3 cvr"), 3)
	assertReal(t, assertRun(t, "(3) cvr"), 3)
	assertReal(t, assertRun(t, "(3.5) cvr"), 3.5)
	assertPSError(t, "(abc) cvr", "syntaxerror")
	assertPSError(t, "true cvr", "typecheck")
	assertPSError(t, "(1e999) cvr", "rangecheck")
}

func validationStringConversions(t *testing.T) {
	t.Helper()
	validationWantString(t, "42 cvs", "42")
	validationWantString(t, "3.5 cvs", "3.5")
	validationWantString(t, "3.0 cvs", "3.0")
	validationWantString(t, "/foo cvs", "foo")
	validationWantString(t, "true cvs", "true")
	validationWantString(t, "null cvs", "--nostringval--")
	validationWantString(t, "(abc) cvs", "--nostringval--")
	got := assertRun(t, "(abc) cvn")
	if len(got) != 1 {
		t.Fatalf("(abc) cvn stack = %+v, want one object", got)
	}
	validationWantName(t, got[0], "abc", false)
	got = assertRun(t, "(abc) cvx cvn")
	if len(got) != 1 {
		t.Fatalf("(abc) cvx cvn stack = %+v, want one object", got)
	}
	validationWantName(t, got[0], "abc", true)
	assertPSError(t, "1 cvn", "typecheck")
}

func validationAccess(t *testing.T) {
	t.Helper()
	assertBool(t, "{1} cvlit xcheck", false)
	assertBool(t, "{1} cvx xcheck", true)
	assertPSError(t, "1 cvx", "typecheck")
	assertPSError(t, "null cvlit", "typecheck")
}

func validationLengthGetPut(t *testing.T) {
	t.Helper()
	assertIntValues(t, assertRun(t, "(abc) length"), 3)
	assertIntValues(t, assertRun(t, "[1 2 3] length"), 3)
	assertIntValues(t, assertRun(t, "<< /a 1 /b 2 >> length"), 2)
	assertIntValues(t, assertRun(t, "/abc length"), 3)
	assertPSError(t, "1 length", "typecheck")
	assertIntValues(t, assertRun(t, "[10 20 30] 1 get"), 20)
	assertIntValues(t, assertRun(t, "(abc) 1 get"), 98)
	assertIntValues(t, assertRun(t, "<< /a 7 >> /a get"), 7)
	assertPSError(t, "[10] 5 get", "rangecheck")
	assertPSError(t, "[10] -1 get", "rangecheck")
	assertPSError(t, "[10] 1.0 get", "typecheck")
	assertPSError(t, "1 0 get", "typecheck")
	assertPSError(t, "<<>> /a get", "undefined")
	assertIntValues(t, assertRun(t, "(abc) dup 0 88 put 0 get"), 88)
	assertIntValues(t, assertRun(t, "[1 2 3] dup 0 9 put 0 get"), 9)
	assertIntValues(t, assertRun(t, "<< /a 1 >> dup /a 2 put /a get"), 2)
	assertPSError(t, "(abc) 0 300 put", "rangecheck")
	assertPSError(t, "(abc) 0 1.5 put", "typecheck")
	assertPSError(t, "(abc) 5 1 put", "rangecheck")
	assertPSError(t, "1 0 1 put", "typecheck")
	assertPSError(t, "[1] 5 2 put", "rangecheck")
	assertPSError(t, "[1] /a 2 put", "typecheck")
	assertPSError(t, "<<>> 1 2 put", "typecheck")
}

func validationNull(t *testing.T) {
	t.Helper()
	got := assertRun(t, "null")
	if len(got) != 1 || got[0].Kind != KindNull {
		t.Fatalf("null stack = %+v, want one null", got)
	}
}

// TestValidationControlFlow locks loop, forall, and exit in each loop form,
// plus a real-valued for.
func TestValidationControlFlow(t *testing.T) {
	t.Run("loop with exit", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "0 { 1 add dup 5 ge { exit } if } loop"), 5)
	})
	t.Run("forall array", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "[1 2 3] { } forall"), 1, 2, 3)
		assertIntValues(t, assertRun(t, "0 [1 2 3] { add } forall"), 6)
	})
	t.Run("forall string", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "(abc) { } forall"), 97, 98, 99)
		assertIntValues(t, assertRun(t, "(abc) { 1 add } forall"), 98, 99, 100)
	})
	t.Run("forall typecheck", func(t *testing.T) {
		assertPSError(t, "5 { } forall", "typecheck")
		assertPSError(t, "1.5 { } forall", "typecheck")
	})
	t.Run("exit in repeat", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "3 { exit } repeat"))
	})
	t.Run("exit in for", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 1 3 { exit } for"), 1)
	})
	t.Run("exit in forall", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "[1 2 3] { exit } forall"), 1)
	})
	t.Run("real for", func(t *testing.T) {
		got := assertRun(t, "0.5 0.5 2.5 { } for")
		want := []float64{0.5, 1, 1.5, 2, 2.5}
		if len(got) != len(want) {
			t.Fatalf("real for stack = %+v, want %d values", got, len(want))
		}
		for i, value := range want {
			if got[i].Kind != KindReal || got[i].Real != value {
				t.Fatalf("real for stack[%d] = %+v, want real %v", i, got[i], value)
			}
		}
	})
}

// TestValidationPSSGraphics locks the device effect of every path, paint, and
// matrix operator the subset lists.
func TestValidationPSSGraphics(t *testing.T) {
	t.Run("path", validationPSPath)
	t.Run("paint", validationPSPaint)
	t.Run("style", validationPSStyle)
	t.Run("showpage", validationPSShowPage)
	t.Run("matrix", validationPSMatrix)
}

func validationPSPath(t *testing.T) {
	t.Helper()
	pages := validationPages(t, "10 10 moveto 5 0 rmoveto 0 5 rlineto stroke", 20, 20)
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	validationWantPixel(t, pages[0], 15, 12, 0, 0, 0)
	validationWantWhite(t, pages[0], 10, 12)
	validationWantWhite(t, pages[0], 15, 8)

	pages = validationPages(t, "2 2 moveto 16 0 16 16 0 16 rcurveto stroke", 20, 20)
	validationWantPixel(t, pages[0], 10, 2, 0, 0, 0)
	validationWantPixel(t, pages[0], 18, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 10, 10)

	pages = validationPages(t, "0 0 moveto 10 0 lineto 10 10 lineto closepath fill", 20, 20)
	validationWantPixel(t, pages[0], 7, 3, 0, 0, 0)
	validationWantWhite(t, pages[0], 3, 7)

	pages = validationPages(t, "0 0 moveto newpath stroke", 20, 20)
	validationWantWhite(t, pages[0], 0, 0)
	validationWantWhite(t, pages[0], 10, 10)

	pages = validationPages(t, "0 0 moveto 10 0 lineto stroke", 20, 20)
	validationWantPixel(t, pages[0], 5, 0, 0, 0, 0)
	validationWantWhite(t, pages[0], 5, 5)
}

func validationPSPaint(t *testing.T) {
	t.Helper()
	square := "0 0 moveto 10 0 lineto 10 10 lineto 0 10 lineto closepath"
	filled := validationPages(t, square+" fill", 20, 20)
	validationWantPixel(t, filled[0], 5, 5, 0, 0, 0)

	hole := "0 0 moveto 30 0 lineto 30 30 lineto 0 30 lineto closepath " +
		"8 8 moveto 22 8 lineto 22 22 lineto 8 22 lineto closepath"
	evenOdd := validationPages(t, hole+" eofill", 40, 40)
	nonzero := validationPages(t, hole+" fill", 40, 40)
	validationWantPixel(t, evenOdd[0], 2, 2, 0, 0, 0)
	validationWantWhite(t, evenOdd[0], 15, 15)
	validationWantPixel(t, nonzero[0], 15, 15, 0, 0, 0)

	red := validationPages(t,
		"1 0 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill", 20, 20)
	validationWantPixel(t, red[0], 10, 10, 255, 0, 0)

	gray := validationPages(t,
		"0.5 setgray 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill", 20, 20)
	validationWantPixel(t, gray[0], 10, 10, 128, 128, 128)
}

func validationPSStyle(t *testing.T) {
	t.Helper()
	pages := validationPages(t, "4 setlinewidth 0 10 moveto 20 10 lineto stroke", 20, 20)
	validationWantPixel(t, pages[0], 10, 10, 0, 0, 0)
	validationWantWhite(t, pages[0], 10, 7)
}

func validationPSShowPage(t *testing.T) {
	t.Helper()
	painted := validationPages(t, "0 0 moveto 10 0 lineto stroke showpage", 20, 20)
	if len(painted) != 1 {
		t.Fatalf("pages = %d, want 1", len(painted))
	}
	validationWantPixel(t, painted[0], 5, 0, 0, 0, 0)
	blank := validationPages(t, "showpage showpage", 20, 20) //nolint:dupword // two showpage operators produce two pages
	if len(blank) != 2 {
		t.Fatalf("pages = %d, want 2", len(blank))
	}
	validationWantWhite(t, blank[0], 5, 0)
	validationWantWhite(t, blank[1], 5, 0)
}

func validationPSMatrix(t *testing.T) {
	t.Helper()
	pages := validationPages(t, "90 rotate 0 0 moveto 10 0 lineto stroke", 20, 20)
	validationWantPixel(t, pages[0], 0, 5, 0, 0, 0)
	validationWantWhite(t, pages[0], 5, 5)

	pages = validationPages(t, "2 0 0 2 0 0 concat 0 0 moveto 5 0 lineto stroke", 20, 20)
	validationWantPixel(t, pages[0], 10, 0, 0, 0, 0)
	validationWantWhite(t, pages[0], 11, 0)

	pages = validationPages(t,
		"5 5 translate 0 0 moveto 10 0 lineto stroke "+
			"1 0 0 1 0 0 setmatrix 0 0 moveto 4 0 lineto stroke", 20, 20)
	validationWantPixel(t, pages[0], 10, 5, 0, 0, 0)
	validationWantPixel(t, pages[0], 2, 0, 0, 0, 0)
	validationWantWhite(t, pages[0], 10, 0)

	assertFloats(t, "10 20 translate currentmatrix", 1, 0, 0, 1, 10, 20)
	assertFloats(t, "2 0 0 2 3 4 setmatrix currentmatrix", 2, 0, 0, 2, 3, 4)
}

func validationWantName(t *testing.T, obj Object, name string, exec bool) {
	t.Helper()
	if obj.Kind != KindName || obj.Name != name || obj.Exec != exec {
		t.Fatalf("object kind=%v name=%q exec=%v, want name %q exec=%v",
			obj.Kind, obj.Name, obj.Exec, name, exec)
	}
}

func validationWantString(t *testing.T, src, want string) {
	t.Helper()
	got := assertRun(t, src)
	if len(got) != 1 || got[0].Kind != KindString || got[0].Str == nil {
		t.Fatalf("Run(%q) stack = %+v, want one string", src, got)
	}
	if string(got[0].Str.Bytes) != want {
		t.Fatalf("Run(%q) = %q, want %q", src, got[0].Str.Bytes, want)
	}
}

// validationPages runs src against a fresh pixmap and returns its pages.
func validationPages(t *testing.T, src string, width, height int) []graphics.Image {
	t.Helper()
	interp := NewInterp()
	pixmap := graphics.NewPixmap(width, height)
	interp.UsePixmap(pixmap, 1)
	if err := interp.Run(t.Context(), []byte(src)); err != nil {
		t.Fatalf("Run(%q) error = %v", src, err)
	}
	return pixmap.Pages()
}

// validationPixel reads the pixel at column col and y points above the page
// bottom. At scale 1 the device pixel grid matches the user grid.
func validationPixel(img graphics.Image, col, y int) (byte, byte, byte) {
	row := img.Height - 1 - y
	i := row*img.Stride + col*3
	return img.Pixels[i], img.Pixels[i+1], img.Pixels[i+2]
}

func validationWantPixel(t *testing.T, img graphics.Image, col, y int, red, green, blue byte) {
	t.Helper()
	gotRed, gotGreen, gotBlue := validationPixel(img, col, y)
	if gotRed != red || gotGreen != green || gotBlue != blue {
		t.Fatalf("pixel (%d,%d) = %d,%d,%d, want %d,%d,%d",
			col, y, gotRed, gotGreen, gotBlue, red, green, blue)
	}
}

func validationWantWhite(t *testing.T, img graphics.Image, col, y int) {
	t.Helper()
	validationWantPixel(t, img, col, y, 255, 255, 255)
}
