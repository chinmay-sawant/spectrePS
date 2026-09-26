package pdf

import (
	"math"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// The phase 6 validation tests lock the font loading and no-outline policy in
// documentation/fonts.md: a program Spectre can parse paints, a font it
// cannot outline still loads its advances and Unicode, and only painting
// reports invalidfont. Every fixture is the synthetic sfnt or Type 1 program
// the package tests build, so no third-party font bytes are checked in.

// TestValidationFontFile3 proves that the synthetic sfnt wrapped as
// /FontFile3 /Subtype /OpenType maps a code, reports an advance, and paints
// pixels, the same as a /FontFile2 program.
func TestValidationFontFile3(t *testing.T) {
	t.Parallel()
	checkOpenTypeMap(t)
	checkOpenTypeFallbackAdvance(t)
	checkOpenTypePaint(t)
}

// openTypePage builds a one-page PDF whose /F1 reads its program from
// /FontFile3 with /Subtype /OpenType. The widths string is the optional
// /Widths and /FirstChar pair.
func openTypePage(t *testing.T, content, widths string) (*File, Resources) {
	t.Helper()
	fontDict := "<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FontDescriptor 6 0 R" +
		widths + " >>"
	descriptor := "<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile3 7 0 R >>"
	return fontPage(t, content, "<< /Font << /F1 5 0 R >> >>",
		fontDict, descriptor, synthFontStream(t, "/Subtype /OpenType"))
}

func checkOpenTypeMap(t *testing.T) {
	t.Helper()
	_, res := openTypePage(t, "", "/FirstChar 65 /Widths [611 389]")
	fnt := res.Fonts["F1"]
	if fnt.program == nil {
		t.Fatal("the /FontFile3 /OpenType program did not parse")
	}
	if !fnt.paintSource() {
		t.Fatal("an OpenType /FontFile3 font is not a paint source")
	}
	if gid, ok := fnt.glyphIndex('A'); !ok || gid != synthGlyphA {
		t.Fatalf("glyphIndex(A) = %d, %v, want %d", gid, ok, synthGlyphA)
	}
	if gid, ok := fnt.glyphIndex('B'); !ok || gid != synthGlyphB {
		t.Fatalf("glyphIndex(B) = %d, %v, want %d", gid, ok, synthGlyphB)
	}
	if got := fnt.Width('A'); got != 611 {
		t.Fatalf("A width = %v, want the /Widths entry 611", got)
	}
}

func checkOpenTypeFallbackAdvance(t *testing.T) {
	t.Helper()
	_, res := openTypePage(t, "", "")
	fnt := res.Fonts["F1"]
	if got := fnt.Width('A'); math.Abs(got-synthAdvanceA) > 0.5 {
		t.Fatalf("A width = %v, want the program advance %d", got, synthAdvanceA)
	}
	if got := fnt.Width('B'); math.Abs(got-synthAdvanceB) > 0.5 {
		t.Fatalf("B width = %v, want the program advance %d", got, synthAdvanceB)
	}
}

func checkOpenTypePaint(t *testing.T) {
	t.Helper()
	file, _ := openTypePage(t, "BT /F1 12 Tf 1 1 Td (A) Tj ET", "/FirstChar 65 /Widths [600]")
	pixmap := graphics.NewPixmap(20, 20)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); !ok {
		t.Fatal("the /FontFile3 /OpenType glyph painted nothing")
	}
}

// TestValidationNoOutlinePolicy proves that a Type 1 /FontFile paints, that a
// bare CFF stream, a non-Identity Type0 font, and a Type 3 font load and
// record the advance and Unicode while painting reports invalidfont, and that
// an Identity-H CIDFontType0 descendant still loads through an indirect
// /DescendantFonts array and an indirect /W array.
func TestValidationNoOutlinePolicy(t *testing.T) {
	t.Parallel()
	checkType1ProgramPaints(t)
	checkBareCFFPolicy(t)
	checkNonIdentityType0Policy(t)
	checkCIDFontType0Policy(t)
	checkType3Policy(t)
}

func checkType1ProgramPaints(t *testing.T) {
	t.Helper()
	file, _ := type1Page(t, "BT /F1 12 Tf 1 1 Td (A) Tj ET", type1PageOptions{
		Flags:  32,
		Widths: "/FirstChar 65 /Widths [600]",
	})
	pixmap := graphics.NewPixmap(20, 20)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatalf("a Type 1 /FontFile did not paint: %v", err)
	}
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); !ok {
		t.Fatal("the Type 1 glyph painted nothing")
	}
}

