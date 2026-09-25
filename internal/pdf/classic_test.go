package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	lineMarks = "0 0 m 10 0 l S"
	freeGen   = 65535
	xrefWidth = 20

	idCatalog = 1
	idPages   = 2
	idNested  = 3
	idPage    = 4
	idContent = 5
)

func TestClassicXref(t *testing.T) {
	// /Count is 2 and /Pages nests another /Pages. The walk still finds one leaf.
	src := classicLine(t)
	file := mustOpen(t, src)
	if file.PageCount() != 1 {
		t.Fatalf("pages %d", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(lineMarks)) {
		t.Fatalf("content %q", got)
	}
	_, err = file.Content(1)
	wantJob(t, err, opRaster, errRange)
	joined := mustOpen(t, joinedPage(t))
	got, err = joined.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("q\nQ")) {
		t.Fatalf("joined %q", got)
	}
	openContext(t)
}

type pdfDoc struct {
	buf     bytes.Buffer
	offsets []int
}

func newDoc() *pdfDoc {
	doc := &pdfDoc{offsets: []int{0}}
	doc.buf.WriteString("%PDF-1.4\n")
	return doc
}

func (doc *pdfDoc) object(body string) int {
	num := len(doc.offsets)
	doc.offsets = append(doc.offsets, doc.buf.Len())
	fmt.Fprintf(&doc.buf, "%d 0 obj\n%s\nendobj\n", num, body)
	return num
}

func (doc *pdfDoc) put(num int, body string) {
	for len(doc.offsets) <= num {
		doc.offsets = append(doc.offsets, -1)
	}
	doc.offsets[num] = doc.buf.Len()
	fmt.Fprintf(&doc.buf, "%d 0 obj\n%s\nendobj\n", num, body)
}

func (doc *pdfDoc) classic(extra string) []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefLine(0, freeGen, false))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefLine(doc.offsets[num], 0, true))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root %d 0 R%s >>\n", size, idCatalog, extra)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

func xrefLine(offset, gen int, inUse bool) string {
	flag := "f"
	if inUse {
		flag = "n"
	}
	line := fmt.Sprintf("%010d %05d %s \n", offset, gen, flag)
	if len(line) != xrefWidth {
		panic("pdf: xref line width")
	}
	return line
}

func streamBody(dict string, raw []byte) string {
	head := fmt.Sprintf("<< /Length %d >>\nstream\n", len(raw))
	if dict != "" {
		head = fmt.Sprintf("<< %s /Length %d >>\nstream\n", dict, len(raw))
	}
	return head + string(raw) + "\nendstream"
}

func flateRaw(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	if _, err := writer.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func classicLine(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	catalog := doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	pages := doc.object("<< /Type /Pages /Kids [3 0 R] /Count 2 >>")
	nested := doc.object("<< /Type /Pages /Kids [4 0 R] /Count 2 >>")
	page := doc.object("<< /Type /Page /Parent 3 0 R /Contents 5 0 R >>")
	raw := flateRaw(t, []byte(lineMarks))
	content := doc.object(streamBody("/Filter /FlateDecode", raw))
	if catalog != idCatalog || pages != idPages || nested != idNested || page != idPage || content != idContent {
		t.Fatal("object ids")
	}
	return doc.classic("")
}

func joinedPage(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents [4 0 R 5 0 R] >>")
	doc.object(streamBody("", []byte("q")))
	doc.object(streamBody("", []byte("Q")))
	return doc.classic("")
}

func openContext(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := Open(ctx, []byte("%PDF-"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled %v", err)
	}
	defer func() {
		got := recover()
		if got != panicNilCtx {
			t.Fatalf("panic %v", got)
		}
	}()
	_, _ = Open(nil, []byte("%PDF-")) //nolint:staticcheck // nil context is the case under test
}

func mustOpen(t *testing.T, src []byte) *File {
	t.Helper()
	file, err := Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func wantJob(t *testing.T, err error, opName, errName string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var got *Error
	if !errors.As(err, &got) {
		t.Fatalf("errors.As %v", err)
	}
	if got.Op != opName || got.Name != errName {
		t.Fatalf("op %s name %s", got.Op, got.Name)
	}
	if stringsContain(err.Error(), "not implemented") || stringsContain(got.Op, "not implemented") {
		t.Fatalf("not implemented: %v", err)
	}
}

func stringsContain(text, part string) bool {
	return len(part) > 0 && strings.Contains(text, part)
}
