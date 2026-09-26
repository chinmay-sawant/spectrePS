package spectreps_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationPDFARules locks all nine PDF/A refusal rules through the
// public RewritePDF. Each refusal is a JobError with Op "PDFA", the rule in
// Msg, and no output bytes.
func TestValidationPDFARules(t *testing.T) {
	in := newInst(t)
	cases := []struct {
		name      string
		mode      spectreps.PDFAMode
		catalog   string
		resources string
		extra     []string
		rule      string
	}{
		{
			name:      "font-not-embedded",
			mode:      spectreps.PDFA4,
			resources: "/Font << /F1 5 0 R >>",
			extra:     []string{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
			rule:      "font-not-embedded",
		},
		{
			name: "lzwdecode",
			mode: spectreps.PDFA4,
			extra: []string{
				"<< /Filter /LZWDecode /Length 0 >>\nstream\n\nendstream",
			},
			rule: "lzwdecode",
		},
		{
			name: "filter-not-allowed",
			mode: spectreps.PDFA4,
			extra: []string{
				"<< /Filter /Crypt /Length 0 >>\nstream\n\nendstream",
			},
			rule: "filter-not-allowed",
		},
		{
			name:      "cmyk-without-profile",
			mode:      spectreps.PDFA4,
			resources: "/ColorSpace << /CS0 5 0 R >>",
			extra: []string{
				"[/Separation /Spot /DeviceCMYK 6 0 R]",
				"<< /FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [1 1 1 1] /N 1 >>",
			},
			rule: "cmyk-without-profile",
		},
		{
			name:      "alternates-not-allowed",
			mode:      spectreps.PDFA4,
			resources: "/XObject << /Im0 5 0 R >>",
			extra: []string{"<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
				"/BitsPerComponent 8 /ColorSpace /DeviceRGB /Alternates [] >>"},
			rule: "alternates-not-allowed",
		},
		{
			name:      "opi-not-allowed",
			mode:      spectreps.PDFA4,
			resources: "/XObject << /Im0 5 0 R >>",
			extra: []string{"<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
				"/BitsPerComponent 8 /ColorSpace /DeviceRGB /OPI << /Version 1.3 >> >>"},
			rule: "opi-not-allowed",
		},
		{
			name:      "blend-mode-not-allowed",
			mode:      spectreps.PDFA4,
			resources: "/ExtGState << /GS0 5 0 R >>",
			extra:     []string{"<< /Type /ExtGState /BM /Multiply >>"},
			rule:      "blend-mode-not-allowed",
		},
		{
			name: "embedded-files-need-4f",
			mode: spectreps.PDFA4,
			catalog: "<< /Type /Catalog /Pages 2 0 R " +
				"/Names << /EmbeddedFiles << /Names [] >> >> >>",
			rule: "embedded-files-need-4f",
		},
		{
			name: "4f-needs-embedded-files",
			mode: spectreps.PDFA4F,
			rule: "4f-needs-embedded-files",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			src := validationPDFAInput(t, testCase.catalog, testCase.resources, testCase.extra...)
			doc, err := in.OpenPDF(t.Context(), src)
			if err != nil {
				t.Fatal(err)
			}
			out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{PDFA: testCase.mode})
			var job spectreps.JobError
			if !errors.As(err, &job) || job.Op != "PDFA" || job.Msg != testCase.rule {
				t.Fatalf("RewritePDF() error = %v, want %s in PDFA", err, testCase.rule)
			}
			if out != nil {
				t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
			}
		})
	}
}

// TestValidationUA2Preserve proves levels 1 through 5 keep the structure tree
// of a tagged input through the public RewritePDF. The corpus sample also
// passes the UA-2 preflight after each rewrite, so the tree, the metadata,
// and every MCID survive.
func TestValidationUA2Preserve(t *testing.T) {
	in := newInst(t)
	samplePath := filepath.Join("..", "sampledata", "pdfua2", "tagged-ua2.pdf")
	sample, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatalf("read %s: %v", samplePath, err)
	}
	cases := []struct {
		name      string
		src       []byte
		preflight bool
	}{
		{name: "synthetic", src: taggedPDF(t)},
		{name: "sample", src: sample, preflight: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			doc, err := in.OpenPDF(t.Context(), testCase.src)
			if err != nil {
				t.Fatal(err)
			}
			if !doc.Tagged() {
				t.Fatal("Tagged() = false before the rewrite")
			}
			wantPages := doc.PageCount()
			for level := 1; level <= 5; level++ {
				out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: level})
				if err != nil {
					t.Fatalf("level %d: RewritePDF() = %v", level, err)
				}
				if !bytes.Contains(out, []byte("/StructTreeRoot")) {
					t.Fatalf("level %d dropped /StructTreeRoot", level)
				}
				reopened, err := in.OpenPDF(t.Context(), out)
				if err != nil {
					t.Fatalf("level %d: OpenPDF() = %v", level, err)
				}
				if !reopened.Tagged() {
					t.Fatalf("level %d: Tagged() = false", level)
				}
				if got := reopened.PageCount(); got != wantPages {
					t.Fatalf("level %d: PageCount = %d, want %d", level, got, wantPages)
				}
				if testCase.preflight {
					if err := in.PreflightUA2(t.Context(), reopened); err != nil {
						t.Fatalf("level %d: PreflightUA2() = %v", level, err)
					}
				}
			}
		})
	}
}

// validationPDFAInput builds a one-page synthetic PDF for the public PDF/A
// rules. The extra object bodies start at object 5.
func validationPDFAInput(t *testing.T, catalog, resources string, extra ...string) []byte {
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
