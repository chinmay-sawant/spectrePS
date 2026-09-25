package pdfout

import (
	"bytes"
	"testing"
)

func TestWritePackedObjects(t *testing.T) {
	file := mustOpenPDF(t, copyFixture(t))
	opt := CopyOptions{PackObjects: true}
	out := mustCopy(t, file, opt)
	if !bytes.HasPrefix(out, []byte(packedHeaderLine)) {
		t.Fatal("bad header")
	}
	if bytes.Contains(out, []byte("xref\n0 ")) {
		t.Fatal("packed file has a classic xref table")
	}
	if !bytes.Contains(out, []byte("/Type /ObjStm")) || !bytes.Contains(out, []byte("/Type /XRef")) {
		t.Fatal("packed containers missing")
	}
	if !bytes.Contains(out, []byte("/Size 9")) || !bytes.Contains(out, []byte("/Root 1 0 R")) {
		t.Fatal("trailer size or root wrong")
	}
	if !bytes.Equal(out, mustCopy(t, file, opt)) {
		t.Fatal("two calls differ")
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
	if _, ok := reopened.RawObject(copyContentNum); !ok {
		t.Fatal("content stream is not a plain object")
	}
	if _, ok := reopened.RawObject(copyFontNum); ok {
		t.Fatal("font is not packed")
	}
	parts := make([][]byte, 0, copyAnnotNum)
	for num := 1; num <= copyAnnotNum; num++ {
		body, ok := file.RawObject(num)
		if !ok {
			t.Fatalf("source raw object %d missing", num)
		}
		parts = append(parts, body)
	}
	wantID(t, out, parts...)
	wantID(t, mustCopy(t, file, CopyOptions{}), parts...)
}
