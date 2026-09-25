package spectreps_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestRewritePDFA4(t *testing.T) {
	checkRewritePDFA4Base(t)
	checkRewritePDFA4F(t)
	checkRewritePDFA4Refusals(t)
}

func checkRewritePDFA4Base(t *testing.T) {
	t.Helper()
	in := newInst(t)
	src := pdfaFixturePDF(t, true, false)
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{PDFA: spectreps.PDFA4})
	if err != nil {
		t.Fatal(err)
	}
	wantPDFA4Payload(t, out, false)
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.PageCount() != 1 {
		t.Fatalf("PageCount = %d, want 1", reopened.PageCount())
	}
}

func checkRewritePDFA4F(t *testing.T) {
	t.Helper()
	in := newInst(t)
	src := pdfaFixturePDF(t, true, true)
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{PDFA: spectreps.PDFA4F})
	if err != nil {
		t.Fatal(err)
	}
	wantPDFA4Payload(t, out, true)
}

func checkRewritePDFA4Refusals(t *testing.T) {
	t.Helper()
	in := newInst(t)
	cases := []struct {
		name      string
		embedFont bool
		embedFile bool
		mode      spectreps.PDFAMode
		rule      string
	}{
		{name: "font", embedFont: false, embedFile: false, mode: spectreps.PDFA4, rule: "font-not-embedded"},
		{name: "base-with-files", embedFont: true, embedFile: true,
			mode: spectreps.PDFA4, rule: "embedded-files-need-4f"},
		{name: "4f-without-files", embedFont: true, embedFile: false,
			mode: spectreps.PDFA4F, rule: "4f-needs-embedded-files"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := in.OpenPDF(t.Context(), pdfaFixturePDF(t, tt.embedFont, tt.embedFile))
			if err != nil {
				t.Fatal(err)
			}
			out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{PDFA: tt.mode})
			var job spectreps.JobError
			if !errors.As(err, &job) || job.Op != "PDFA" || job.Msg != tt.rule {
				t.Fatalf("RewritePDF() error = %v, want %s in PDFA", err, tt.rule)
			}
			if out != nil {
				t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
			}
		})
	}
}

func TestRewritePDFA4Stable(t *testing.T) {
	in := newInst(t)
	doc, err := in.OpenPDF(t.Context(), pdfaFixturePDF(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	opt := spectreps.RewriteOptions{PDFA: spectreps.PDFA4}
	first, err := in.RewritePDF(t.Context(), doc, opt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := in.RewritePDF(t.Context(), doc, opt)
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareFiles(first, second); !compared.Equal {
		t.Fatalf("CompareFiles = %+v", compared)
	}
	if bytes.Contains(first, []byte("CreationDate")) || bytes.Contains(first, []byte("ModDate")) {
		t.Fatal("output contains a date")
	}
	if bytes.Contains(first, []byte("xmp:MetadataDate")) {
		t.Fatal("output contains an XMP metadata date")
	}
}

// wantPDFA4Payload checks the claim metadata for one rewrite result.
func wantPDFA4Payload(t *testing.T, out []byte, conformance bool) {
	t.Helper()
	prefix := []byte("%PDF-2.0\n%")
	if !bytes.HasPrefix(out, prefix) {
		t.Fatalf("header %q", out[:min(20, len(out))])
	}
	marker := out[len(prefix):]
	if len(marker) < 5 {
		t.Fatal("binary marker is short")
	}
	for i := range 4 {
		if marker[i] <= 127 {
			t.Fatalf("marker byte %d = %d, want above 127", i, marker[i])
		}
	}
	needles := [][]byte{
		[]byte("/GTS_PDFA1"),
		[]byte("/DestOutputProfile"),
		[]byte("<pdfaid:part>4</pdfaid:part>"),
		[]byte("<pdfaid:rev>2020</pdfaid:rev>"),
	}
	for _, needle := range needles {
		if !bytes.Contains(out, needle) {
			t.Fatalf("output is missing %q", needle)
		}
	}
	if bytes.Contains(out, []byte("DestOutputProfileRef")) {
		t.Fatal("output carries /DestOutputProfileRef")
	}
	if bytes.Contains(out, []byte("/Encrypt")) {
		t.Fatal("output contains /Encrypt")
	}
	hasF := bytes.Contains(out, []byte("<pdfaid:conformance>F</pdfaid:conformance>"))
	if hasF != conformance {
		t.Fatalf("conformance letter present %v, want %v", hasF, conformance)
	}
}

// pdfaFixturePDF builds a one-page path fixture. embedFont adds a font file
// and embedFile adds a catalog /Names /EmbeddedFiles entry.
func pdfaFixturePDF(t *testing.T, embedFont, embedFile bool) []byte {
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