// cffStreamPage wraps the synthetic sfnt in a /FontFile3 stream whose subtype
// is not /OpenType. The program bytes are parseable, so the subtype alone
// keeps the font from painting.
func cffStreamPage(t *testing.T, subtype string) (*File, Resources) {
	t.Helper()
	return fontPage(t, "BT /F1 12 Tf 10 20 Td (A) Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FirstChar 65 /Widths [321] "+
			"/Encoding /WinAnsiEncoding /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile3 7 0 R >>",
		synthFontStream(t, "/Subtype /"+subtype),
	)
}

func checkBareCFFPolicy(t *testing.T) {
	t.Helper()
	file, res := cffStreamPage(t, "Type1C")
	fnt := res.Fonts["F1"]
	if fnt.program != nil {
		t.Fatal("a /FontFile3 /Type1C stream parsed as an outline program")
	}
	if fnt.paintSource() {
		t.Fatal("a bare CFF font is a paint source")
	}
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
	img := shown(t, pixmap)
	if !rowWhite(img, 0) || !rowWhite(img, img.Height-1) {
		t.Fatal("a bare CFF glyph painted pixels")
	}
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 'A', "A", 321.0/1000*12, 10, 20)
}

// type0PolicyPage builds an Identity-H font with a ToUnicode map. identity
// selects /Identity-H or /Identity-V for the outer encoding.
func type0PolicyPage(t *testing.T, identity, descendant string, bodies ...string) (*File, Resources) {
	t.Helper()
	return fontPage(t, "BT /F1 12 Tf 0 0 Td <00010002> Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		append([]string{
			"<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /" + identity +
				" /DescendantFonts [" + descendant + "] /ToUnicode 9 0 R >>",
		}, bodies...)...,
	)
}

const policyToUnicode = "/CIDInit /ProcSet findresource begin\n" +
	"12 dict begin\nbegincmap\n/CMapType 2 def\n" +
	"1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n" +
	"2 beginbfchar\n<0001> <0058>\n<0002> <0059>\nendbfchar\n" +
	"endcmap\nend\nend\n"

func checkNonIdentityType0Policy(t *testing.T) {
	t.Helper()
	file, res := type0PolicyPage(t, "Identity-V", "6 0 R",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth /DW 1000 /W [1 [750]] "+
			"/FontDescriptor 7 0 R /CIDToGIDMap /Identity >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 8 0 R >>",
		synthFontStream(t, ""),
		streamBody("", []byte(policyToUnicode)),
	)
	fnt := res.Fonts["F1"]
	if fnt.program == nil || !fnt.twoByteCodes() || fnt.identity {
		t.Fatalf("F1 = program %v, twoByte %v, identity %v", fnt.program != nil, fnt.twoByteCodes(), fnt.identity)
	}
	if fnt.paintSource() {
		t.Fatal("a non-Identity Type0 font is a paint source")
	}
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); ok {
		t.Fatal("a non-Identity Type0 glyph painted pixels")
	}
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 1, "X", 750.0/1000*12, 0, 0)
	checkGlyph(t, log, 1, 2, "Y", 1000.0/1000*12, 750.0/1000*12, 0)
}

// checkCIDFontType0Policy is the Embedded_font.pdf shape: an Identity-H font
// whose /DescendantFonts entry and /W entry are indirect, and whose
// descendant is a CIDFontType0 that carries an /OpenType program. The font
// loads its advance and Unicode and reports invalidfont only when it paints.
func checkCIDFontType0Policy(t *testing.T) {
	t.Helper()
	toUnicode := "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\nbegincmap\n/CMapType 2 def\n" +
		"1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n" +
		"1 beginbfchar\n<A911> <9664>\nendbfchar\n" +
		"endcmap\nend\nend\n"
	file, res := fontPage(t, "BT /F1 12 Tf 0 0 Td <A911> Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /Noto /Encoding /Identity-H "+
			"/DescendantFonts 6 0 R /ToUnicode 9 0 R >>",
		"[ 7 0 R ]",
		"<< /Type /Font /Subtype /CIDFontType0 /BaseFont /Noto /W 8 0 R /FontDescriptor 10 0 R >>",
		"[ 43281 [1000] ]",
		streamBody("", []byte(toUnicode)),
		"<< /Type /FontDescriptor /FontName /Noto /Flags 32 /FontFile3 11 0 R >>",
		synthFontStream(t, "/Subtype /OpenType"),
	)
	fnt := res.Fonts["F1"]
	if !fnt.identity || !fnt.twoByteCodes() {
		t.Fatalf("F1 = identity %v, twoByte %v", fnt.identity, fnt.twoByteCodes())
	}
	if got := fnt.Width(0xA911); got != 1000 {
		t.Fatalf("CID 43281 width = %v, want the indirect /W entry 1000", got)
	}
	if got, ok := fnt.Unicode(0xA911); !ok || got != "\u9664" {
		t.Fatalf("Unicode(43281) = %q, %v, want the ToUnicode result", got, ok)
	}
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); ok {
		t.Fatal("a CIDFontType0 glyph painted pixels")
	}
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 0xA911, "\u9664", 12, 0, 0)
}

