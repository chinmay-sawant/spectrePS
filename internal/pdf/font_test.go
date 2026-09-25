package pdf

import (
	"math"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"golang.org/x/image/math/fixed"
)

// glyphLog records the glyphs the text machine sends to the sink.
type glyphLog struct {
	glyphs []Glyph
}

func (log *glyphLog) Glyph(g Glyph) {
	log.glyphs = append(log.glyphs, g)
}

// fontPage is doPage plus the resolved page resources.
func fontPage(t *testing.T, content, resources string, bodies ...string) (*File, Resources) {
	t.Helper()
	file := doPage(t, content, resources, bodies...)
	res, err := file.PageResources(0)
	if err != nil {
		t.Fatal(err)
	}
	return file, res
}

// TestFontResource proves the /Font resource lookup by page and name and the
// TextOptions seam: a nil marker with a sink shows text without painting.
func TestFontResource(t *testing.T) {
	t.Parallel()
	const helveticaName = "Helvetica"
	file, res := fontPage(t, "BT /F1 12 Tf 10 20 Td (A) Tj ET",
		"<< /Font << /F1 5 0 R /F2 6 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Widths [667] /FirstChar 65 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>",
	)
	checkFontNames(t, file, res, helveticaName)
	checkFontSink(t, file, res)
	checkFontOverride(t, file, res)
}

func checkFontNames(t *testing.T, file *File, res Resources, helveticaName string) {
	t.Helper()
	fnt, ok := res.Font("F1")
	if !ok || fnt.baseFont != helveticaName {
		t.Fatalf("F1 = %+v, want %s", fnt, helveticaName)
	}
	if _, ok := res.Font("Nope"); ok {
		t.Fatal("Nope resolved")
	}
	pageFont, err := file.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	if pageFont.baseFont != helveticaName {
		t.Fatalf("PageFont = %q, want %s", pageFont.baseFont, helveticaName)
	}
	if _, err := file.PageFont(0, "Nope"); err != nil {
		wantJobErr(t, err, opTextFont, nameUndefined)
	}
	if _, err := file.PageFont(1, "F1"); err != nil {
		wantJobErr(t, err, opRaster, errRange)
	}
}

func checkFontSink(t *testing.T, file *File, res Resources) {
	t.Helper()
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 'A', "A", 667.0/1000*12, 10, 20)
}

func checkFontOverride(t *testing.T, file *File, res Resources) {
	t.Helper()
	courier, ok := res.Fonts["F2"]
	if !ok {
		t.Fatal("F2 is missing")
	}
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, map[string]*Font{"F1": courier}, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 'A', "A", 600.0/1000*12, 10, 20)
}

func paintWithSink(t *testing.T, file *File, res Resources, fonts map[string]*Font, sink GlyphSink) error {
	t.Helper()
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	opt := PaintOptions{Resources: res, Text: TextOptions{Fonts: fonts, Sink: sink}}
	return PaintWith(t.Context(), content, nil, 1, opt)
}

func checkGlyph(t *testing.T, log *glyphLog, index int, code uint32, unicode string, advance, posX, posY float64) {
	t.Helper()
	if index >= len(log.glyphs) {
		t.Fatalf("glyphs = %d, want at least %d", len(log.glyphs), index+1)
	}
	got := log.glyphs[index]
	if got.Code != code || got.Unicode != unicode {
		t.Fatalf("glyph = %+v, want code %d %q", got, code, unicode)
	}
	if !near(got.Advance, advance) || !near(got.X, posX) || !near(got.Y, posY) {
		t.Fatalf("glyph = %+v, want advance %v at %v,%v", got, advance, posX, posY)
	}
}

