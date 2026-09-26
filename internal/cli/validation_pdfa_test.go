package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestValidationPDFACLI locks the PDF/A claim at levels 1 through 5, the F
// conformance letter for 4f, and every refusal's exit 1 with no output file.
func TestValidationPDFACLI(t *testing.T) {
	t.Run("levels", checkValidationPDFALevels)
	t.Run("4f letter", checkValidationPDFA4FLetter)
	t.Run("refusals", checkValidationPDFARefusals)
}

// checkValidationPDFALevels proves -pdfa 4 succeeds at levels 1 through 5
// with the header claim and the page count held.
func checkValidationPDFALevels(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "a4.pdf", cliPDFAFixture(t, true, false))
	for level := 1; level <= 5; level++ {
		t.Run(fmt.Sprintf("level %d", level), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.pdf")
			args := []string{
				"rewrite", "-pdfa", "4", "-level", strconv.Itoa(level), "-o", out, src,
			}
			want(t, args, 0, "", "")
			payload := readPayload(t, out)
			if !bytes.HasPrefix(payload, []byte("%PDF-2.0\n%")) {
				t.Fatalf("level %d header %q", level, payload[:min(20, len(payload))])
			}
			if got := openPDFBytes(t, payload).PageCount(); got != 1 {
				t.Fatalf("level %d PageCount = %d, want 1", level, got)
			}
		})
	}
}

// checkValidationPDFA4FLetter proves the level 3 4f claim carries the F
// conformance letter.
func checkValidationPDFA4FLetter(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "a4f.pdf", cliPDFAFixture(t, true, true))
	out := filepath.Join(t.TempDir(), "out.pdf")
	args := []string{"rewrite", "-pdfa", "4f", "-level", "3", "-o", out, src}
	want(t, args, 0, "", "")
	payload := readPayload(t, out)
	if !bytes.Contains(payload, []byte("<pdfaid:conformance>F</pdfaid:conformance>")) {
		t.Fatal("4f output lacks the F conformance letter")
	}
	if got := openPDFBytes(t, payload).PageCount(); got != 1 {
		t.Fatalf("PageCount = %d, want 1", got)
	}
}

// validationPDFARefusalCase is one refusal fixture: the catalog and resources
// overrides, the extra object bodies, and the rule the rewrite must name.
type validationPDFARefusalCase struct {
	name      string
	pdfa      string
	catalog   string
	resources string
	extra     []string
	rule      string
}

// checkValidationPDFARefusals proves each of the nine rules exits 1 with the
// rule line and writes no file.
func checkValidationPDFARefusals(t *testing.T) {
	t.Helper()
	cases := validationPDFAFontAndStreamCases()
	cases = append(cases, validationPDFAColorAndCatalogCases()...)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			src := writeTemp(t, testCase.name+".pdf",
				validationPDFAFixture(t, testCase.catalog, testCase.resources, testCase.extra...))
			out := filepath.Join(t.TempDir(), "out.pdf")
			args := []string{"rewrite", "-pdfa", testCase.pdfa, "-o", out, src}
			want(t, args, 1, "", "Error: /"+testCase.rule+" in PDFA\n")
			if _, err := os.Stat(out); err == nil {
				t.Fatalf("refused rewrite wrote %s", out)
			}
		})
	}
}

