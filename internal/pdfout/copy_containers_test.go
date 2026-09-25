package pdfout

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	containerObjStmNum = 5
	containerXRefNum   = 6
	containerSize      = 7
	containerFreeRows  = 2
)

// containerFixture builds a one-page PDF 1.5 file whose catalog and page tree
// live in a Flate object stream (object 5) and whose xref is a Flate xref
// stream (object 6). Objects 3 and 4 are plain, and objects 1 and 2 are
// compressed members of object 5.
func containerFixture(t *testing.T) []byte {
	t.Helper()
	catalog := "<< /Type /Catalog /Pages 2 0 R >>"
	pages := "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"
	second := len(catalog) + 1
	header := fmt.Sprintf("1 0 2 %d\n", second)
	plain := []byte(header + catalog + "\n" + pages)
	objStm := containerStream(
		fmt.Sprintf("/Type /ObjStm /N 2 /First %d /Filter /FlateDecode", len(header)),
		mustFlate(t, plain),
	)
	page := containerPageBody()
	content := containerContentBody()

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.5\n")
	offsets := make([]int, containerSize)
	put := func(num int, body string) {
		offsets[num] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", num, body)
	}
	put(3, page)
	put(4, content)
	put(containerObjStmNum, objStm)
	xrefAt := buf.Len()
	dict := fmt.Sprintf(
		"/Type /XRef /Size %d /Root 1 0 R /W [1 2 2] /Filter /FlateDecode",
		containerSize,
	)
	put(containerXRefNum, containerStream(dict, mustFlate(t, containerRows(offsets, xrefAt))))
	if offsets[containerXRefNum] != xrefAt {
		t.Fatalf("xref offset %d, want %d", offsets[containerXRefNum], xrefAt)
	}
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return buf.Bytes()
}

func containerStream(dict string, raw []byte) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(raw), raw)
}

func containerPageBody() string {
	return "<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>"
}

func containerContentBody() string {
	return string(streamBody([]byte(lineMarks), false))
}

// containerRows writes one type 1 or type 2 row per object, in order.
func containerRows(offsets []int, xrefAt int) []byte {
	var buf bytes.Buffer
	writeContainerRow(&buf, 0, 0, freeGen)
	writeContainerRow(&buf, 2, containerObjStmNum, 0)
	writeContainerRow(&buf, 2, containerObjStmNum, 1)
	writeContainerRow(&buf, 1, offsets[3], 0)
	writeContainerRow(&buf, 1, offsets[4], 0)
	writeContainerRow(&buf, 1, offsets[containerObjStmNum], 0)
	writeContainerRow(&buf, 1, xrefAt, 0)
	return buf.Bytes()
}

func writeContainerRow(buf *bytes.Buffer, kind, second, third int) {
	buf.WriteByte(byte(kind))
	buf.WriteByte(byte(second >> 8))
	buf.WriteByte(byte(second))
	buf.WriteByte(byte(third >> 8))
	buf.WriteByte(byte(third))
}

func TestCopySkipsContainers(t *testing.T) {
	src := containerFixture(t)
	file := mustOpenPDF(t, src)
	if file.PageCount() != 1 {
		t.Fatalf("source pages %d", file.PageCount())
	}
	first := mustCopy(t, file, CopyOptions{})
	if !bytes.Equal(first, mustCopy(t, file, CopyOptions{})) {
		t.Fatal("two calls differ")
	}
	checkContainerOutput(t, first)
	checkContainerRoundTrip(t, first)
}

// checkContainerOutput proves the containers are absent from the bytes and the
// digest, and that their object numbers became free rows.
func checkContainerOutput(t *testing.T, out []byte) {
	t.Helper()
	if bytes.Contains(out, []byte("/Type /ObjStm")) || bytes.Contains(out, []byte("/Type /XRef")) {
		t.Fatal("output copies a dead container")
	}
	if got := bytes.Count(out, []byte(xrefRow(0, 0, 'f'))); got != containerFreeRows {
		t.Fatalf("free rows %d, want %d", got, containerFreeRows)
	}
	if !bytes.Contains(out, []byte("/Size 7")) || !bytes.Contains(out, []byte("/Root 1 0 R")) {
		t.Fatal("trailer size or root wrong")
	}
	wantID(t, out,
		pdf.SerializeValue(containerCatalogValue()),
		pdf.SerializeValue(containerPagesValue()),
		[]byte(containerPageBody()),
		[]byte(containerContentBody()),
	)
}

func checkContainerRoundTrip(t *testing.T, out []byte) {
	t.Helper()
	reopened := mustOpenPDF(t, out)
	if reopened.PageCount() != 1 {
		t.Fatalf("output pages %d", reopened.PageCount())
	}
	content, err := reopened.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(lineMarks)) {
		t.Fatalf("content %q", content)
	}
}

func containerCatalogValue() pdf.Value {
	return pdf.DictVal(map[string]pdf.Value{
		"Type":  pdf.NameVal("Catalog"),
		"Pages": pdf.RefVal(2, 0),
	})
}

func containerPagesValue() pdf.Value {
	return pdf.DictVal(map[string]pdf.Value{
		"Type":  pdf.NameVal("Pages"),
		"Kids":  pdf.ArrayVal([]pdf.Value{pdf.RefVal(3, 0)}),
		"Count": pdf.IntVal(1),
	})
}
