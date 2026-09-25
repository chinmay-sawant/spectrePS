package pdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
)

// type1PageOptions selects the font dictionary and program one test page
// carries.
type type1PageOptions struct {
	Program      type1synth.Options
	Flags        int
	Widths       string
	Encoding     string
	NoLengths    bool
	Subtype      string
	ProgramKey   string
	BaseFont     string
	MissingWidth int
}

// type1Page builds a one-page PDF whose /F1 is the synthetic Type 1 font.
func type1Page(t *testing.T, content string, opt type1PageOptions) (*File, Resources) {
	t.Helper()
	if opt.Flags == 0 {
		opt.Flags = symbolicFontFlag
	}
	if opt.Subtype == "" {
		opt.Subtype = subtypeType1
	}
	if opt.BaseFont == "" {
		opt.BaseFont = type1synth.FontName
	}
	if opt.ProgramKey == "" {
		opt.ProgramKey = keyFontFile
	}
	data, lengths := type1synth.Program(opt.Program)
	dict := "/Filter /FlateDecode"
	if !opt.NoLengths {
		dict += fmt.Sprintf(" /Length1 %d /Length2 %d /Length3 %d",
			lengths[0], lengths[1], lengths[2])
	}
	font := fmt.Sprintf("<< /Type /Font /Subtype /%s /BaseFont /%s /FontDescriptor 6 0 R%s%s >>",
		opt.Subtype, opt.BaseFont, opt.Widths, opt.Encoding)
	descriptor := fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags %d /%s 7 0 R >>",
		type1synth.FontName, opt.Flags, opt.ProgramKey)
	if opt.MissingWidth > 0 {
		descriptor = fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags %d /MissingWidth %d /%s 7 0 R >>",
			type1synth.FontName, opt.Flags, opt.MissingWidth, opt.ProgramKey)
	}
	return fontPage(t, content, "<< /Font << /F1 5 0 R >> >>",
		font, descriptor, streamBody(dict, flateRaw(t, data)))
}

// type1Font resolves the /F1 resource of one built page.
func type1Font(t *testing.T, res Resources) *Font {
	t.Helper()
	fnt, ok := res.Font("F1")
	if !ok || fnt == nil {
		t.Fatal("F1 did not resolve")
	}
	return fnt
}

// TestType1Glyph proves the /FontFile load, the glyph lookup through the
// PDF and built-in encodings, the outline conversion, and the broken program
// path.
func TestType1Glyph(t *testing.T) {
	t.Parallel()
	checkType1Loaded(t)
	checkType1OutlineBounds(t)
	checkType1MissingName(t)
	checkType1BrokenProgram(t)
	checkType1MMType1(t)
}

func checkType1Loaded(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{})
	fnt := type1Font(t, res)
	if fnt.type1 == nil {
		t.Fatal("the Type 1 program did not load")
	}
	if !fnt.paintSource() {
		t.Fatal("a loaded Type 1 font is not a paint source")
	}
	if fnt.builtinEncoding[type1synth.CodeA] != type1synth.GlyphA {
		t.Fatalf("built-in encoding A = %q", fnt.builtinEncoding[type1synth.CodeA])
	}
}

func checkType1OutlineBounds(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{})
	segments, ok := type1Font(t, res).outline(type1synth.CodeA)
	if !ok || len(segments) == 0 {
		t.Fatalf("outline(A) = %d segments, %v", len(segments), ok)
	}
	bounds := segments.Bounds()
	cases := []struct {
		got  float64
		want float64
	}{
		{fontUnits(bounds.Min.X), type1synth.AMinX},
		{fontUnits(bounds.Max.X), type1synth.AMaxX},
		{fontUnits(bounds.Min.Y), -type1synth.AMaxY},
		{fontUnits(bounds.Max.Y), 0},
	}
	for _, testCase := range cases {
		if diff := testCase.got - testCase.want; diff > 0.5 || diff < -0.5 {
			t.Fatalf("outline bound = %v, want %v", testCase.got, testCase.want)
		}
	}
}

func checkType1MissingName(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{Flags: 32})
	fnt := type1Font(t, res)
	name := fnt.glyphName('Z')
	if name != "Z" {
		t.Fatalf("glyphName(Z) = %q, want the StandardEncoding name", name)
	}
	if _, ok := fnt.outline('Z'); ok {
		t.Fatal("outline(Z) resolved a name missing from CharStrings")
	}
}

