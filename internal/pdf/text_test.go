package pdf

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/font"
	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// metricsFont is a font value with standard 14 metrics, StandardEncoding, and
// no outline source.
func metricsFont(t *testing.T, name string) *Font {
	t.Helper()
	metrics, ok := font.Standard14(name)
	if !ok {
		t.Fatalf("Standard14(%q) missing", name)
	}
	return &Font{metrics: metrics, encoding: font.EncodingStandard.GlyphNames()}
}

func textRunner(t *testing.T) *runner {
	t.Helper()
	run := newRunner(nil, 1)
	run.fonts = map[string]*Font{
		"F1": metricsFont(t, "Helvetica"),
		"F2": metricsFont(t, "Courier"),
	}
	return run
}

func playContent(t *testing.T, run *runner, content string) {
	t.Helper()
	lex := scanner{src: []byte(content), pos: 0}
	for {
		tok, ok, err := lex.next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return
		}
		if err := run.take(tok); err != nil {
			t.Fatal(err)
		}
	}
}

// TestTextState proves the text state operators and the q/Q save.
func TestTextState(t *testing.T) {
	t.Parallel()
	checkTextParameters(t)
	checkTextMoves(t)
	checkTextSaveRestore(t)
}

func checkTextParameters(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	playContent(t, run, "BT /F1 12 Tf 2 Tc 3 Tw 50 Tz 4 TL 5 Ts ET")
	if run.text.fontName != "F1" || run.text.font == nil {
		t.Fatalf("font = %q, %v", run.text.fontName, run.text.font)
	}
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"size", run.text.size, 12},
		{"charSpacing", run.text.charSpacing, 2},
		{"wordSpacing", run.text.wordSpacing, 3},
		{"hscale", run.text.hscale, 0.5},
		{"leading", run.text.leading, 4},
		{"rise", run.text.rise, 5},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Fatalf("%s = %v, want %v", testCase.name, testCase.got, testCase.want)
		}
	}
}

func checkTextMoves(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	playContent(t, run, "BT 10 20 Td")
	wantMove := graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 10, F: 20}
	if run.textLine != wantMove || run.textMatrix != wantMove {
		t.Fatalf("Td = %+v, %+v", run.textLine, run.textMatrix)
	}
	playContent(t, run, "BT 1 2 3 4 5 6 Tm")
	wantMatrix := graphics.Matrix{A: 1, B: 2, C: 3, D: 4, E: 5, F: 6}
	if run.textLine != wantMatrix || run.textMatrix != wantMatrix {
		t.Fatalf("Tm = %+v, %+v", run.textLine, run.textMatrix)
	}
	playContent(t, run, "BT 10 20 TD")
	if run.text.leading != -20 || run.textMatrix != wantMove {
		t.Fatalf("TD leading = %v, matrix = %+v", run.text.leading, run.textMatrix)
	}
	playContent(t, run, "BT 4 TL 10 20 Td T*")
	wantNext := graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 10, F: 16}
	if run.textMatrix != wantNext {
		t.Fatalf("T* = %+v, want %+v", run.textMatrix, wantNext)
	}
	playContent(t, run, "BT 5 5 Td ET BT")
	if run.textMatrix != graphics.Identity() || run.textLine != graphics.Identity() {
		t.Fatalf("BT did not reset the matrices: %+v", run.textMatrix)
	}
}

func checkTextSaveRestore(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	playContent(t, run, "BT /F1 12 Tf 2 Tc q /F2 10 Tf 5 Tc 3 Tw Q")
	if run.text.fontName != "F1" || run.text.size != 12 {
		t.Fatalf("saved font = %q %v", run.text.fontName, run.text.size)
	}
	if run.text.charSpacing != 2 || run.text.wordSpacing != 0 {
		t.Fatalf("saved spacing = %v, %v", run.text.charSpacing, run.text.wordSpacing)
	}
	// The text matrices are not part of the graphics state.
	playContent(t, run, "BT 0 0 Td q 10 5 Td Q")
	want := graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 10, F: 5}
	if run.textMatrix != want {
		t.Fatalf("Q restored the text matrix: %+v", run.textMatrix)
	}
}

