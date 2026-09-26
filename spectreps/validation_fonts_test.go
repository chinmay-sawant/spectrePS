package spectreps_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// validationCorpusDir is the committed validation corpus, relative to the
// spectreps package directory. Tests skip when the tree is absent.
const validationCorpusDir = "../sampledata/validation"

// TestValidationExtractCases locks extraction through the public API:
// /ToUnicode overrides /Encoding /Differences, a gap wider than a quarter of
// the glyph box inserts a space, and lines and columns come out top to bottom
// and left to right even when the content shows them out of order. One
// synthetic page and three committed corpus PDFs carry the cases.
func TestValidationExtractCases(t *testing.T) {
	in := newInst(t)
	checkSyntheticExtract(t, in)
	checkCorpusExtract(t, in)
}

// syntheticExtractPDF is one page whose /F1 renames A through /Differences and
// overrides it through /ToUnicode, and whose content shows two columns and
// three gaps. The content order is right column first, so the expected text
// proves the layout sort.
//
// Codes: A is /Differences /Aacute and /ToUnicode U+03A9, B keeps WinAnsi,
// and C is /Differences /eacute. Helvetica metrics: A and B advance 667, C
// advances 556, and the text is 12 points, so the box is 12 points high and
// the quarter-box threshold is 3 points.
func syntheticExtractPDF(t *testing.T) []byte {
	t.Helper()
	content := "BT\n/F1 12 Tf\n" +
		"1 0 0 1 120 70 Tm (R1) Tj\n" +
		"1 0 0 1 120 50 Tm (R2) Tj\n" +
		"1 0 0 1 20 70 Tm (A) Tj\n" +
		"1 0 0 1 20 50 Tm (B) Tj\n" +
		"1 0 0 1 20 30 Tm (C) Tj\n" +
		"1 0 0 1 28 30 Tm (C) Tj\n" +
		"1 0 0 1 40 30 Tm (C) Tj\n" +
		"ET"
	toUnicode := "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\nbegincmap\n/CMapType 2 def\n" +
		"1 begincodespacerange\n<00> <FF>\nendcodespacerange\n" +
		"1 beginbfchar\n<41> <03A9>\nendbfchar\n" +
		"endcmap\nend\nend\n"
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, content),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica " +
			"/Encoding << /Type /Encoding /BaseEncoding /WinAnsiEncoding " +
			"/Differences [65 /Aacute 67 /eacute] >> /ToUnicode 6 0 R >>"),
		flateStream(t, toUnicode),
	}
	return classicXref(t, objects)
}

func checkSyntheticExtract(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), syntheticExtractPDF(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := in.ExtractText(t.Context(), doc, 0)
	if err != nil {
		t.Fatal(err)
	}
	const want = "\u03A9 R1\r\nB R2\r\n\u00E9\u00E9 \u00E9\r\n"
	if got != want {
		t.Fatalf("synthetic ExtractText = %q, want %q", got, want)
	}
}

// checkCorpusExtract reads the committed text corpus. Each row locks the real
// file: /Differences with /ToUnicode, an identity ToUnicode map, and the
// OpenType /FontFile3 CID font that must load even though painting refuses it.
func checkCorpusExtract(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	if _, err := os.Stat(filepath.FromSlash(validationCorpusDir)); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s is absent", validationCorpusDir)
	}
	cases := []struct {
		name string
		want string
	}{
		{name: "repo-tagged-text.pdf", want: "Hello, tagged text.\r\n"},
		{name: "IdentityToUnicodeMap_charCodeOf.pdf", want: "ABCdef\r\n"},
		{name: "Embedded_font.pdf", want: "\u9664\r\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(validationCorpusDir, "text", testCase.name)
			src, err := os.ReadFile(filepath.FromSlash(path))
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			doc, err := in.OpenPDF(t.Context(), src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := in.ExtractText(t.Context(), doc, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got != testCase.want {
				t.Fatalf("%s ExtractText = %q, want %q", testCase.name, got, testCase.want)
			}
		})
	}
}
