package pdf

import "testing"

// TestExtractLayout proves the line sort, the run merge, the inserted spaces,
// the code-point fallback, and the CRLF line ends.
func TestExtractLayout(t *testing.T) {
	t.Parallel()
	checkLayoutOrder(t)
	checkLayoutSpaces(t)
	checkLayoutBaseline(t)
	checkLayoutFallback(t)
	checkExtractPage(t)
}

// layoutSize is the glyph size the layout tests use.
const layoutSize = 12

// layoutGlyph builds one extraction glyph with the same box shape the text
// machine records.
func layoutGlyph(code uint32, text string, posX, posY, advance float64) Glyph {
	return Glyph{
		Code:    code,
		Unicode: text,
		Advance: advance,
		X:       posX,
		Y:       posY,
		Box: Box{
			MinX: posX,
			MinY: posY - boxDescent*layoutSize,
			MaxX: posX + advance,
			MaxY: posY + boxAscent*layoutSize,
		},
	}
}

func checkLayoutOrder(t *testing.T) {
	t.Helper()
	glyphs := []Glyph{
		layoutGlyph('W', "W", 10, 700, 6),
		layoutGlyph('H', "H", 10, 720, 6),
		layoutGlyph('i', "i", 16, 720, 3),
		layoutGlyph('o', "o", 16, 700, 6),
	}
	const want = "Hi\r\nWo\r\n"
	if got := layoutGlyphs(glyphs); got != want {
		t.Fatalf("layout = %q, want %q", got, want)
	}
}

func checkLayoutSpaces(t *testing.T) {
	t.Helper()
	glyphs := []Glyph{
		layoutGlyph('A', "A", 10, 700, 6),
		layoutGlyph('B', "B", 20, 700, 6),
		layoutGlyph('C', "C", 28, 700, 6),
	}
	const want = "A BC\r\n"
	if got := layoutGlyphs(glyphs); got != want {
		t.Fatalf("layout = %q, want %q", got, want)
	}
}

func checkLayoutBaseline(t *testing.T) {
	t.Helper()
	glyphs := []Glyph{
		layoutGlyph('B', "B", 16, 702, 6),
		layoutGlyph('A', "A", 10, 700, 6),
		layoutGlyph('C', "C", 10, 660, 6),
	}
	const want = "AB\r\nC\r\n"
	if got := layoutGlyphs(glyphs); got != want {
		t.Fatalf("layout = %q, want %q", got, want)
	}
}

func checkLayoutFallback(t *testing.T) {
	t.Helper()
	glyphs := []Glyph{
		layoutGlyph(0x41, "", 10, 700, 6),
		layoutGlyph(0x01, "", 16, 700, 6),
	}
	const want = "A\r\n"
	if got := layoutGlyphs(glyphs); got != want {
		t.Fatalf("layout = %q, want %q", got, want)
	}
}

// checkExtractPage runs the whole path: resources, text machine, sink, and
// layout, with a standard 14 font that has no outline program.
func checkExtractPage(t *testing.T) {
	t.Helper()
	file, _ := fontPage(t, "BT /F1 12 Tf 20 40 Td (Hello) Tj 0 -16 Td (World) Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	)
	got, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	const want = "Hello\r\nWorld\r\n"
	if got != want {
		t.Fatalf("ExtractText = %q, want %q", got, want)
	}
	if _, err := file.ExtractText(t.Context(), 1); err != nil {
		wantJobErr(t, err, opRaster, errRange)
	}
}
