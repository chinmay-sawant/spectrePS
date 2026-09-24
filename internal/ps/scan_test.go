package ps

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

const (
	errRangeName  = "rangecheck"
	errSyntaxName = "syntaxerror"
	opScanName    = "scan"
)

func TestScan(t *testing.T) {
	commentAndInts(t)
	boundsAndReals(t)
	namesAndStrings(t)
	hexMarksSpace(t)
	rejects(t)
}

func commentAndInts(t *testing.T) {
	t.Helper()
	wantInt(t, "%!PS\n1", 1)
	wantInt(t, "%!PS-Adobe-3.0\n1", 1)
	wantInts(t, "123 -4 +8", 123, -4, 8)
	wantInt(t, "2147483647", math.MaxInt32)
	wantInt(t, "-2147483648", math.MinInt32)
	wantInts(t, "1% comment\n2", 1, 2)
}

func boundsAndReals(t *testing.T) {
	t.Helper()
	wantErrName(t, "2147483648", errRangeName)
	wantErrName(t, "-2147483649", errRangeName)
	wantErrName(t, "1e309", errRangeName)
	wantReal(t, "1.5", 1.5)
	wantReal(t, ".5", 0.5)
	wantReal(t, "1.", 1)
	wantReal(t, "1e2", 100)
	wantReal(t, "1E-2", 1e-2)
	wantReal(t, "-2.5", -2.5)
	wantReal(t, "2147483648.0", 2147483648)
}

func namesAndStrings(t *testing.T) {
	t.Helper()
	wantText(t, "add", TokName, "add")
	wantText(t, "/Add", TokLiteral, "Add")
	wantText(t, "moveto", TokName, "moveto")
	wantText(t, "MoveTo", TokName, "MoveTo")
	namesDiffer(t)
	wantText(t, "+", TokName, "+")
	wantText(t, "-", TokName, "-")
	wantText(t, "1e+", TokName, "1e+")
	wantBytes(t, "(hello)", []byte("hello"))
	wantBytes(t, `(a\n)`, []byte("a\n"))
	wantBytes(t, `(A\101)`, []byte{'A', 0x41})
	wantBytes(t, `(\))`, []byte{')'})
	wantBytes(t, `(\\)`, []byte{'\\'})
	wantBytes(t, "(a(b))", []byte("a(b)"))
	wantBytes(t, "(\\\nb)", []byte("b"))
	wantBytes(t, "(\\\r\nb)", []byte("b"))
}

func hexMarksSpace(t *testing.T) {
	t.Helper()
	wantBytes(t, "<41 42>", []byte("AB"))
	wantBytes(t, "<414>", []byte{0x41, 0x40})
	wantMarks(t)
	wantSpaceMix(t)
}

func rejects(t *testing.T) {
	t.Helper()
	wantErrName(t, "/", errSyntaxName)
	wantErrName(t, "/ ", errSyntaxName)
	wantErrName(t, "(", errSyntaxName)
	wantErrName(t, "<41", errSyntaxName)
	wantErrName(t, "<4G>", errSyntaxName)
	wantErrName(t, ")", errSyntaxName)
	wantErrName(t, ">", errSyntaxName)
}

func namesDiffer(t *testing.T) {
	t.Helper()
	toks := mustScan(t, "moveto MoveTo")
	if len(toks) != 2 || toks[0].Kind != TokName || toks[1].Kind != TokName || toks[0].Text == toks[1].Text {
		t.Fatalf("moveto/MoveTo = %+v", toks)
	}
}

func wantMarks(t *testing.T) {
	t.Helper()
	toks := mustScan(t, "{ } [ ] << >>")
	kinds := []TokKind{TokLBrace, TokRBrace, TokName, TokName, TokName, TokName}
	texts := []string{"", "", "[", "]", "<<", ">>"}
	if len(toks) != len(kinds) {
		t.Fatalf("marks: %d tokens, want %d", len(toks), len(kinds))
	}
	for i := range kinds {
		if toks[i].Kind != kinds[i] || toks[i].Text != texts[i] {
			t.Fatalf("marks[%d] = kind %d text %q", i, toks[i].Kind, toks[i].Text)
		}
	}
}