func near(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// TestSimpleFont proves the /Widths, /FirstChar, /MissingWidth,
// /FontDescriptor, /BaseFont standard-14 fallback, and /Encoding with
// /Differences.
func TestSimpleFont(t *testing.T) {
	t.Parallel()
	_, res := fontPage(t, "",
		"<< /Font << /F1 5 0 R /F2 6 0 R /F3 7 0 R /F4 8 0 R /F5 9 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FirstChar 65 /Widths [600 400] "+
			"/FontDescriptor 10 0 R "+
			"/Encoding << /BaseEncoding /WinAnsiEncoding /Differences [65 /Aacute /B 67 /bullet 68 /.notdef] >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /UnknownFont >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Symbol >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Custom /FontDescriptor 11 0 R >>",
		"<< /Type /FontDescriptor /FontName /Helvetica /Flags 32 /MissingWidth 300 >>",
		"<< /Type /FontDescriptor /FontName /Custom /Flags 4 >>",
	)
	checkWidthsArray(t, res.Fonts["F1"])
	checkDifferences(t, res.Fonts["F1"])
	checkStandardFallback(t, res.Fonts["F2"])
	checkUnknownBase(t, res.Fonts["F3"])
	checkSymbolBase(t, res.Fonts["F4"])
	checkSymbolicFlags(t, res.Fonts["F5"])
}

func checkWidthsArray(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.Width('A'); got != 600 {
		t.Fatalf("A width = %v, want 600", got)
	}
	if got := fnt.Width('B'); got != 400 {
		t.Fatalf("B width = %v, want 400", got)
	}
	if got := fnt.Width('C'); got != 300 {
		t.Fatalf("C width = %v, want missing width 300", got)
	}
	if fnt.missingWidth != 300 {
		t.Fatalf("missing width = %v, want 300", fnt.missingWidth)
	}
	if fnt.metrics == nil {
		t.Fatal("standard 14 metrics are not loaded")
	}
}

func checkDifferences(t *testing.T, fnt *Font) {
	t.Helper()
	cases := []struct {
		code byte
		name string
	}{
		{'A', "Aacute"},
		{'B', "B"},
		{'C', "bullet"},
		{'D', ""},
	}
	for _, testCase := range cases {
		if got := fnt.encoding[testCase.code]; got != testCase.name {
			t.Fatalf("code %d name = %q, want %q", testCase.code, got, testCase.name)
		}
	}
}

func checkStandardFallback(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.Width('A'); got != 667 {
		t.Fatalf("A width = %v, want the Helvetica metric 667", got)
	}
	if got := fnt.Width(0x80); got != 556 {
		t.Fatalf("Euro width = %v, want the Helvetica metric 556", got)
	}
	if got := fnt.encoding[0x80]; got != "Euro" {
		t.Fatalf("code 0x80 name = %q, want Euro", got)
	}
}

func checkUnknownBase(t *testing.T, fnt *Font) {
	t.Helper()
	if fnt.metrics != nil {
		t.Fatal("UnknownFont has standard 14 metrics")
	}
	if got := fnt.Width('A'); got != 0 {
		t.Fatalf("A width = %v, want 0", got)
	}
	if got := fnt.encoding['A']; got != "A" {
		t.Fatalf("default encoding A = %q, want A", got)
	}
}

func checkSymbolBase(t *testing.T, fnt *Font) {
	t.Helper()
	if fnt.metrics == nil {
		t.Fatal("Symbol has no metrics")
	}
	if got := fnt.encoding[0x20]; got != "" {
		t.Fatalf("Symbol code 0x20 = %q, want the empty built-in table", got)
	}
	if got := fnt.Width(0x20); got != 250 {
		t.Fatalf("Symbol space width = %v, want 250", got)
	}
}

func checkSymbolicFlags(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.encoding['A']; got != "" {
		t.Fatalf("symbolic code A = %q, want empty", got)
	}
	if got := fnt.Width('A'); got != 0 {
		t.Fatalf("symbolic A width = %v, want 0", got)
	}
}

// TestToUnicode proves bfchar and bfrange override the encoding.
func TestToUnicode(t *testing.T) {
	t.Parallel()
	cmap := "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\nbegincmap\n/CMapName /Test-UCS def\n/CMapType 2 def\n" +
		"1 begincodespacerange\n<00> <FF>\nendcodespacerange\n" +
		"2 beginbfchar\n<41> <03A9>\n<46> <00660069>\nendbfchar\n" +
		"2 beginbfrange\n<42> <43> <0042>\n<44> <45> [<0100> <0101>]\nendbfrange\n" +
		"endcmap\nend\nend\n"
	_, res := fontPage(t, "", "<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding /ToUnicode 6 0 R >>",
		streamBody("", []byte(cmap)),
	)
	fnt := res.Fonts["F1"]
	cases := []struct {
		code    uint32
		unicode string
		present bool
	}{
		{'A', "\u03A9", true},
		{'F', "fi", true},
		{'B', "B", true},
		{'C', "C", true},
		{'D', "\u0100", true},
		{'E', "\u0101", true},
		{'G', "G", true},
	}
	for _, testCase := range cases {
		got, ok := fnt.Unicode(testCase.code)
		if ok != testCase.present || got != testCase.unicode {
			t.Fatalf("Unicode(%#x) = %q, %v, want %q, %v", testCase.code, got, ok, testCase.unicode, testCase.present)
		}
	}
}

// TestTrueTypeGlyph proves that /FontFile2 parses through sfnt, that the
// character code maps to a glyph index, and that advances come from the
// /Widths array or the program.
func TestTrueTypeGlyph(t *testing.T) {
	t.Parallel()
	file, res := fontPage(t, "BT /F1 12 Tf 0 0 Td (AB) Tj ET",
		"<< /Font << /F1 5 0 R /F2 8 0 R >> >>",
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FirstChar 65 /Widths [611 389] "+
			"/FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
		synthFontStream(t, ""),
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FontDescriptor 9 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
	)
	checkGlyphIndexes(t, res.Fonts["F1"])
	checkWidthsWin(t, res.Fonts["F1"])
	checkProgramAdvance(t, res.Fonts["F2"])
	checkOutline(t, res.Fonts["F1"])
	checkTrueTypePaint(t, file)
}

func checkGlyphIndexes(t *testing.T, fnt *Font) {
	t.Helper()
	if fnt.program == nil {
		t.Fatal("no sfnt program")
	}
	cases := []struct {
		code uint32
		want int
	}{
		{'A', synthGlyphA},
		{'B', synthGlyphB},
	}
	for _, testCase := range cases {
		gid, ok := fnt.glyphIndex(testCase.code)
		if !ok || int(gid) != testCase.want {
			t.Fatalf("glyphIndex(%#x) = %d, %v, want %d", testCase.code, gid, ok, testCase.want)
		}
	}
}

func checkWidthsWin(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.Width('A'); got != 611 {
		t.Fatalf("A width = %v, want the /Widths entry 611", got)
	}
	if got := fnt.Width('B'); got != 389 {
		t.Fatalf("B width = %v, want the /Widths entry 389", got)
	}
	if got := fnt.Width('C'); got != 0 {
		t.Fatalf("C width = %v, want 0", got)
	}
}

func checkProgramAdvance(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.Width('A'); math.Abs(got-synthAdvanceA) > 0.5 {
		t.Fatalf("A width = %v, want the program advance %d", got, synthAdvanceA)
	}
	if got := fnt.Width('B'); math.Abs(got-synthAdvanceB) > 0.5 {
		t.Fatalf("B width = %v, want the program advance %d", got, synthAdvanceB)
	}
}

func checkOutline(t *testing.T, fnt *Font) {
	t.Helper()
	segments, ok := fnt.outline('A')
	if !ok || len(segments) == 0 {
		t.Fatalf("outline(A) = %d segments, %v", len(segments), ok)
	}
	bounds := segments.Bounds()
	cases := []struct {
		got  fixed.Int26_6
		want float64
	}{
		{bounds.Min.X, 100},
		{bounds.Max.X, 600},
		{bounds.Min.Y, -700},
		{bounds.Max.Y, 0},
	}
	for _, testCase := range cases {
		if math.Abs(fontUnits(testCase.got)-testCase.want) > 0.5 {
			t.Fatalf("outline bound = %v units, want %v", fontUnits(testCase.got), testCase.want)
		}
	}
}

// fontUnits converts one 26.6 coordinate at outlinePPEM to font units per
// 1000 units per em.
func fontUnits(value fixed.Int26_6) float64 {
	return float64(value) / 64 / outlinePPEM * 1000
}

func checkTrueTypePaint(t *testing.T, file *File) {
	t.Helper()
	pixmap := graphics.NewPixmap(20, 20)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	marked := 0
	for _, channel := range img.Pixels {
		if channel != whiteByte {
			marked++
		}
	}
	if marked == 0 {
		t.Fatal("the embedded glyph painted nothing")
	}
}

// TestIdentityHText proves that a Type0 Identity-H font maps two-byte codes
// through /CIDToGIDMap and reads /W and /DW.
func TestIdentityHText(t *testing.T) {
	t.Parallel()
	file, res := fontPage(t, "BT /F1 12 Tf 0 0 Td <00010002> Tj ET",
		"<< /Font << /F1 5 0 R /F2 9 0 R >> >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H /DescendantFonts [6 0 R] >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth /FontDescriptor 7 0 R /DW 1000 "+
			"/W [1 [500 600]] /CIDToGIDMap 8 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 10 0 R >>",
		streamBody("", []byte{0, 0, 0, 2, 0, 1}),
		"<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H /DescendantFonts [11 0 R] >>",
		synthFontStream(t, ""),
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth /FontDescriptor 12 0 R "+
			"/CIDToGIDMap /Identity >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 10 0 R >>",
	)
	checkIdentityFont(t, res.Fonts["F1"])
	checkIdentityMap(t, res.Fonts["F2"])
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 1, "", 6, 0, 0)
	checkGlyph(t, log, 1, 2, "", 7.2, 6, 0)
}

