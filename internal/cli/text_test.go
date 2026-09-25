package cli

import "testing"

// TestTextCommand extracts the two lines of a PDF and the code-point fallback
// of a symbolic font. The -pages range reuses the shared page grammar.
func TestTextCommand(t *testing.T) {
	twoLines := writeTemp(t, "two-lines.pdf", textPDF(t, twoLineTextObjects(t)))
	want(t, []string{"text", twoLines}, 0, "Hello\r\nWorld\r\n", "")
	want(t, []string{"text", "-pages", "1", twoLines}, 0, "Hello\r\nWorld\r\n", "")
	want(t, []string{"text", "-pages", "2", twoLines}, 1, "", "Error: /rangecheck in pages\n")
	symbolic := writeTemp(t, "symbolic.pdf", textPDF(t, symbolicTextObjects(t)))
	want(t, []string{"text", symbolic}, 0, "AB\r\n", "")
	wantCode(t, []string{"text"}, 2)
	wantCode(t, []string{"text", "-w", "20", twoLines}, 2)
}

func textPDF(t *testing.T, objects [][]byte) []byte {
	t.Helper()
	return classicXref(t, objects)
}

func twoLineTextObjects(t *testing.T) [][]byte {
	t.Helper()
	return [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, "BT /F1 12 Tf 20 40 Td (Hello) Tj 0 -16 Td (World) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
}

func symbolicTextObjects(t *testing.T) [][]byte {
	t.Helper()
	return [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, "BT /F1 12 Tf 10 20 Td (AB) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Custom /FontDescriptor 6 0 R >>"),
		[]byte("<< /Type /FontDescriptor /FontName /Custom /Flags 4 >>"),
	}
}
