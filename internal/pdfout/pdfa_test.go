package pdfout

import (
	"bytes"
	"testing"
)

const pdfaMarkerBytes = 4

func TestPDFA4Header(t *testing.T) {
	checkPDFAWriteHeader(t)
	checkPDFACopyHeader(t)
	checkClassicHeader(t)
}

func checkPDFAWriteHeader(t *testing.T) {
	t.Helper()
	out, err := WriteWithOptions(t.Context(), []Page{{Content: []byte(lineMarks)}}, WriteOptions{PDFA: true})
	if err != nil {
		t.Fatal(err)
	}
	wantPDFAHeader(t, out)
	wantPDFATrailer(t, out)
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
}

func checkPDFACopyHeader(t *testing.T) {
	t.Helper()
	src := mustOpenPDF(t, copyFixture(t))
	out := mustCopy(t, src, CopyOptions{PDFA: true})
	wantPDFAHeader(t, out)
	wantPDFATrailer(t, out)
	reopened := mustOpenPDF(t, out)
	if reopened.PageCount() != 1 {
		t.Fatalf("pages %d", reopened.PageCount())
	}
	checkCopiedValue(t, reopened, copyFontNum, "BaseFont", "Helvetica")
}

// checkClassicHeader keeps the classic file at PDF 1.4 with no marker.
func checkClassicHeader(t *testing.T) {
	t.Helper()
	out, err := WriteWithOptions(t.Context(), nil, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte(headerLine)) {
		t.Fatalf("header %q", headBytes(out))
	}
	if bytes.Contains(out, []byte("%\xE2")) {
		t.Fatal("classic file carries a binary marker")
	}
}

// wantPDFAHeader checks %PDF-2.0 and a four-byte binary marker above 127.
func wantPDFAHeader(t *testing.T, out []byte) {
	t.Helper()
	prefix := []byte("%PDF-2.0\n%")
	if !bytes.HasPrefix(out, prefix) {
		t.Fatalf("header %q", headBytes(out))
	}
	marker := out[len(prefix):]
	if len(marker) < pdfaMarkerBytes+1 {
		t.Fatal("binary marker is short")
	}
	for i := range pdfaMarkerBytes {
		if marker[i] <= 127 {
			t.Fatalf("marker byte %d = %d, want above 127", i, marker[i])
		}
	}
	if marker[pdfaMarkerBytes] != '\n' {
		t.Fatalf("marker byte %d = %d, want newline", pdfaMarkerBytes, marker[pdfaMarkerBytes])
	}
}

// wantPDFATrailer checks the trailer keeps /ID and writes no /Encrypt.
func wantPDFATrailer(t *testing.T, out []byte) {
	t.Helper()
	if !bytes.Contains(out, []byte("/ID [")) {
		t.Fatal("missing /ID [")
	}
	if bytes.Contains(out, []byte("/Encrypt")) {
		t.Fatal("file contains /Encrypt")
	}
}

func headBytes(out []byte) []byte {
	return out[:min(20, len(out))]
}