// validationPDFAFontAndStreamCases returns the font and stream filter refusal
// fixtures.
func validationPDFAFontAndStreamCases() []validationPDFARefusalCase {
	return []validationPDFARefusalCase{
		{
			name:      "font-not-embedded",
			pdfa:      "4",
			resources: "/Font << /F1 5 0 R >>",
			extra:     []string{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
			rule:      "font-not-embedded",
		},
		{
			name: "lzwdecode",
			pdfa: "4",
			extra: []string{
				"<< /Filter /LZWDecode /Length 0 >>\nstream\n\nendstream",
			},
			rule: "lzwdecode",
		},
		{
			name: "filter-not-allowed",
			pdfa: "4",
			extra: []string{
				"<< /Filter /Crypt /Length 0 >>\nstream\n\nendstream",
			},
			rule: "filter-not-allowed",
		},
	}
}

// validationPDFAColorAndCatalogCases returns the color, object, and catalog
// refusal fixtures.
func validationPDFAColorAndCatalogCases() []validationPDFARefusalCase {
	return []validationPDFARefusalCase{
		{
			name:      "cmyk-without-profile",
			pdfa:      "4",
			resources: "/ColorSpace << /CS0 5 0 R >>",
			extra: []string{
				"[/Separation /Spot /DeviceCMYK 6 0 R]",
				"<< /FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [1 1 1 1] /N 1 >>",
			},
			rule: "cmyk-without-profile",
		},
		{
			name:      "alternates-not-allowed",
			pdfa:      "4",
			resources: "/XObject << /Im0 5 0 R >>",
			extra: []string{"<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
				"/BitsPerComponent 8 /ColorSpace /DeviceRGB /Alternates [] >>"},
			rule: "alternates-not-allowed",
		},
		{
			name:      "opi-not-allowed",
			pdfa:      "4",
			resources: "/XObject << /Im0 5 0 R >>",
			extra: []string{"<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
				"/BitsPerComponent 8 /ColorSpace /DeviceRGB /OPI << /Version 1.3 >> >>"},
			rule: "opi-not-allowed",
		},
		{
			name:      "blend-mode-not-allowed",
			pdfa:      "4",
			resources: "/ExtGState << /GS0 5 0 R >>",
			extra:     []string{"<< /Type /ExtGState /BM /Multiply >>"},
			rule:      "blend-mode-not-allowed",
		},
		{
			name: "embedded-files-need-4f",
			pdfa: "4",
			catalog: "<< /Type /Catalog /Pages 2 0 R " +
				"/Names << /EmbeddedFiles << /Names [] >> >> >>",
			rule: "embedded-files-need-4f",
		},
		{
			name: "4f-needs-embedded-files",
			pdfa: "4f",
			rule: "4f-needs-embedded-files",
		},
	}
}

// TestValidationValidateUA2 locks open decision 11: validate runs the UA-2
// machine checks after the pages paint for a tagged input, and never for an
// untagged one.
func TestValidationValidateUA2(t *testing.T) {
	tagged := writeTemp(t, "tagged.pdf", validationTaggedNoTreePDF(t))
	untagged := writeTemp(t, "plain.pdf", onePagePDF(t, "0 0 m 10 0 l S"))

	t.Run("tagged runs the check", func(t *testing.T) {
		want(t, []string{"validate", tagged}, 1, "", "Error: /ua2-structtree in PDFUA\n")
	})
	t.Run("untagged skips the check", func(t *testing.T) {
		want(t, []string{"validate", untagged}, 0, "", "")
	})
	t.Run("passing sample", func(t *testing.T) {
		sample := filepath.Join("..", "..", "sampledata", "pdfua2", "tagged-ua2.pdf")
		if _, err := os.Stat(sample); err != nil {
			t.Skipf("sample absent: %v", err)
		}
		want(t, []string{"validate", sample}, 0, "", "")
	})
}

// validationPDFAFixture builds a one-page synthetic PDF. The extra object
// bodies start at object 5.
func validationPDFAFixture(t *testing.T, catalog, resources string, extra ...string) []byte {
	t.Helper()
	if catalog == "" {
		catalog = "<< /Type /Catalog /Pages 2 0 R >>"
	}
	objects := [][]byte{
		[]byte(catalog),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << " + resources + " >> >>"),
		plainStream(t, "0 0 m 10 0 l S"),
	}
	for _, body := range extra {
		objects = append(objects, []byte(body))
	}
	return classicXref(t, objects)
}

// validationTaggedNoTreePDF claims tags with /MarkInfo /Marked true and no
// /StructTreeRoot: the page paints, and the UA-2 check refuses ua2-structtree.
func validationTaggedNoTreePDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>"),
		plainStream(t, "0 0 m 10 0 l S"),
	}
	return classicXref(t, objects)
}