func checkType1BrokenProgram(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{Program: type1synth.Options{LenIV: 4, Broken: true}})
	fnt := type1Font(t, res)
	if fnt.type1 != nil {
		t.Fatal("a corrupt program loaded a Type 1 font")
	}
	if fnt.paintSource() {
		t.Fatal("a corrupt program is a paint source")
	}
	if _, ok := fnt.outline(type1synth.CodeA); ok {
		t.Fatal("a corrupt program produced an outline")
	}
}

func checkType1MMType1(t *testing.T) {
	t.Helper()
	file, res := type1Page(t, "BT /F1 12 Tf 10 20 Td (A) Tj ET", type1PageOptions{
		Subtype: subtypeMMType1,
		Widths:  "/FirstChar 65 /Widths [321]",
	})
	fnt := type1Font(t, res)
	if fnt.type1 != nil {
		t.Fatal("MMType1 read an instance program")
	}
	if got := fnt.Width(type1synth.CodeA); got != 321 {
		t.Fatalf("MMType1 width = %v, want the /Widths entry 321", got)
	}
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
}

// TestType1Widths proves the width fallback: /Widths, then standard 14
// metrics, then the charstring hsbw, then /MissingWidth.
func TestType1Widths(t *testing.T) {
	t.Parallel()
	widths := "/FirstChar 65 /Widths [111]"
	_, res := type1Page(t, "", type1PageOptions{Widths: widths})
	if got := type1Font(t, res).Width(type1synth.CodeA); got != 111 {
		t.Fatalf("/Widths width = %v, want 111", got)
	}
	_, res = type1Page(t, "", type1PageOptions{
		Flags:    32,
		BaseFont: "Helvetica",
	})
	if got := type1Font(t, res).Width(type1synth.CodeA); got != 667 {
		t.Fatalf("standard 14 width = %v, want 667", got)
	}
	_, res = type1Page(t, "", type1PageOptions{BaseFont: "Custom"})
	fnt := type1Font(t, res)
	if got := fnt.Width(type1synth.CodeA); got != type1synth.AdvanceA {
		t.Fatalf("charstring width = %v, want %d", got, type1synth.AdvanceA)
	}
	if got := fnt.Width(type1synth.CodeB); got != type1synth.AdvanceB {
		t.Fatalf("charstring width B = %v, want %d", got, type1synth.AdvanceB)
	}
	// /MissingWidth is the fallback when the code has no glyph at all.
	_, res = type1Page(t, "", type1PageOptions{
		BaseFont:     "Custom",
		MissingWidth: 250,
	})
	fnt = type1Font(t, res)
	if got := fnt.Width('Z'); got != 250 {
		t.Fatalf("missing /MissingWidth = %v, want 250", got)
	}
	if got := fnt.Width(type1synth.CodeA); got != type1synth.AdvanceA {
		t.Fatalf("program width with MissingWidth = %v, want %d", got, type1synth.AdvanceA)
	}
	_, res = type1Page(t, "", type1PageOptions{
		BaseFont: "Custom",
		Flags:    32,
	})
	fnt = type1Font(t, res)
	if got := fnt.Width('Z'); got != 0 {
		t.Fatalf("unnamed code width = %v, want 0", got)
	}
}

// TestType1Unicode proves the built-in encoding start, the PDF encoding and
// /Differences on top, and the AGL lookup.
func TestType1Unicode(t *testing.T) {
	t.Parallel()
	checkType1BuiltinUnicode(t)
	checkType1DifferenceUnicode(t)
}

func checkType1BuiltinUnicode(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{})
	fnt := type1Font(t, res)
	if got, ok := fnt.Unicode(type1synth.CodeA); !ok || got != "A" {
		t.Fatalf("Unicode(A) = %q, %v", got, ok)
	}
	if got, ok := fnt.Unicode(type1synth.CodeHint); !ok || got != "E" {
		t.Fatalf("Unicode(E) = %q, %v", got, ok)
	}
	if got, ok := fnt.Unicode(90); ok || got != "" {
		t.Fatalf("Unicode(90) = %q, %v, want the code-point fallback", got, ok)
	}
}

