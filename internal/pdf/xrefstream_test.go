package pdf

import (
	"bytes"
	"fmt"
	"testing"
)

const (
	streamPage    = 3
	streamContent = 4
	streamObjs    = 5
	streamXRef    = 6
	streamSize    = 7
	fieldLimit    = 0xFFFF
	rowKindFree   = 0
	rowKindPlain  = 1
	rowKindPacked = 2
	rowWidthSum   = 5
)

func TestXrefStream(t *testing.T) {
	src := buildXRefStream(t)
	assertXRefStream(t, src)
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

func buildXRefStream(t *testing.T) []byte {
	t.Helper()
	plain, first := objectStreamPlain()
	objStm := streamBody(
		fmt.Sprintf("/Type /ObjStm /N 2 /First %d /Filter /FlateDecode", first),
		flateRaw(t, plain),
	)
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>"
	content := streamBody("", []byte(lineMarks))

	doc := newDoc()
	doc.put(streamObjs, objStm)
	doc.put(streamPage, page)
	doc.put(streamContent, content)
	xrefAt := doc.buf.Len()
	rows := streamRows(doc.offsets, xrefAt)
	if len(rows)%rowWidthSum != 0 {
		t.Fatalf("row bytes %d", len(rows))
	}
	dict := fmt.Sprintf(
		"/Type /XRef /Size %d /Root 1 0 R /W [1 2 2] /Filter /FlateDecode",
		streamSize,
	)
	doc.put(streamXRef, streamBody(dict, flateRaw(t, rows)))
	if doc.offsets[streamXRef] != xrefAt {
		t.Fatalf("xref offset %d", doc.offsets[streamXRef])
	}
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

func assertXRefStream(t *testing.T, src []byte) {
	t.Helper()
	found := bytes.LastIndex(src, []byte("startxref"))
	if found < 0 {
		t.Fatal("startxref")
	}
	offset, _, ok := pdfInt(src, found+len("startxref"))
	if !ok || offset < 0 || offset >= len(src) || bytes.HasPrefix(src[offset:], []byte("xref")) {
		t.Fatalf("classic xref at %d", offset)
	}
}

func objectStreamPlain() ([]byte, int) {
	catalog := "<< /Type /Catalog /Pages 2 0 R >>"
	pages := "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"
	second := len(catalog) + 1
	header := fmt.Sprintf("1 0 2 %d\n", second)
	var buf bytes.Buffer
	buf.WriteString(header)
	buf.WriteString(catalog)
	buf.WriteByte('\n')
	buf.WriteString(pages)
	return buf.Bytes(), len(header)
}

func streamRows(offsets []int, xrefAt int) []byte {
	var buf bytes.Buffer
	writeRow(&buf, rowKindFree, 0, freeGen)
	writeRow(&buf, rowKindPacked, streamObjs, 0)
	writeRow(&buf, rowKindPacked, streamObjs, 1)
	writeRow(&buf, rowKindPlain, offsets[streamPage], 0)
	writeRow(&buf, rowKindPlain, offsets[streamContent], 0)
	writeRow(&buf, rowKindPlain, offsets[streamObjs], 0)
	writeRow(&buf, rowKindPlain, xrefAt, 0)
	return buf.Bytes()
}

func writeRow(buf *bytes.Buffer, kind, second, third int) {
	if second < 0 || second > fieldLimit || third < 0 || third > fieldLimit {
		panic("pdf: xref field")
	}
	buf.WriteByte(byte(kind))
	buf.WriteByte(byte(second >> 8))
	buf.WriteByte(byte(second))
	buf.WriteByte(byte(third >> 8))
	buf.WriteByte(byte(third))
}
