package pdfout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	copyContentNum = 4
	copyFontNum    = 5
	copyAnnotNum   = 6
)

func TestCopyObjects(t *testing.T) {
	checkCopyRoundTrip(t)
	checkCopyOverride(t)
	checkCopyStable(t)
	checkCopySerialized(t)
	checkCopyError(t)
	checkCopyContext(t)
}

func checkCopyRoundTrip(t *testing.T) {
	t.Helper()
	out := mustCopy(t, mustOpenPDF(t, copyFixture(t)), CopyOptions{})
	if !bytes.HasPrefix(out, []byte(headerLine)) {
		t.Fatal("bad header")
	}
	reopened := mustOpenPDF(t, out)
	if reopened.PageCount() != 1 {
		t.Fatalf("pages %d", reopened.PageCount())
	}
	content, err := reopened.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(lineMarks)) {
		t.Fatalf("content %q", content)
	}
	checkCopiedValue(t, reopened, copyFontNum, "BaseFont", "Helvetica")
	checkCopiedValue(t, reopened, copyAnnotNum, "Subtype", "Text")
}

func checkCopyOverride(t *testing.T) {
	t.Helper()
	file := mustOpenPDF(t, copyFixture(t))
	marks := []byte("0 0 m 20 0 l S")
	for _, compress := range []bool{false, true} {
		_, body, err := streamParts(marks, compress)
		if err != nil {
			t.Fatal(err)
		}
		opt := CopyOptions{Overrides: map[int][]byte{copyContentNum: body}}
		reopened := mustOpenPDF(t, mustCopy(t, file, opt))
		if reopened.PageCount() != 1 {
			t.Fatalf("compress %v pages %d", compress, reopened.PageCount())
		}
		content, err := reopened.Content(0)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, marks) {
			t.Fatalf("compress %v content %q", compress, content)
		}
		checkCopiedValue(t, reopened, copyFontNum, "BaseFont", "Helvetica")
	}
}

func checkCopyStable(t *testing.T) {
	t.Helper()
	file := mustOpenPDF(t, copyFixture(t))
	first := mustCopy(t, file, CopyOptions{})
	second := mustCopy(t, file, CopyOptions{})
	if !bytes.Equal(first, second) {
		t.Fatal("two calls differ")
	}
	rejectDates(t, first)
	if bytes.Contains(first, []byte("/Info")) {
		t.Fatal("file contains /Info")
	}
	if !bytes.Contains(first, []byte("/ID [")) {
		t.Fatal("missing /ID [")
	}
}

// checkCopySerialized drives a fake source whose catalog lives in an object
// stream, so the writer has to serialize it. Object 1 is a free hole and the
// root is not object 1.
func checkCopySerialized(t *testing.T) {
	t.Helper()
	rawPages := []byte("<< /Type /Pages /Kids [4 0 R] /Count 1 >>")
	rawPage := []byte("<< /Type /Page /Parent 3 0 R >>")
	rawFont := []byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	catalog := pdf.DictVal(map[string]pdf.Value{
		"Type":  pdf.NameVal("Catalog"),
		"Pages": pdf.RefVal(3, 0),
	})
	src := &fakeSource{
		count:  5,
		root:   2,
		raw:    map[int][]byte{3: rawPages, 4: rawPage, 5: rawFont},
		values: map[int]pdf.Value{2: catalog},
	}
	out := mustCopy(t, src, CopyOptions{})
	serialized := pdf.SerializeValue(catalog)
	if !bytes.Contains(out, serialized) {
		t.Fatalf("missing serialized catalog %q", serialized)
	}
	wantID(t, out, serialized, rawPages, rawPage, rawFont)
	if !bytes.Contains(out, []byte("/Size 6")) || !bytes.Contains(out, []byte("/Root 2 0 R")) {
		t.Fatal("trailer size or root wrong")
	}
	if bytes.Count(out, []byte(xrefRow(0, 0, 'f'))) != 1 {
		t.Fatal("free row for object 1 missing")
	}
	reopened := mustOpenPDF(t, out)
	if reopened.PageCount() != 1 {
		t.Fatalf("pages %d", reopened.PageCount())
	}
	checkCopiedValue(t, reopened, 5, "BaseFont", "Helvetica")
	if _, ok := reopened.RawObject(1); ok {
		t.Fatal("free object 1 has bytes")
	}
}

func checkCopyError(t *testing.T) {
	t.Helper()
	job := pdf.NewError("Copy", "syntaxerror")
	src := &fakeSource{count: 1, root: 1, fail: map[int]error{1: job}}
	got, err := WriteCopy(t.Context(), src, CopyOptions{})
	if err == nil || got != nil {
		t.Fatalf("got %#v err %v", got, err)
	}
	var want *pdf.Error
	if !errors.As(err, &want) || want.Op != "Copy" {
		t.Fatalf("err %v", err)
	}
}

func checkCopyContext(t *testing.T) {
	t.Helper()
	src := &fakeSource{count: 1, root: 1}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := WriteCopy(ctx, src, CopyOptions{})
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("canceled %v %#v", err, got)
	}
	defer func() {
		recovered := recover()
		if recovered != panicNilContext {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_, _ = WriteCopy(nil, src, CopyOptions{}) //nolint:staticcheck // nil context is the case under test
}

func mustCopy(t *testing.T, src CopySource, opt CopyOptions) []byte {
	t.Helper()
	got, err := WriteCopy(t.Context(), src, opt)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustOpenPDF(t *testing.T, src []byte) *pdf.File {
	t.Helper()
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func checkCopiedValue(t *testing.T, file *pdf.File, num int, key, want string) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	got, found := val.NameEntry(key)
	if !found || got != want {
		t.Fatalf("object %d %s = %q found %v, want %q", num, key, got, found, want)
	}
}

type fakeSource struct {
	count  int
	root   int
	raw    map[int][]byte
	values map[int]pdf.Value
	fail   map[int]error
}

func (src *fakeSource) ObjectCount() int { return src.count }

func (src *fakeSource) RootNum() int { return src.root }

func (src *fakeSource) RawObject(num int) ([]byte, bool) {
	body, ok := src.raw[num]
	return body, ok
}

func (src *fakeSource) ObjectValue(num int) (pdf.Value, bool, error) {
	if err, ok := src.fail[num]; ok {
		return pdf.NullVal(), true, err
	}
	val, ok := src.values[num]
	return val, ok, nil
}

// copyFixture builds a one-page PDF with a font and an annotation.
func copyFixture(t *testing.T) []byte {
	t.Helper()
	doc := newFixtureDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> /Annots [6 0 R] >>"
	doc.object(page)
	doc.object(string(streamBody([]byte(lineMarks), false)))
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	doc.object("<< /Type /Annot /Subtype /Text /Rect [0 0 10 10] /Contents (note) >>")
	return doc.classic()
}

type fixtureDoc struct {
	buf     bytes.Buffer
	offsets []int
}

func newFixtureDoc() *fixtureDoc {
	doc := &fixtureDoc{offsets: []int{0}}
	doc.buf.WriteString(headerLine)
	return doc
}

func (doc *fixtureDoc) object(body string) {
	doc.offsets = append(doc.offsets, doc.buf.Len())
	num := len(doc.offsets) - 1
	fmt.Fprintf(&doc.buf, "%d 0 obj\n%s\nendobj\n", num, body)
}

func (doc *fixtureDoc) classic() []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefRow(0, freeGen, 'f'))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefRow(doc.offsets[num], 0, 'n'))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}