func checkIdentityFont(t *testing.T, fnt *Font) {
	t.Helper()
	if !fnt.twoByteCodes() || !fnt.identity {
		t.Fatal("F1 is not Identity-H")
	}
	checkIdentityGIDs(t, fnt)
	checkIdentityWidths(t, fnt)
	if _, ok := fnt.Unicode(1); ok {
		t.Fatal("F1 has a Unicode mapping without /ToUnicode")
	}
}

func checkIdentityGIDs(t *testing.T, fnt *Font) {
	t.Helper()
	if gid, ok := fnt.glyphIndex(1); !ok || gid != synthGlyphB {
		t.Fatalf("CID 1 = %d, %v, want glyph B", gid, ok)
	}
	if gid, ok := fnt.glyphIndex(2); !ok || gid != synthGlyphA {
		t.Fatalf("CID 2 = %d, %v, want glyph A", gid, ok)
	}
}

func checkIdentityWidths(t *testing.T, fnt *Font) {
	t.Helper()
	if got := fnt.Width(1); got != 500 {
		t.Fatalf("CID 1 width = %v, want 500", got)
	}
	if got := fnt.Width(2); got != 600 {
		t.Fatalf("CID 2 width = %v, want 600", got)
	}
	if got := fnt.Width(3); got != 1000 {
		t.Fatalf("CID 3 width = %v, want the /DW default 1000", got)
	}
}

func checkIdentityMap(t *testing.T, fnt *Font) {
	t.Helper()
	if gid, ok := fnt.glyphIndex(1); !ok || gid != synthGlyphA {
		t.Fatalf("identity CID 1 = %d, %v, want glyph A", gid, ok)
	}
	if got := fnt.Width(1); got != 1000 {
		t.Fatalf("identity CID 1 width = %v, want 1000", got)
	}
}
