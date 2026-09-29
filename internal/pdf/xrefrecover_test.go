package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"
)

// TestRecoverCrossRef locks the rebuild shapes: a startxref that names nothing
// or names a damaged table rebuilds from the file's own headers, a file with no
// trailer /Root still refuses, and a table that reads is never displaced.
func TestRecoverCrossRef(t *testing.T) {
	t.Run("garbage startxref rebuilds", recoverGarbage)
	t.Run("damaged table at the offset rebuilds", recoverDamagedTable)
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

// recoverDamagedTable is the shape of the batch2 files whose startxref names a
// real table whose rows carry trailing junk: the rows do not sit on the
// 20-byte grid, so the mapping from row slot to object number is lost and the
// reader rebuilds the table from the object headers, which is what Ghostscript
// does after reporting the same damage. structural/bad-xref.pdf is this shape.
func recoverDamagedTable(t *testing.T) {
	t.Helper()
	file := mustOpen(t, damagedTable(t))
	if file.PageCount() != 1 {
		t.Fatalf("pages %d", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("q\nQ")) {
		t.Fatalf("content %q", got)
	}
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

// TestRecoverGenerationFromHeader locks the generation half of the rebuild: a
// rebuilt table numbers objects from their own headers, so a trailer /Root
// whose generation disagrees with the header still resolves. Ghostscript reads
// it the same way, and GHOSTSCRIPT-695040-0.zip-31.pdf is the corpus shape.
func TestRecoverGenerationFromHeader(t *testing.T) {
	t.Parallel()
	src := bytes.Replace(damagedTable(t), []byte("/Root 1 0 R"), []byte("/Root 1 1 R"), 1)
	file := mustOpen(t, src)
	if file.PageCount() != 1 {
		t.Fatalf("pages %d", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("q\nQ")) {
		t.Fatalf("content %q", got)
	}
}

// TestValidTableGenerationStaysStrict locks the boundary of the tolerance: a
// table the file wrote keeps its own generation numbers, including the row for
// the trailer /Root, so a reference to another generation is still dead.
func TestValidTableGenerationStaysStrict(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody("", []byte(lineMarks)))
	src := bytes.Replace(doc.classic(""), []byte("/Root 1 0 R"), []byte("/Root 1 1 R"), 1)
	_, err := Open(t.Context(), src)
	wantJob(t, err, opXRef, errSyntax)
}

// TestRecoverContentLossFromMissingStream locks the content half of the
// rebuild: under a rebuilt table a /Contents reference the file does not carry
// is a page-local loss, not a document failure. Ghostscript reports the page as
// incomplete and paints the rest, and GHOSTSCRIPT-698699-0.pdf is the shape.
func TestRecoverContentLossFromMissingStream(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, damagedTableMissingContent(t))
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

// damagedTableMissingContent writes the same off-grid damaged table as
// damagedTable, but the page references a content object the file does not
// carry.
func damagedTableMissingContent(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 9 0 R >>")
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
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

// TestRecoverMissingStartxref locks the rebuild for a file whose startxref is
// absent or unreadable. Ghostscript reports "Cannot find a 'startxref' anywhere
// in the file", repairs from the object headers, and so does the reader. The
// original startxref error is only reported when the rebuild finds no trailer
// /Root, which recoverNoRoot pins.
func TestRecoverMissingStartxref(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, missingStartxrefDoc(t))
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

// missingStartxrefDoc writes a complete document whose final bytes carry the
// trailer and %%EOF with no startxref keyword at all.
func missingStartxrefDoc(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	body.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	body.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	body.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>\nendobj\n")
	body.WriteString("4 0 obj\n" + streamBody("", []byte(lineMarks)) + "\nendobj\n")
	body.WriteString("trailer\n<< /Size 5 /Root 1 0 R >>\n%%EOF\n")
	return body.Bytes()
}

// TestRecoverFreeRowFromHeader locks the row-level half of the rebuild for a
// free row: the table says the object is free, but the file carries its header,
// and Ghostscript's rebuild would use it. The dead-row contract is unchanged:
// a number the file does not carry anywhere still fails.
func TestRecoverFreeRowFromHeader(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, freeContentRow(t))
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(lineMarks)) {
		t.Fatalf("content %q", got)
	}
}

// freeContentRow writes the content object with a normal header and the row
// for it marked free.
func freeContentRow(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	contentNum := doc.object(streamBody("", []byte(lineMarks)))
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	for num := range size {
		inUse := num != 0 && num != contentNum
		doc.buf.WriteString(xrefLine(doc.offsets[num], 0, inUse))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

// TestRecoverGridDamagedRow locks the grid reader: a classic table whose rows
// sit on the specification's 20-byte grid is usable when only one field is
// damaged, and the damaged row falls back to the object header. A table whose
// rows do not line up keeps its refusal, which recoverDamagedTable pins.
func TestRecoverGridDamagedRow(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, gridDamagedRow(t))
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(lineMarks)) {
		t.Fatalf("content %q", got)
	}
}

// gridDamagedRow writes a normal table, then breaks one digit of the content
// object's offset without moving any row off its 20-byte slot.
func gridDamagedRow(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	contentNum := doc.object(streamBody("", []byte(lineMarks)))
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefLine(0, freeGen, false))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefLine(doc.offsets[num], 0, true))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	src := doc.buf.Bytes()
	row := bytes.Index(src[xrefAt:], []byte(fmt.Sprintf("%010d", doc.offsets[contentNum])))
	if row < 0 {
		t.Fatal("no row for the content object")
	}
	src[xrefAt+row] = 'x'
	return src
}

