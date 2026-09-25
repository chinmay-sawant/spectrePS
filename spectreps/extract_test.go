package spectreps_test

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestExtractTextGolden locks the two-line text and the code-point fallback.
// Text is the oracle here: text pixels never byte-match Ghostscript, because
// hinting and antialiasing differ.
func TestExtractTextGolden(t *testing.T) {
	in := newInst(t)
	checkExtractGolden(t, in, twoLinePDF(t), "Hello\r\nWorld\r\n")
	checkExtractGolden(t, in, symbolicPDF(t), "AB\r\n")
}

func checkExtractGolden(t *testing.T, in *spectreps.Instance, src []byte, want string) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := in.ExtractText(t.Context(), doc, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ExtractText = %q, want %q", got, want)
	}
	_, err = in.ExtractText(t.Context(), doc, 1)
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != "ExtractText" || job.Msg != rangecheckMsg {
		t.Fatalf("page 1 error = %v, want rangecheck in ExtractText", err)
	}
}

// twoLinePDF is one page with two Helvetica lines. The font has no /Widths,
// so the standard 14 metrics cover the advances.
func twoLinePDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, "BT /F1 12 Tf 20 40 Td (Hello) Tj 0 -16 Td (World) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
	return classicXref(t, objects)
}

// symbolicPDF is one page whose font has no /Encoding and no /ToUnicode, so
// the printable code points stand for themselves.
func symbolicPDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, "BT /F1 12 Tf 10 20 Td (AB) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Custom /FontDescriptor 6 0 R >>"),
		[]byte("<< /Type /FontDescriptor /FontName /Custom /Flags 4 >>"),
	}
	return classicXref(t, objects)
}
