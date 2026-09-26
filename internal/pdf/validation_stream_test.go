package pdf

import (
	"bytes"
	"strconv"
	"testing"
)

// TestValidationStreamLengthIndirect opens a page whose content stream carries
// an indirect /Length N 0 R. The body itself contains the bytes endstream, so a
// reader that only scans would truncate it or fail; the declared span must win.
func TestValidationStreamLengthIndirect(t *testing.T) {
	body := "hello\nendstream\ntail"
	file := mustOpen(t, indirectLengthDoc(t, body))
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(body)) {
		t.Fatalf("content %q, want %q", got, body)
	}
}

// TestValidationStreamLengthResolved locks the resolver form of parseIndirect:
// an indirect /Length reads the resolved value.
func TestValidationStreamLengthResolved(t *testing.T) {
	src := []byte("1 0 obj\n<< /Length 9 0 R >>\nstream\nhello\nendstream\nendobj")
	_, _, val, next, err := parseIndirect(src, 0, stubLength(5))
	if err != nil {
		t.Fatalf("parseIndirect: %v", err)
	}
	if next != len(src) || val.Kind != KindStream || !bytes.Equal(val.Stream, []byte("hello")) {
		t.Fatalf("stream %q next %d kind %d", val.Stream, next, val.Kind)
	}
}

// stubLength is a lengthResolver with one fixed answer.
type stubLength int

func (length stubLength) resolveLength(Value) (int, bool) {
	return int(length), true
}

// TestValidationStreamLengthDirect locks an exact direct /Length.
func TestValidationStreamLengthDirect(t *testing.T) {
	src := []byte("1 0 obj\n<< /Length 5 >>\nstream\nhello\nendstream\nendobj")
	_, _, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("ParseIndirect: %v", err)
	}
	if next != len(src) || val.Kind != KindStream || !bytes.Equal(val.Stream, []byte("hello")) {
		t.Fatalf("stream %q next %d kind %d", val.Stream, next, val.Kind)
	}
}

// TestValidationStreamLengthWrong locks the scan fallback when the declared
// length disagrees with the bytes before endstream.
func TestValidationStreamLengthWrong(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "short",
			src:  "1 0 obj\n<< /Length 3 >>\nstream\nhello\nendstream\nendobj",
			want: "hello",
		},
		{
			name: "mid-body",
			src:  "1 0 obj\n<< /Length 6 >>\nstream\nhello world\nendstream\nendobj",
			want: "hello world",
		},
		{
			name: "past-end-of-file",
			src:  "1 0 obj\n<< /Length 999 >>\nstream\nhello\nendstream\nendobj",
			want: "hello",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, val, next, err := ParseIndirect([]byte(testCase.src), 0)
			if err != nil {
				t.Fatalf("ParseIndirect: %v", err)
			}
			if next != len(testCase.src) || val.Kind != KindStream ||
				!bytes.Equal(val.Stream, []byte(testCase.want)) {
				t.Fatalf("stream %q next %d kind %d", val.Stream, next, val.Kind)
			}
		})
	}
}

// TestValidationStreamEndstreamMissing locks the error contract: a stream with
// no endstream is syntaxerror in endstream, and a missing /Length is
// syntaxerror in Length.
func TestValidationStreamEndstreamMissing(t *testing.T) {
	job := mustFailIndirect(t, []byte("1 0 obj\n<< /Length 5 >>\nstream\nhello\nendobj"))
	if job.Op != wordEndStream {
		t.Fatalf("Op = %q, want %s", job.Op, wordEndStream)
	}
	job = mustFailIndirect(t, []byte("1 0 obj\n<< >>\nstream\nhello\nendstream\nendobj"))
	if job.Op != wordLength {
		t.Fatalf("Op = %q, want %s", job.Op, wordLength)
	}
}

// TestValidationXRefUnknownFilter locks the refusal for an xref stream behind
// an unknown filter. The error names the filter, so a reader never blames a
// predictor entry the unknown filter never reads.
func TestValidationXRefUnknownFilter(t *testing.T) {
	src := []byte("1 0 obj\n<< /Type /XRef /Filter /XXXDecode /Length 3" +
		" /W [1 1 1] /Size 1 >>\nstream\nabc\nendstream\nendobj")
	_, _, err := readStreamXRef(src, 0)
	wantJob(t, err, "XXXDecode", errUndefined)
}

// indirectLengthDoc builds a one-page PDF whose content stream is object 4
// with /Length 5 0 R, and whose object 5 holds the byte count.
func indirectLengthDoc(t *testing.T, body string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object("<< /Length 5 0 R >>\nstream\n" + body + "\nendstream")
	doc.object(strconv.Itoa(len(body)))
	return doc.classic("")
}