func checkType1DifferenceUnicode(t *testing.T) {
	t.Helper()
	_, res := type1Page(t, "", type1PageOptions{
		Encoding: "/Encoding << /BaseEncoding /WinAnsiEncoding /Differences [65 /Aacute] >>",
	})
	fnt := type1Font(t, res)
	if got, ok := fnt.Unicode(type1synth.CodeA); !ok || got != "\u00C1" {
		t.Fatalf("Unicode with differences = %q, %v", got, ok)
	}
	if got, ok := fnt.Unicode(type1synth.CodeB); !ok || got != "B" {
		t.Fatalf("Unicode(B) = %q, %v", got, ok)
	}
}

// TestType1Paint proves the pixels through the coverage path: the Type 1 A
// matches the synthetic TrueType A, a locked PPM records the result, and a
// seac glyph, a Subr glyph, and a corrupt program behave.
func TestType1Paint(t *testing.T) {
	t.Parallel()
	checkType1Pixels(t)
	checkType1LockedPPM(t)
	checkType1SeacPaint(t)
	checkType1CorruptPaint(t)
}

// paintType1Page paints one synthetic Type 1 page on a 20 by 20 pixmap.
func paintType1Page(t *testing.T, content string, opt type1PageOptions) graphics.Image {
	t.Helper()
	file, _ := type1Page(t, content, opt)
	pixmap := graphics.NewPixmap(20, 20)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	return shown(t, pixmap)
}

func checkType1Pixels(t *testing.T) {
	t.Helper()
	const content = "BT /F1 12 Tf 1 1 Td (A) Tj ET"
	trueType := paintTextPage(t, content)
	type1 := paintType1Page(t, content, type1PageOptions{
		Flags:  32,
		Widths: "/FirstChar 65 /Widths [600]",
	})
	if !bytes.Equal(ppmBytes(trueType), ppmBytes(type1)) {
		t.Fatal("the Type 1 A does not match the synthetic TrueType A")
	}
}

func checkType1LockedPPM(t *testing.T) {
	t.Helper()
	image := paintType1Page(t, "BT /F1 12 Tf 1 1 Td (A) Tj ET", type1PageOptions{
		Flags:  32,
		Widths: "/FirstChar 65 /Widths [600]",
	})
	checkPPM(t, "../../sampledata/fixtures/type1-tj.ppm", image)
	checkFixtureSHA(t, "../../sampledata/fixtures/type1-tj.ppm", type1PPMHash)
}

// type1PPMHash locks the fixture bytes. UPDATE_FIXTURES=1 rewrites the file,
// and the hash is recorded from that run.
const type1PPMHash = "f42fa88995735407f44c5146d579bdf53d0b1332c63a7f0628a85cf70037357a"

// checkFixtureSHA verifies a checked-in fixture against a recorded SHA-256.
func checkFixtureSHA(t *testing.T, path, want string) {
	t.Helper()
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("fixture %s sha256 = %s, want %s", path, got, want)
	}
}

func checkType1SeacPaint(t *testing.T) {
	t.Helper()
	for _, code := range []string{"B", "C"} {
		image := paintType1Page(t, "BT /F1 12 Tf 1 1 Td ("+code+") Tj ET", type1PageOptions{
			Widths: "/FirstChar 65 /Widths [600 400 600 500 600]",
		})
		if _, _, _, _, ok := markedBounds(image); !ok {
			t.Fatalf("glyph %s painted nothing", code)
		}
	}
}

func checkType1CorruptPaint(t *testing.T) {
	t.Helper()
	file, _ := type1Page(t, "BT /F1 12 Tf 10 20 Td (A) Tj ET", type1PageOptions{
		Program: type1synth.Options{LenIV: 4, Broken: true},
		Widths:  "/FirstChar 65 /Widths [600]",
	})
	pixmap := graphics.NewPixmap(20, 20)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, opShow, errInvalidFont)
	if _, _, _, _, marked := markedBounds(shown(t, pixmap)); marked {
		t.Fatal("a corrupt program painted pixels")
	}
}

// TestType1Extract proves the built-in AGL names and the code-point fallback
// through File.ExtractText.
func TestType1Extract(t *testing.T) {
	t.Parallel()
	file, _ := type1Page(t, "BT /F1 12 Tf 10 20 Td (ABZ) Tj ET", type1PageOptions{})
	text, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "ABZ\r\n" {
		t.Fatalf("ExtractText = %q, want %q", text, "ABZ\r\n")
	}
}