func checkType3Policy(t *testing.T) {
	t.Helper()
	file, res := fontPage(t, "BT /F1 12 Tf 10 20 Td (ab) Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type3 /BaseFont /SandT /FontBBox [0 0 750 750] "+
			"/FontMatrix [0.001 0 0 0.001 0 0] /FirstChar 97 /LastChar 98 /Widths [1000 500] "+
			"/Encoding << /Differences [97 /square /triangle] >> /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /SandT /Flags 4 >>",
	)
	fnt := res.Fonts["F1"]
	if fnt.paintSource() || fnt.program != nil || fnt.type1 != nil {
		t.Fatal("a Type 3 font is a paint source")
	}
	if got := fnt.Width(97); got != 1000 {
		t.Fatalf("Type 3 code 97 width = %v, want 1000", got)
	}
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); ok {
		t.Fatal("a Type 3 glyph painted pixels")
	}
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, log, 0, 97, "", 12, 10, 20)
	checkGlyph(t, log, 1, 98, "", 6, 22, 20)
	text, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "ab\r\n" {
		t.Fatalf("ExtractText = %q, want %q", text, "ab\r\n")
	}
}

// TestValidationTextOpErrors locks the text operator error shapes: an unknown
// Tf resource name, a TJ array with an inline dictionary, a Type0 show with a
// trailing odd byte, and a glyph mask over the pixel cap.
func TestValidationTextOpErrors(t *testing.T) {
	t.Parallel()
	checkTfUnknownName(t)
	checkTJInlineDict(t)
	checkType0OddByte(t)
	checkGlyphPixelCap(t)
}

func textErrorPage(t *testing.T, content string) (*File, Resources) {
	t.Helper()
	return fontPage(t, content, "<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
}

func checkTfUnknownName(t *testing.T) {
	t.Helper()
	file, _ := textErrorPage(t, "BT /Nope 12 Tf 10 20 Td (A) Tj ET")
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opTextFont, errUndefined)
}

func checkTJInlineDict(t *testing.T) {
	t.Helper()
	file, _ := textErrorPage(t, "BT /F1 12 Tf 10 20 Td [(A) << /K 1 >>] TJ ET")
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShowArray, errType)
}

func checkType0OddByte(t *testing.T) {
	t.Helper()
	file, res := fontPage(t, "BT /F1 12 Tf 0 0 Td <0001FF> Tj ET",
		"<< /Font << /F1 5 0 R >> >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H "+
			"/DescendantFonts [6 0 R] /ToUnicode 7 0 R >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth /DW 1000 /W [1 [750]] >>",
		streamBody("", []byte(policyToUnicode)),
	)
	log := &glyphLog{}
	if err := paintWithSink(t, file, res, nil, log); err != nil {
		t.Fatal(err)
	}
	if len(log.glyphs) != 1 {
		t.Fatalf("glyphs = %d, want 1 with the trailing odd byte dropped", len(log.glyphs))
	}
	checkGlyph(t, log, 0, 1, "X", 750.0/1000*12, 0, 0)
	text, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "X\r\n" {
		t.Fatalf("ExtractText = %q, want %q", text, "X\r\n")
	}
}

// checkGlyphPixelCap proves a text size whose mask crosses the 20,000 side cap
// returns limitcheck from the show operator and paints nothing, while
// extraction still reads the text because it needs no outline.
func checkGlyphPixelCap(t *testing.T) {
	t.Helper()
	file := synthTextPage(t, "BT /F1 40000 Tf 0 0 Td (A) Tj ET")
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errLimit)
	if _, _, _, _, ok := markedBounds(shown(t, pixmap)); ok {
		t.Fatal("a glyph over the pixel cap painted pixels")
	}
	text, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "A\r\n" {
		t.Fatalf("ExtractText = %q, want %q", text, "A\r\n")
	}
}
