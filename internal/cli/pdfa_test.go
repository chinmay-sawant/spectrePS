package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRewritePDFA4CLI(t *testing.T) {
	checkRewritePDFA4CLI(t)
	checkRewritePDFA4CLIRefusal(t)
	checkRewritePDFA4CLIUsage(t)
}

func checkRewritePDFA4CLI(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	src := writeTemp(t, "a4.pdf", cliPDFAFixture(t, true, false))
	out := filepath.Join(dir, "a4-out.pdf")
	want(t, []string{"rewrite", "-pdfa", "4", "-o", out, src}, 0, "", "")
	payload := readPayload(t, out)
	if !bytes.HasPrefix(payload, []byte("%PDF-2.0\n%")) {
		t.Fatalf("header %q", payload[:min(20, len(payload))])
	}
	for _, needle := range [][]byte{
		[]byte("/GTS_PDFA1"),
		[]byte("/DestOutputProfile"),
		[]byte("<pdfaid:part>4</pdfaid:part>"),
		[]byte("<pdfaid:rev>2020</pdfaid:rev>"),
	} {
		if !bytes.Contains(payload, needle) {
			t.Fatalf("output is missing %q", needle)
		}
	}
	if got := openPDFBytes(t, payload).PageCount(); got != 1 {
		t.Fatalf("PageCount = %d, want 1", got)
	}

	fileSrc := writeTemp(t, "a4f.pdf", cliPDFAFixture(t, true, true))
	fileOut := filepath.Join(dir, "a4f-out.pdf")
	want(t, []string{"rewrite", "-pdfa", "4f", "-o", fileOut, fileSrc}, 0, "", "")
	filePayload := readPayload(t, fileOut)
	if !bytes.Contains(filePayload, []byte("<pdfaid:conformance>F</pdfaid:conformance>")) {
		t.Fatal("4f output is missing the F conformance letter")
	}
}

func checkRewritePDFA4CLIRefusal(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "plain.pdf", cliPDFAFixture(t, false, false))
	out := filepath.Join(t.TempDir(), "refused.pdf")
	want(t, []string{"rewrite", "-pdfa", "4", "-o", out, src}, 1, "", "Error: /font-not-embedded in PDFA\n")
	if _, err := os.Stat(out); err == nil {
		t.Fatal("refused rewrite wrote a file")
	}
}

func checkRewritePDFA4CLIUsage(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "usage.pdf", cliPDFAFixture(t, true, false))
	out := filepath.Join(t.TempDir(), "out.pdf")
	want(t, []string{"rewrite", "-pdfa", "5", "-o", out, src}, 2, "",
		"spectreps: -pdfa wants 4 or 4f, got \"5\"\n")
}

// cliPDFAFixture builds a one-page path fixture. embedFont adds a font file
// and embedFile adds a catalog /Names /EmbeddedFiles entry.
func cliPDFAFixture(t *testing.T, embedFont, embedFile bool) []byte {
	t.Helper()
	catalog := "<< /Type /Catalog /Pages 2 0 R >>"
	if embedFile {
		catalog = "<< /Type /Catalog /Pages 2 0 R /Names << /EmbeddedFiles << /Names [] >> >> >>"
	}
	font := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	if embedFont {
		font = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FontDescriptor 6 0 R >>"
	}
	objects := [][]byte{
		[]byte(catalog),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		plainStream(t, "0 0 m 10 0 l S"),
		[]byte(font),
	}
	if embedFont {
		objects = append(objects,
			[]byte("<< /Type /FontDescriptor /FontName /Helvetica /FontFile 7 0 R >>"),
			plainStream(t, "ABCD"))
	}
	return classicXref(t, objects)
}