// TestStandard14Paint proves the painting policy in documentation/fonts.md:
// a standard 14 font has metrics but no outline source, so painting returns
// invalidfont while advances and extraction still work.
func TestStandard14Paint(t *testing.T) {
	t.Parallel()
	file, res := fontPage(t, "BT /F1 12 Tf 10 20 Td (A) Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	)
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, "Tj", errInvalidFont)
	img := shown(t, pixmap)
	if !rowWhite(img, 0) || !rowWhite(img, img.Height-1) {
		t.Fatal("a rejected standard 14 glyph painted pixels")
	}
	log := &glyphLog{}
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	opt := PaintOptions{Resources: res, Text: TextOptions{Fonts: nil, Sink: log}}
	if err := PaintWith(t.Context(), content, nil, 1, opt); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 'A', "A", 667.0/1000*12, 10, 20)
}

// TestTJAdvance proves that TJ numbers and the spacing parameters position
// the next glyph.
func TestTJAdvance(t *testing.T) {
	t.Parallel()
	checkTJNumbers(t)
	checkTJSpacing(t)
	checkTJHScale(t)
}

func checkTJNumbers(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 10 Tf 0 0 Td [(A) -500 (A)] TJ ET")
	checkGlyph(t, log, 0, 'A', "A", 6.67, 0, 0)
	checkGlyph(t, log, 1, 'A', "A", 6.67, 11.67, 0)
}

func checkTJSpacing(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 10 Tf 1 Tc 2 Tw 0 0 Td (A A) Tj ET")
	checkGlyph(t, log, 0, 'A', "A", 7.67, 0, 0)
	checkGlyph(t, log, 1, ' ', " ", 5.78, 7.67, 0)
	checkGlyph(t, log, 2, 'A', "A", 7.67, 13.45, 0)
}

func checkTJHScale(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 10 Tf 50 Tz 0 0 Td (A) Tj (A) Tj ET")
	checkGlyph(t, log, 0, 'A', "A", 3.335, 0, 0)
	checkGlyph(t, log, 1, 'A', "A", 3.335, 3.335, 0)
}

// TestQuoteShow proves that ' and " move to the next line and that " sets the
// spacing first.
func TestQuoteShow(t *testing.T) {
	t.Parallel()
	checkQuoteMove(t)
	checkDQuote(t)
}

func checkQuoteMove(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 10 Tf 3 TL 0 0 Td (A) ' (B) Tj ET")
	checkGlyph(t, log, 0, 'A', "A", 6.67, 0, -3)
	checkGlyph(t, log, 1, 'B', "B", 6.67, 6.67, -3)
}

func checkDQuote(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 10 Tf 3 TL 0 0 Td 4 1 (A) \" ET")
	checkGlyph(t, log, 0, 'A', "A", 7.67, 0, -3)
	if run.text.wordSpacing != 4 || run.text.charSpacing != 1 {
		t.Fatalf("spacing = %v, %v", run.text.wordSpacing, run.text.charSpacing)
	}
}

// TestTextSink proves the sink records each positioned glyph with its code,
// Unicode, advance, and device bounding box, without an outline program.
func TestTextSink(t *testing.T) {
	t.Parallel()
	checkSinkGlyphs(t)
	checkSinkTranslate(t)
}

func checkSinkGlyphs(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "BT /F1 12 Tf 10 20 Td (A A) Tj ET")
	if len(log.glyphs) != 3 {
		t.Fatalf("glyphs = %d, want 3", len(log.glyphs))
	}
	checkGlyph(t, log, 0, 'A', "A", 667.0/1000*12, 10, 20)
	checkGlyphBox(t, log.glyphs[0].Box, 10, 17, 10+667.0/1000*12, 29)
	checkGlyph(t, log, 1, ' ', " ", 278.0/1000*12, 10+667.0/1000*12, 20)
	checkGlyph(t, log, 2, 'A', "A", 667.0/1000*12, 10+667.0/1000*12+278.0/1000*12, 20)
}

