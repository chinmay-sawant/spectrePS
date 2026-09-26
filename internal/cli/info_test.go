package cli

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestPDFInfoCLI prints the document summary, and maps a bad flag, a missing
// file, and wrong argument counts to exit 2, an unreadable path to exit 3,
// and a malformed PDF to exit 1.
func TestPDFInfoCLI(t *testing.T) {
	t.Parallel()
	path := writeTemp(t, "info.pdf", infoPDF(t))
	want(t, []string{"info", path}, 0, infoExpected, "")

	plain := writeTemp(t, "plain.pdf", onePagePDF(t, "1 2 add"))
	want(t, []string{"info", plain}, 0,
		"PDF version: 1.4\nPages: 1\nPage 1: 20 x 20\nTagged: false\nFonts: none\nImages: 0\n", "")

	wantCode(t, []string{"info"}, 2)
	wantCode(t, []string{"info", "-x", path}, 2)
	wantCode(t, []string{"info", path, path}, 2)
	missing := filepath.Join(t.TempDir(), "missing.pdf")
	wantCode(t, []string{"info", missing}, 2)
	wantCode(t, []string{"info", t.TempDir()}, 3)

	bad := writeTemp(t, "bad.pdf", []byte("%PDF-1.4\njunk\n"))
	want(t, []string{"info", bad}, 1, "", "Error: /syntaxerror in xref\n")
}

const infoExpected = "PDF version: 1.4\n" +
	"Pages: 1\n" +
	"Page 1: 20 x 20\n" +
	"Tagged: false\n" +
	"Fonts:\n" +
	"  Alpha embedded=false\n" +
	"  Beta embedded=true\n" +
	"Images: 1\n"

// infoPDF is a one-page file with an image and two fonts, one embedded.
func infoPDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 /MediaBox [0 0 20 20] >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
			"/Resources << /XObject << /Im0 5 0 R >> /Font << /F1 6 0 R /F2 9 0 R >> >> >>"),
		flateStream(t, "q Q"),
		infoStream("/Type /XObject /Subtype /Image /Width 1 /Height 1 "+
			"/ColorSpace /DeviceGray /BitsPerComponent 8", []byte{0x80}),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Beta /FontDescriptor 7 0 R >>"),
		[]byte("<< /Type /FontDescriptor /FontName /Beta /FontFile2 8 0 R >>"),
		infoStream("", []byte("program")),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Alpha >>"),
	}
	return classicXref(t, objects)
}

func infoStream(dict string, raw []byte) []byte {
	head := fmt.Sprintf("<< %s /Length %d >>\nstream\n", dict, len(raw))
	return []byte(head + string(raw) + "\nendstream")
}
