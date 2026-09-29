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
	return replaceStartxref(classicLine(t), offset)
}

// replaceStartxref rewrites the number after the last startxref keyword.
func replaceStartxref(src []byte, offset int) []byte {
	mark := bytes.LastIndex(src, []byte(wordStart))
	if mark < 0 {
		return src
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

// TestRecoverPrevScanFill locks the /Prev half of the recovery: when a /Prev
// link names no section at all, the sections already read stand and the rows
// they leave out come from the file's own object headers. A /Prev that names a
// section still reports that section's damage, which the cycle, malformed, and
// broken-older tests in validation_xref_prev_test.go pin.
func TestRecoverPrevScanFill(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, prevScanFill(t))
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
}

// prevScanFill writes a newest classic section that lists only the catalog and
// the page tree. Its /Prev points inside the content stream's body, which names
// no section, so the page and the content come from the object header scan.
func prevScanFill(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	catalogAt := body.Len()
	body.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	pagesAt := body.Len()
	body.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	body.WriteString("3 0 obj\n" + pageBody + "\nendobj\n")
	contentAt := body.Len()
	body.WriteString("4 0 obj\n" + streamBody("", []byte(lineMarks)) + "\nendobj\n")
	newestAt := body.Len()
	body.Write(validationClassicSection(
		[]objPos{{num: 0}, {num: 1, offset: catalogAt}, {num: 2, offset: pagesAt}},
		fmt.Sprintf("/Root 1 0 R /Prev %d", contentAt+3)))
	fmt.Fprintf(&body, "startxref\n%d\n%%%%EOF\n", newestAt)
	return body.Bytes()
}

// TestRecoverStreamXRef locks the xref stream half of the recovery: a startxref
// that names nothing still opens when the file carries a cross-reference stream
// of its own, because that stream is the file's table and trailer.
func TestRecoverStreamXRef(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, streamXRefNowhere(t))
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
}

// streamXRefNowhere takes the hand-built xref stream document and points its
// startxref value into the xref stream's own body, where no section starts.
func streamXRefNowhere(t *testing.T) []byte {
	t.Helper()
	src := buildXRefStream(t)
	at := bytes.LastIndex(src, []byte("stream\n")) + len("stream\n") + 4
	return replaceStartxref(src, at)
}

// TestRecoverObjectRowFromScan locks the per-row recovery: a row that names the
// wrong bytes loses to the object header the file carries, which is the local
// form of the rebuild Ghostscript performs after an invalid xref entry.
func TestRecoverObjectRowFromScan(t *testing.T) {
	t.Parallel()
	src := rowAtWrongOffset(t, classicLine(t), idContent, idPages)
	file := mustOpen(t, src)
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(lineMarks)) {
		t.Fatalf("content %q", got)
	}
}

// TestRecoverCompressedRow locks the same recovery for a compressed object: the
// row is plain and wrong, the object has no header of its own, and the object
// stream still carries it.
func TestRecoverCompressedRow(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, compressedRowDoc(t))
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
}

// compressedRowDoc writes the page as object 3 inside an object stream. The
// xref row for object 3 is in use but points at the catalog's bytes, so only the
// object stream index can place it.
func compressedRowDoc(t *testing.T) []byte {
	t.Helper()
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>"
	header := "3 0 0\n"
	plain := append([]byte(header), page...)
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.put(4, streamBody("", []byte(lineMarks)))
	doc.put(5, streamBody(fmt.Sprintf("/Type /ObjStm /N 1 /First %d /Filter /FlateDecode",
		len(header)), flateRaw(t, plain)))
	doc.deadRow(3, 1)
	return doc.classic("")
}

// rowAtWrongOffset rewrites the xref row for num in a finished classic document
// so it names wrongAt's bytes instead.
func rowAtWrongOffset(t *testing.T, doc []byte, num, wrongAt int) []byte {
	t.Helper()
	out := append([]byte(nil), doc...)
	copy(out[rowStart(t, out, num):], fmt.Sprintf("%010d", tableOffset(t, out, wrongAt)))
	return out
}

// rowStart returns the byte offset of the row for num in a classic table.
func rowStart(t *testing.T, doc []byte, num int) int {
	t.Helper()
	table := bytes.Index(doc, []byte("xref\n0 "))
	if table < 0 {
		t.Fatal("no xref table")
	}
	first := bytes.IndexByte(doc[table:], '\n')
	second := bytes.IndexByte(doc[table+first+1:], '\n')
	row := table + first + 1 + second + 1 + num*xrefWidth
	if row+xrefWidth > len(doc) {
		t.Fatalf("row %d past table", num)
	}
	return row
}

// tableOffset reads the offset the row for num carries in a classic table.
func tableOffset(t *testing.T, doc []byte, num int) int {
	t.Helper()
	offset, _, ok := pdfInt(doc, rowStart(t, doc, num))
	if !ok {
		t.Fatalf("row %d has no offset", num)
	}
	return offset
}

// TestRecoverInvalidContentRow locks the last recovery shape: a page whose
// /Contents row is in use, names bytes that are not that object, and whose
// object the header scan does not find. Ghostscript reports an invalid xref
// entry there and paints the page without content, so the reader opens the
// document and treats the content as empty.
func TestRecoverInvalidContentRow(t *testing.T) {
	t.Parallel()
	src := rowAtWrongOffset(t, classicLine(t), idContent, idPages)
	// Break the content object's own header without changing its length, so
	// the scan cannot place it either and the row is all that is left.
	src = bytes.Replace(src, []byte("5 0 obj"), []byte("5\x100 obj"), 1)
	file := mustOpen(t, src)
	if file.PageCount() != 1 {
		t.Fatalf("pages %d", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("content %q, want empty", got)
	}
}
