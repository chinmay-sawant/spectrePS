package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"
)

// A startxref value that names nothing is recoverable: the reader rebuilds the
// table from the file's own object headers. That is what the batch2 corpus needs
// and what Ghostscript does, so the recovery is locked here rather than left to
// the corpus alone.
func TestRecoverCrossRef(t *testing.T) {
	t.Run("garbage startxref rebuilds", recoverGarbage)
	t.Run("damaged table at the offset stays refused", recoverDamagedTable)
	t.Run("no trailer root stays refused", recoverNoRoot)
	t.Run("valid table still wins", recoverValidTable)
}

func recoverGarbage(t *testing.T) {
	t.Helper()
	file := mustOpen(t, wrongStartxref(t, 3))
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
	report, err := file.Info()
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != "1.4" || report.Pages != 1 {
		t.Fatalf("report %+v", report)
	}
}

// recoverDamagedTable is the shape of structural/bad-xref.pdf: the startxref
// names a real table whose rows are damaged. The offset does present a section,
// so the reader reports the damage instead of rebuilding over it.
func recoverDamagedTable(t *testing.T) {
	t.Helper()
	_, err := Open(t.Context(), damagedTable(t))
	wantJob(t, err, opXRef, errSyntax)
}

// recoverNoRoot is the shape of
// structural/parser_rebuildxref_error_notrailer.pdf: a scan finds the objects,
// but no trailer carries a /Root. A scan alone is not enough to claim a
// document, so the reader keeps the failure it had before the scan ran, which is
// why the operator here is the one the offset named rather than xref.
func recoverNoRoot(t *testing.T) {
	t.Helper()
	_, err := Open(t.Context(), noRootTrailer(t))
	if err == nil {
		t.Fatal("expected error")
	}
	var got *Error
	if !errors.As(err, &got) {
		t.Fatalf("errors.As %v", err)
	}
	if got.Name != errSyntax {
		t.Fatalf("op %s name %s", got.Op, got.Name)
	}
}

// recoverValidTable proves the recovery did not displace a table that already
// reads: the same document resolves the same way with a good and a bad
// startxref value.
func recoverValidTable(t *testing.T) {
	t.Helper()
	good := mustOpen(t, classicLine(t))
	fixed := mustOpen(t, wrongStartxref(t, 3))
	if good.PageCount() != fixed.PageCount() {
		t.Fatalf("pages %d and %d", good.PageCount(), fixed.PageCount())
	}
	want, err := good.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fixed.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content %q", got)
	}
}

// wrongStartxref returns classicLine's bytes with the startxref value replaced
// by a number that points into the middle of the first object body.
func wrongStartxref(t *testing.T, offset int) []byte {
	t.Helper()
	src := append([]byte(nil), classicLine(t)...)
	mark := bytes.LastIndex(src, []byte(wordStart))
	if mark < 0 {
		t.Fatal("no startxref")
	}
	from := skipSpace(src, mark+len(wordStart))
	end := from
	for end < len(src) && isDigit(src[end]) {
		end++
	}
	out := append([]byte(nil), src[:from]...)
	out = append(out, strconv.Itoa(offset)...)
	return append(out, src[end:]...)
}

// damagedTable writes the objects, then a table whose rows carry trailing junk,
// then a startxref that names the keyword of that damaged table.
func damagedTable(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody("", []byte("q\nQ")))
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	for num := range size {
		flag := "n"
		if num == 0 {
			flag = "f"
		}
		fmt.Fprintf(&doc.buf, "%010d %05d %s junk\n", doc.offsets[num], 0, flag)
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

// noRootTrailer writes objects and a readable xref table, then points startxref
// into an object body and leaves the trailer without a /Root.
func noRootTrailer(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody("", []byte("q\nQ")))
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefLine(0, freeGen, false))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefLine(doc.offsets[num], 0, true))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", 3)
	return doc.buf.Bytes()
}