func checkSinkTranslate(t *testing.T) {
	t.Helper()
	run := textRunner(t)
	log := &glyphLog{}
	run.sink = log
	playContent(t, run, "q 1 0 0 1 5 7 cm BT /F1 12 Tf 0 0 Td (A) Tj ET Q")
	checkGlyph(t, log, 0, 'A', "A", 667.0/1000*12, 5, 7)
	checkGlyphBox(t, log.glyphs[0].Box, 5, 4, 5+667.0/1000*12, 16)
}

func checkGlyphBox(t *testing.T, box Box, minX, minY, maxX, maxY float64) {
	t.Helper()
	if !near(box.MinX, minX) || !near(box.MinY, minY) || !near(box.MaxX, maxX) || !near(box.MaxY, maxY) {
		t.Fatalf("box = %+v, want [%v %v %v %v]", box, minX, minY, maxX, maxY)
	}
}

// TestTjGlyphPixels paints one embedded glyph and compares the page with a
// checked-in PPM fixture. The translated page proves the rendering matrix.
func TestTjGlyphPixels(t *testing.T) {
	t.Parallel()
	base := paintTextPage(t, "BT /F1 12 Tf 1 1 Td (A) Tj ET")
	checkPPM(t, "../../sampledata/fixtures/text-tj.ppm", base)
	minCol, minRow, maxCol, maxRow, ok := markedBounds(base)
	if !ok {
		t.Fatal("the glyph painted nothing")
	}
	moved := paintTextPage(t, "BT /F1 12 Tf 5 1 Td (A) Tj ET")
	movedCol, movedRow, movedMax, movedMaxRow, ok := markedBounds(moved)
	if !ok {
		t.Fatal("the translated glyph painted nothing")
	}
	if movedCol != minCol+4 || movedRow != minRow || movedMax != maxCol+4 || movedMaxRow != maxRow {
		t.Fatalf("translated glyph box = %d..%d rows %d..%d, want %d..%d rows %d..%d",
			movedCol, movedMax, movedRow, movedMaxRow, minCol+4, maxCol+4, minRow, maxRow)
	}
}

func paintTextPage(t *testing.T, content string) graphics.Image {
	t.Helper()
	file := synthTextPage(t, content)
	pixmap := graphics.NewPixmap(20, 20)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	return shown(t, pixmap)
}

func markedBounds(img graphics.Image) (int, int, int, int, bool) {
	minCol, minRow := img.Width, img.Height
	maxCol, maxRow := -1, -1
	for row := range img.Height {
		for col := range img.Width {
			offset := row*img.Stride + col*rgbBytes
			if img.Pixels[offset] == whiteByte && img.Pixels[offset+1] == whiteByte && img.Pixels[offset+2] == whiteByte {
				continue
			}
			minCol = min(minCol, col)
			maxCol = max(maxCol, col)
			minRow = min(minRow, row)
			maxRow = max(maxRow, row)
		}
	}
	if maxCol < 0 {
		return 0, 0, 0, 0, false
	}
	return minCol, minRow, maxCol, maxRow, true
}

// checkPPM compares one page with a P6 fixture. UPDATE_FIXTURES=1 rewrites
// the fixture, which is how the checked-in file was locked.
func checkPPM(t *testing.T, path string, img graphics.Image) {
	t.Helper()
	want := ppmBytes(img)
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		if err := os.WriteFile(path, want, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v (run with UPDATE_FIXTURES=1 to write it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("pixels differ from %s", path)
	}
}

func ppmBytes(img graphics.Image) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "P6\n%d %d\n255\n", img.Width, img.Height)
	for row := range img.Height {
		start := row * img.Stride
		buf.Write(img.Pixels[start : start+img.Width*rgbBytes])
	}
	return buf.Bytes()
}
