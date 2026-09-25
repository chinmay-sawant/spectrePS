package pdfout

import (
	"bytes"
	"testing"
)

// TestTaggedHeaderOption proves the tag switch writes the PDF 2.0 header with
// its binary marker for a PDF 1.x source, the default keeps the 1.4 header,
// and PDFA still wins over Tag.
func TestTaggedHeaderOption(t *testing.T) {
	t.Parallel()
	plain := mustOpenPDF(t, copyFixture(t))
	tagged := mustCopy(t, plain, CopyOptions{Tag: true})
	if !bytes.HasPrefix(tagged, []byte(pdfaHeaderLine)) {
		t.Fatalf("tag header = %q", headerText(tagged))
	}
	if !bytes.HasPrefix(mustCopy(t, plain, CopyOptions{}), []byte(headerLine)) {
		t.Fatal("default options lost the 1.4 header")
	}
	both := mustCopy(t, plain, CopyOptions{Tag: true, PDFA: true})
	if !bytes.HasPrefix(both, []byte(pdfaHeaderLine)) {
		t.Fatalf("combined header = %q", headerText(both))
	}
}
