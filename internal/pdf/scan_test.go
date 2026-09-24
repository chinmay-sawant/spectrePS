package pdf

import (
	"errors"
	"testing"
)

func TestScan(t *testing.T) {
	scanStrings(t)
	scanHex(t)
	scanNames(t)
	scanNumbers(t)
	scanKeywords(t)
	scanRejects(t)
}

func scanStrings(t *testing.T) {
	t.Helper()
	cases := []struct {
		src  string
		want string
	}{
		{src: `(a\n)`, want: "a\n"},
		{src: `(a\r)`, want: "a\r"},
		{src: `(a\t)`, want: "a\t"},
		{src: `(a\b)`, want: "a\b"},
		{src: `(a\f)`, want: "a\f"},
		{src: `(\\)`, want: `\`},
		{src: `(\()`, want: "("},
		{src: `(\))`, want: ")"},
		{src: `(A\101)`, want: "AA"},
		{src: `(\7)`, want: string(byte(7))},
		{src: `(\12)`, want: "\n"},
		{src: `(a(b))`, want: "a(b)"},
		{src: `(\a)`, want: "a"},
		{src: "(a\\\nb)", want: "ab"},
		{src: "(a\\\r\nb)", want: "ab"},
		{src: "(a\r\nb)", want: "a\nb"},
	}
	for _, item := range cases {
		wantString(t, item.src, item.want)
	}
}

func scanHex(t *testing.T) {
	t.Helper()
	wantString(t, "<4869>", "Hi")
	wantString(t, "<48 69>", "Hi")
	wantString(t, "<414>", string([]byte{0x41, 0x40}))
	wantString(t, "<>", "")
}

func scanNames(t *testing.T) {
	t.Helper()
	wantName(t, "/A#20B", "A B")
	wantName(t, "/Type", "Type")
	wantName(t, "/", "")
}

func scanNumbers(t *testing.T) {
	t.Helper()
	wantInt(t, "42", 42)
	wantInt(t, "-7", -7)
	wantInt(t, "+8", 8)
	wantReal(t, "1.5", 1.5)
	wantReal(t, ".5", 0.5)
	wantReal(t, "1.", 1)
	wantReal(t, "-2.25", -2.25)
	wantReal(t, "+.5", 0.5)
}

func scanKeywords(t *testing.T) {
	t.Helper()
	wantBool(t, "true", true)
	wantBool(t, "false", false)
	wantNull(t, "null")
	wantBool(t, "% comment\ntrue", true)
	wantBool(t, "\x00\t\r\n\f true", true)
}

func scanRejects(t *testing.T) {
	t.Helper()
	rejectValue(t, "(abc")
	rejectValue(t, "<41")
	rejectValue(t, "<4G>")
	rejectValue(t, ">")
	rejectValue(t, "/#2")
	rejectValue(t, "/A#2G")
	rejectValue(t, "1e2")
	rejectValue(t, "foo")
}

func wantString(t *testing.T, src, want string) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindString || val.String != want || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d %q next %d", src, val.Kind, val.String, next)
	}
}

func wantName(t *testing.T, src, want string) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindName || val.Name != want || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d name %q next %d", src, val.Kind, val.Name, next)
	}
}

func wantInt(t *testing.T, src string, want int64) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindInt || val.Int != want || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d int %d next %d", src, val.Kind, val.Int, next)
	}
}

func wantReal(t *testing.T, src string, want float64) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindReal || val.Real != want || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d real %v next %d", src, val.Kind, val.Real, next)
	}
}

func wantBool(t *testing.T, src string, want bool) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindBool || val.Bool != want || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d bool %v next %d", src, val.Kind, val.Bool, next)
	}
}

func wantNull(t *testing.T, src string) {
	t.Helper()
	val, next := mustValue(t, src)
	if val.Kind != KindNull || next != len(src) {
		t.Fatalf("ParseValue(%q) = kind %d next %d, want null", src, val.Kind, next)
	}
}

func mustValue(t *testing.T, src string) (Value, int) {
	t.Helper()
	val, next, err := ParseValue([]byte(src), 0)
	if err != nil {
		t.Fatalf("ParseValue(%q): %v", src, err)
	}
	return val, next
}

func rejectValue(t *testing.T, src string) {
	t.Helper()
	_, _, err := ParseValue([]byte(src), 0)
	job := wantSyntax(t, err)
	if job.Name != nameSyntax {
		t.Fatal(job)
	}
}

func wantSyntax(t *testing.T, err error) *Error {
	t.Helper()
	var job *Error
	if err == nil || !errors.As(err, &job) || job.Name != nameSyntax {
		t.Fatalf("error = %v, want *Error syntaxerror", err)
	}
	return job
}