// TestRecoverMissingEndobj locks the missing-endobj repair: an object whose
// value is complete and whose endobj is not there, in front of the next object
// header, is the shape Ghostscript reports as "Encountered 'obj' while
// expecting 'endobj'" and reads as complete.
func TestRecoverMissingEndobj(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, missingEndobjDoc(t))
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
	raw, ok := file.RawObject(1)
	if !ok || string(raw) != "<< /Type /Catalog /Pages 2 0 R >>" {
		t.Fatalf("RawObject(1) = %q ok %v, want the catalog body without junk", raw, ok)
	}
}

// missingEndobjDoc writes object 1 without its endobj; the next object header
// follows immediately.
func missingEndobjDoc(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	catalogAt := body.Len()
	body.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\n")
	pagesAt := body.Len()
	body.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	pageAt := body.Len()
	body.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>\nendobj\n")
	contentAt := body.Len()
	body.WriteString("4 0 obj\n" + streamBody("", []byte(lineMarks)) + "\nendobj\n")
	xrefAt := body.Len()
	body.WriteString("xref\n0 5\n")
	body.WriteString(xrefLine(0, freeGen, false))
	body.WriteString(xrefLine(catalogAt, 0, true))
	body.WriteString(xrefLine(pagesAt, 0, true))
	body.WriteString(xrefLine(pageAt, 0, true))
	body.WriteString(xrefLine(contentAt, 0, true))
	fmt.Fprintf(&body, "trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefAt)
	return body.Bytes()
}

// TestRecoverPrevLoopKeepsRefusal locks the boundary of the grid recovery: the
// section at the startxref reads, so the failure is in its /Prev chain, and a
// chain that loops is not a damaged table. Only a section that itself fails to
// read may fall back to the grid reader.
func TestRecoverPrevLoopKeepsRefusal(t *testing.T) {
	t.Parallel()
	_, err := Open(t.Context(), prevLoopDoc(t))
	wantJob(t, err, opXRef, errSyntax)
}

// prevLoopDoc writes three fixed-width classic sections that chain in a cycle.
func prevLoopDoc(t *testing.T) []byte {
	t.Helper()
	base := len("%PDF-1.4\n")
	width := len(validationPrevSection(0))
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	body.Write(validationPrevSection(base + 2*width))
	body.Write(validationPrevSection(base))
	body.Write(validationPrevSection(base + width))
	fmt.Fprintf(&body, "startxref\n%d\n%%%%EOF\n", base)
	return body.Bytes()
}

// TestRecoverDamagedTrailerKeyword locks lastRootDict. A producer that damages
// the `trailer` keyword still wrote the dictionary, and a document is defined
// by its root rather than by the punctuation in front of it, so the header
// rebuild reads the dictionary it finds. GHOSTSCRIPT-687796-0.pdf is this
// shape in the corpus.
func TestRecoverDamagedTrailerKeyword(t *testing.T) {
	t.Parallel()
	src := bytes.Replace(classicLine(t), []byte("trailer"), []byte("trailes"), 1)
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
}

// TestRecoverNoRootDictionaryStaysRefused locks the boundary of lastRootDict: a
// file with no dictionary carrying /Root anywhere is still refused, so the
// fallback is not a blanket acceptance of any file with objects in it.
func TestRecoverNoRootDictionaryStaysRefused(t *testing.T) {
	t.Parallel()
	src := bytes.Replace(classicLine(t), []byte("/Root 1 0 R"), []byte("/NotRoot 1 0 R"), 1)
	if _, err := Open(t.Context(), src); err == nil {
		t.Fatal("a file with no /Root anywhere opened")
	}
}

// TestRecoverNearTable locks recoverNearTable. Two corpus files name a
// startxref offset that lands inside their own table rather than at its
// keyword, 23 and 55 bytes past it, which is a producer counting from a
// different base rather than a damaged file.
func TestRecoverNearTable(t *testing.T) {
	t.Parallel()
	full := classicLine(t)
	keyword := bytes.LastIndex(full, []byte(wordXRef))
	if keyword < 0 {
		t.Fatal("fixture has no xref keyword")
	}
	for _, delta := range []int{1, 23, 55, nearTableWindow - 1} {
		src := replaceStartxref(full, keyword+delta)
		file := mustOpen(t, src)
		if file.PageCount() != 1 {
			t.Fatalf("delta %d: pages %d", delta, file.PageCount())
		}
	}
}

// TestRecoverNearTableWindowIsBounded locks the other side of the window: an
// offset further back than nearTableWindow does not reach a table that sits
// outside it, so the search cannot wander to an unrelated table.
func TestRecoverNearTableWindowIsBounded(t *testing.T) {
	t.Parallel()
	full := classicLine(t)
	keyword := bytes.LastIndex(full, []byte(wordXRef))
	src := replaceStartxref(full, keyword-nearTableWindow-1)
	if _, _, ok := recoverNearTable(src, keyword-nearTableWindow-1); ok {
		t.Fatal("recoverNearTable reached a keyword outside its window")
	}
}