func wantSpaceMix(t *testing.T) {
	t.Helper()
	toks := mustScan(t, " \t1\r\nadd\f/Add\x00(hi) <41> { } ")
	if len(toks) != 7 {
		t.Fatalf("space mix: %d tokens, want 7: %+v", len(toks), toks)
	}
	if toks[0].Kind != TokInt || toks[0].Int != 1 || toks[1].Text != "add" || toks[2].Text != "Add" {
		t.Fatalf("space mix head = %+v", toks[:3])
	}
	if !bytes.Equal(toks[3].Bytes, []byte("hi")) || !bytes.Equal(toks[4].Bytes, []byte("A")) {
		t.Fatalf("space mix strings = %+v", toks[3:5])
	}
	if toks[5].Kind != TokLBrace || toks[6].Kind != TokRBrace {
		t.Fatalf("space mix braces = %+v", toks[5:])
	}
}

func wantInts(t *testing.T, src string, wants ...int32) {
	t.Helper()
	toks := mustScan(t, src)
	if len(toks) != len(wants) {
		t.Fatalf("Scan(%q) tokens %d, want %d", src, len(toks), len(wants))
	}
	for i := range wants {
		if toks[i].Kind != TokInt || toks[i].Int != wants[i] {
			t.Fatalf("Scan(%q)[%d] = kind %d int %d, want %d", src, i, toks[i].Kind, toks[i].Int, wants[i])
		}
	}
}

func wantInt(t *testing.T, src string, want int32) {
	t.Helper()
	got := mustOne(t, src)
	if got.Kind != TokInt || got.Int != want {
		t.Fatalf("Scan(%q) = kind %d int %d, want int %d", src, got.Kind, got.Int, want)
	}
}

func wantReal(t *testing.T, src string, want float64) {
	t.Helper()
	got := mustOne(t, src)
	if got.Kind != TokReal || got.Real != want {
		t.Fatalf("Scan(%q) = kind %d real %v, want real %v", src, got.Kind, got.Real, want)
	}
}

func wantText(t *testing.T, src string, kind TokKind, text string) {
	t.Helper()
	got := mustOne(t, src)
	if got.Kind != kind || got.Text != text {
		t.Fatalf("Scan(%q) = kind %d text %q, want kind %d text %q", src, got.Kind, got.Text, kind, text)
	}
}

func wantBytes(t *testing.T, src string, want []byte) {
	t.Helper()
	got := mustOne(t, src)
	if got.Kind != TokString || !bytes.Equal(got.Bytes, want) {
		t.Fatalf("Scan(%q) = kind %d bytes %q, want %q", src, got.Kind, got.Bytes, want)
	}
}

func wantErrName(t *testing.T, src, name string) {
	t.Helper()
	_, err := Scan([]byte(src))
	if err == nil {
		t.Fatalf("Scan(%q) error = nil, want %s", src, name)
	}
	var got *Error
	if !errors.As(err, &got) {
		t.Fatalf("Scan(%q) error = %T, want *Error", src, err)
	}
	if got.Name != name || got.Op != opScanName {
		t.Fatalf("Scan(%q) name = %q op = %q, want %s", src, got.Name, got.Op, name)
	}
}

func mustOne(t *testing.T, src string) Token {
	t.Helper()
	toks := mustScan(t, src)
	if len(toks) != 1 {
		t.Fatalf("Scan(%q) tokens %d, want 1", src, len(toks))
	}
	return toks[0]
}

func mustScan(t *testing.T, src string) []Token {
	t.Helper()
	toks, err := Scan([]byte(src))
	if err != nil {
		t.Fatalf("Scan(%q): %v", src, err)
	}
	return toks
}
