package font

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
)

// Type 1 opcodes the hand-built charstrings use. The byte type keeps the
// encoder from writing them as numbers.
const (
	testOpHStem     byte = 1
	testOpVStem     byte = 3
	testOpRLineTo   byte = 5
	testOpSeac      byte = 6
	testOpClosePath byte = 9
	testOpCallSubr  byte = 10
	testOpReturn    byte = 11
	testOpEscape    byte = 12
	testOpDiv       byte = 12
	testOpHSBW      byte = 13
	testOpEndChar   byte = 14
	testOpCallOther byte = 16
	testOpPop       byte = 17
	testOpRMoveTo   byte = 21
	testOpHMoveTo   byte = 22
	testOpSetCurPt  byte = 33
)

// loadSynthProgram loads one synthetic program and fails the test when the
// decoder rejects it.
func loadSynthProgram(t *testing.T, opt type1synth.Options) *Type1Font {
	t.Helper()
	data, lengths := type1synth.Program(opt)
	program, err := LoadType1(data, lengths)
	if err != nil {
		t.Fatalf("LoadType1: %v", err)
	}
	return program
}

// checkSynthIdentity proves the name, matrix, and built-in encoding the
// cleartext and private section carry.
func checkSynthIdentity(t *testing.T, program *Type1Font) {
	t.Helper()
	if program.Name != type1synth.FontName {
		t.Fatalf("Name = %q, want %q", program.Name, type1synth.FontName)
	}
	want := [6]float64{0.001, 0, 0, 0.001, 0, 0}
	if program.FontMatrix != want {
		t.Fatalf("FontMatrix = %v, want %v", program.FontMatrix, want)
	}
	codes := map[byte]string{
		type1synth.CodeA:    type1synth.GlyphA,
		type1synth.CodeB:    type1synth.GlyphB,
		type1synth.CodeSeac: type1synth.GlyphSeac,
		type1synth.CodeFlex: type1synth.GlyphFlex,
		type1synth.CodeHint: type1synth.GlyphHint,
	}
	for code := range 256 {
		want := codes[byte(code)]
		if got := program.Encoding[code]; got != want {
			t.Fatalf("Encoding[%d] = %q, want %q", code, got, want)
		}
	}
	for _, name := range []string{type1synth.GlyphA, type1synth.GlyphB,
		type1synth.GlyphSeac, type1synth.GlyphFlex, type1synth.GlyphHint} {
		if !program.HasGlyph(name) {
			t.Fatalf("HasGlyph(%q) = false", name)
		}
	}
	if program.HasGlyph("Nope") {
		t.Fatal("HasGlyph(Nope) = true")
	}
}

// TestType1Program proves the PFA and PFB containers, the hex and binary
// eexec forms, lenIV 0 and 4, the /Length preference, and the error paths for
// a truncated or undecryptable program.
func TestType1Program(t *testing.T) {
	t.Parallel()
	checkType1Containers(t)
	checkType1Lengths(t)
	checkType1Errors(t)
}

func checkType1Containers(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		opt  type1synth.Options
	}{
		{"pfa-binary-lenIV4", type1synth.Options{LenIV: 4}},
		{"pfa-hex-lenIV4", type1synth.Options{Hex: true, LenIV: 4}},
		{"pfa-binary-lenIV0", type1synth.Options{LenIV: 0}},
		{"pfb-binary-lenIV4", type1synth.Options{PFB: true, LenIV: 4}},
		{"pfb-binary-lenIV0", type1synth.Options{PFB: true, LenIV: 0}},
	}
	for _, testCase := range cases {
		program := loadSynthProgram(t, testCase.opt)
		checkSynthIdentity(t, program)
	}
}

func checkType1Lengths(t *testing.T) {
	t.Helper()
	data, lengths := type1synth.Program(type1synth.Options{LenIV: 4})
	junk := append(append([]byte{}, data...), []byte("\n%% trailing junk\n")...)
	program, err := LoadType1(junk, lengths)
	if err != nil {
		t.Fatalf("lengths with trailing junk: %v", err)
	}
	checkSynthIdentity(t, program)
	program, err = LoadType1(junk, [3]int{})
	if err != nil {
		t.Fatalf("markers with trailing junk: %v", err)
	}
	checkSynthIdentity(t, program)
}

func checkType1Errors(t *testing.T) {
	t.Helper()
	broken, lengths := type1synth.Program(type1synth.Options{LenIV: 4, Broken: true})
	if _, err := LoadType1(broken, lengths); err == nil {
		t.Fatal("LoadType1 accepted a corrupt program")
	}
	good, lengths := type1synth.Program(type1synth.Options{LenIV: 4})
	if _, err := LoadType1(good[:len(good)/2], lengths); err == nil {
		t.Fatal("LoadType1 accepted a truncated program")
	}
	if _, err := LoadType1(nil, [3]int{}); err == nil {
		t.Fatal("LoadType1 accepted an empty program")
	}
}

// TestType1Charstring proves the interpreter: hsbw and sbw widths, the first
// operator fallback, the moveto, lineto, and curveto families, closepath,
// callsubr with a direct and a negative index, div, endchar, hint operators,
// and the caps for malformed programs.
func TestType1Charstring(t *testing.T) {
	t.Parallel()
	checkType1A(t)
	checkType1SubrB(t)
	checkType1Widths(t)
	checkType1Curves(t)
	checkType1Layout(t)
	checkType1Caps(t)
	checkType1Malformed(t)
}

// checkType1Segments compares a segment slice with expected points.
func checkType1Segments(t *testing.T, segments []Type1Segment, want []Type1Segment) {
	t.Helper()
	if len(segments) != len(want) {
		t.Fatalf("segments = %d, want %d: %+v", len(segments), len(want), segments)
	}
	for index := range want {
		got, expected := segments[index], want[index]
		if got.Op != expected.Op || len(got.Args) != len(expected.Args) {
			t.Fatalf("segment %d = %+v, want %+v", index, got, expected)
		}
		for point := range expected.Args {
			if !t1Near(got.Args[point].X, expected.Args[point].X) ||
				!t1Near(got.Args[point].Y, expected.Args[point].Y) {
				t.Fatalf("segment %d point %d = %+v, want %+v",
					index, point, got.Args[point], expected.Args[point])
			}
		}
	}
}

func t1Near(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// type1Point is a shorter spelling for the test tables.
func type1Point(x, y float64) Type1Point {
	return Type1Point{X: x, Y: y}
}

// rectSegments is the closed rectangle the A, B, and seac parts draw.
func rectSegments(minX, maxX, maxY float64) []Type1Segment {
	return []Type1Segment{
		{Op: Type1MoveTo, Args: []Type1Point{type1Point(minX, 0)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(maxX, 0)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(maxX, maxY)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(minX, maxY)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(minX, 0)}},
	}
}

func checkType1A(t *testing.T) {
	t.Helper()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	glyph, err := program.Glyph(type1synth.GlyphA)
	if err != nil {
		t.Fatal(err)
	}
	checkType1Segments(t, glyph.Segments, rectSegments(type1synth.AMinX, type1synth.AMaxX, type1synth.AMaxY))
	if !glyph.HasWidth || !t1Near(glyph.Advance, type1synth.AdvanceA) {
		t.Fatalf("A advance = %v, has = %v", glyph.Advance, glyph.HasWidth)
	}
}

func checkType1SubrB(t *testing.T) {
	t.Helper()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	glyph, err := program.Glyph(type1synth.GlyphB)
	if err != nil {
		t.Fatal(err)
	}
	checkType1Segments(t, glyph.Segments, rectSegments(type1synth.BMinX, type1synth.BMaxX, type1synth.BMaxY))
	if !glyph.HasWidth || !t1Near(glyph.Advance, type1synth.AdvanceB) {
		t.Fatalf("B advance = %v, has = %v", glyph.Advance, glyph.HasWidth)
	}
}

// customFont builds a font from raw charstrings so one operator can be
// isolated in a test.
func customFont(charstrings map[string][]byte) *Type1Font {
	return &Type1Font{
		FontMatrix:  [6]float64{0.001, 0, 0, 0.001, 0, 0},
		charStrings: charstrings,
		lenIV:       0,
	}
}

func checkType1Widths(t *testing.T) {
	t.Helper()
	// sbw carries an explicit sidebearing and width vector.
	fnt := customFont(map[string][]byte{"sbw": cs(10, 20, 700, 0, testOpEscape, byte(7), testOpEndChar)})
	glyph, err := fnt.Glyph("sbw")
	if err != nil {
		t.Fatal(err)
	}
	if !glyph.HasWidth || !t1Near(glyph.Advance, 700) || len(glyph.Segments) != 0 {
		t.Fatalf("sbw = %+v, want width 700 and no segments", glyph)
	}
	// A charstring without hsbw takes its width from the extra rmoveto
	// operand, the first stack-clearing operator.
	fnt = customFont(map[string][]byte{"first": cs(700, 0, 0, testOpRMoveTo, 0, 0, testOpRLineTo, testOpEndChar)})
	glyph, err = fnt.Glyph("first")
	if err != nil {
		t.Fatal(err)
	}
	if !glyph.HasWidth || !t1Near(glyph.Advance, 700) {
		t.Fatalf("first rmoveto width = %+v", glyph)
	}
	// div runs before the moveto and yields a real result.
	fnt = customFont(map[string][]byte{"div": cs(1000, 2, testOpEscape, testOpDiv, 0, testOpRMoveTo, testOpEndChar)})
	glyph, err = fnt.Glyph("div")
	if err != nil {
		t.Fatal(err)
	}
	want := []Type1Segment{{Op: Type1MoveTo, Args: []Type1Point{type1Point(500, 0)}}}
	checkType1Segments(t, glyph.Segments, want)
}

func checkType1Curves(t *testing.T) {
	t.Helper()
	// rrcurveto, then hvcurveto, then vhcurveto.
	code := cs(
		0, 0, testOpRMoveTo,
		10, 0, 0, 10, 10, 0, byte(8),
		5, 0, 0, 5, byte(31),
		5, 0, 0, 5, byte(30),
		testOpEndChar)
	fnt := customFont(map[string][]byte{"curve": code})
	glyph, err := fnt.Glyph("curve")
	if err != nil {
		t.Fatal(err)
	}
	want := []Type1Segment{
		{Op: Type1MoveTo, Args: []Type1Point{type1Point(0, 0)}},
		{Op: Type1CurveTo, Args: []Type1Point{type1Point(10, 0), type1Point(10, 10), type1Point(20, 10)}},
		{Op: Type1CurveTo, Args: []Type1Point{type1Point(25, 10), type1Point(25, 10), type1Point(25, 15)}},
		{Op: Type1CurveTo, Args: []Type1Point{type1Point(25, 20), type1Point(25, 20), type1Point(30, 20)}},
	}
	checkType1Segments(t, glyph.Segments, want)
}

func checkType1Layout(t *testing.T) {
	t.Helper()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	hint, err := program.Glyph(type1synth.GlyphHint)
	if err != nil {
		t.Fatal(err)
	}
	if !hint.HasWidth || !t1Near(hint.Advance, type1synth.AdvanceHint) {
		t.Fatalf("hint replacement advance = %+v", hint)
	}
	if len(hint.Segments) != 5 {
		t.Fatalf("hint replacement segments = %d, want 5", len(hint.Segments))
	}
	// A negative callsubr index counts from the end of the Subrs array.
	fnt := customFont(map[string][]byte{
		"neg": cs(10, 20, testOpRMoveTo, -1, testOpCallSubr, testOpEndChar),
	})
	fnt.subrs = [][]byte{nil, cs(100, 0, testOpRLineTo, 0, 50, testOpRLineTo, testOpReturn)}
	glyph, err := fnt.Glyph("neg")
	if err != nil {
		t.Fatal(err)
	}
	negative := []Type1Segment{
		{Op: Type1MoveTo, Args: []Type1Point{type1Point(10, 20)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(110, 20)}},
		{Op: Type1LineTo, Args: []Type1Point{type1Point(110, 70)}},
	}
	checkType1Segments(t, glyph.Segments, negative)
	// An unknown OtherSubr returns its arguments for the following pops and
	// keeps the stack balanced.
	fnt = customFont(map[string][]byte{
		"unknown": cs(5, 6, 7, 3, 99, testOpEscape, testOpCallOther,
			testOpEscape, testOpPop, testOpEscape, testOpPop, testOpEscape, testOpPop, testOpEndChar),
	})
	if _, err := fnt.Glyph("unknown"); err != nil {
		t.Fatalf("unknown OtherSubr: %v", err)
	}
}

func checkType1Caps(t *testing.T) {
	t.Helper()
	long := make([]byte, 0, 64)
	for range type1MaxOperands + 1 {
		long = append(long, 139)
	}
	long = append(long, testOpEndChar)
	if _, err := customFont(map[string][]byte{"deep": long}).Glyph("deep"); err == nil {
		t.Fatal("the operand stack cap did not stop the charstring")
	}
	// A Subr that calls itself reaches the call depth cap.
	fnt := customFont(map[string][]byte{"deep": cs(0, testOpCallSubr, testOpEndChar)})
	fnt.subrs = [][]byte{cs(0, testOpCallSubr, testOpReturn)}
	if _, err := fnt.Glyph("deep"); err == nil {
		t.Fatal("the call depth cap did not stop the charstring")
	}
	// Enough rlineto segments to cross the point cap.
	points := make([]byte, 0, 64)
	points = append(points, 0, 0, testOpRMoveTo)
	for range type1MaxPoints + 1 {
		points = append(points, 139, 139, testOpRLineTo)
	}
	points = append(points, testOpEndChar)
	if _, err := customFont(map[string][]byte{"points": points}).Glyph("points"); err == nil {
		t.Fatal("the point cap did not stop the charstring")
	}
}

func checkType1Malformed(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		code []byte
	}{
		{"reserved-op", cs(0, 0, byte(2), testOpEndChar)},
		{"reserved-escape", cs(0, 0, testOpEscape, byte(99), testOpEndChar)},
		{"truncated-number", []byte{255, 1, 2}},
		{"missing-subr", cs(0, testOpCallSubr, testOpEndChar)},
		{"div-by-zero", cs(1, 0, testOpEscape, testOpDiv, testOpEndChar)},
	}
	fnt := customFont(map[string][]byte{"bad": nil})
	for _, testCase := range cases {
		fnt.charStrings["bad"] = testCase.code
		if _, err := fnt.Glyph("bad"); err == nil {
			t.Fatalf("%s: Glyph accepted the program", testCase.name)
		}
	}
	if _, err := fnt.Glyph("absent"); err == nil {
		t.Fatal("Glyph accepted an absent name")
	}
}

// TestType1Encoding proves the built-in encoding forms: the array and put
// loop, a literal array, and a named table.
func TestType1Encoding(t *testing.T) {
	t.Parallel()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	if program.Encoding[type1synth.CodeA] != type1synth.GlyphA {
		t.Fatalf("array encoding A = %q", program.Encoding[type1synth.CodeA])
	}
	if program.Encoding[64] != "" {
		t.Fatalf("code 64 = %q, want empty", program.Encoding[64])
	}
	checkType1EncodingForm(t, "/Encoding [ /A /B /C ] def", "C", 2)
	checkType1EncodingForm(t, "/Encoding StandardEncoding def", "A", type1synth.CodeA)
}

func checkType1EncodingForm(t *testing.T, source, want string, code int) {
	t.Helper()
	tokens, err := type1Tokenize([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	start := t1FindLiteral(tokens, "Encoding")
	encoding, _, ok := t1Encoding(tokens, start+1)
	if !ok {
		t.Fatalf("%q did not parse", source)
	}
	if code >= 0 && encoding[code] != want {
		t.Fatalf("%q code %d = %q, want %q", source, code, encoding[code], want)
	}
}

// TestType1Seac proves the base and accent composition, the accent offset,
// the base width, and the nested seac error.
func TestType1Seac(t *testing.T) {
	t.Parallel()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	glyph, err := program.Glyph(type1synth.GlyphSeac)
	if err != nil {
		t.Fatal(err)
	}
	offset := float64(150 + type1synth.AMinX - type1synth.BMinX)
	want := append(rectSegments(type1synth.AMinX, type1synth.AMaxX, type1synth.AMaxY),
		rectSegments(float64(type1synth.BMinX)+offset, float64(type1synth.BMaxX)+offset, type1synth.BMaxY)...)
	checkType1Segments(t, glyph.Segments, want)
	if !glyph.HasWidth || !t1Near(glyph.Advance, type1synth.AdvanceSeac) {
		t.Fatalf("seac advance = %v, has = %v", glyph.Advance, glyph.HasWidth)
	}
	// A base that is itself a seac is malformed.
	seacCode := func(base, accent int) []byte {
		return cs(0, 500, testOpHSBW, 50, 150, 0, base, accent, testOpEscape, testOpSeac, testOpEndChar)
	}
	fnt := customFont(map[string][]byte{
		"bad":                seacCode(type1synth.CodeSeac, type1synth.CodeA),
		type1synth.GlyphSeac: seacCode(type1synth.CodeA, type1synth.CodeB),
		type1synth.GlyphA:    cs(0, 500, testOpHSBW, testOpEndChar),
		type1synth.GlyphB:    cs(0, 500, testOpHSBW, testOpEndChar),
	})
	if _, err := fnt.Glyph("bad"); err == nil {
		t.Fatal("a seac base that is a seac was accepted")
	}
	// An accent whose StandardEncoding code has no glyph is malformed too.
	fnt.charStrings["bad"] = seacCode(type1synth.CodeA, 200)
	if _, err := fnt.Glyph("bad"); err == nil {
		t.Fatal("a missing accent was accepted")
	}
}

// TestType1Flex proves the flex curves, the final point, the hint
// replacement Subr, and the Multiple Master blend entry.
func TestType1Flex(t *testing.T) {
	t.Parallel()
	program := loadSynthProgram(t, type1synth.Options{LenIV: 4})
	glyph, err := program.Glyph(type1synth.GlyphFlex)
	if err != nil {
		t.Fatal(err)
	}
	want := []Type1Segment{
		{Op: Type1MoveTo, Args: []Type1Point{type1Point(type1synth.AMinX, 90)}},
		{Op: Type1CurveTo, Args: []Type1Point{type1Point(115, 90), type1Point(125, 100), type1Point(150, 100)}},
		{Op: Type1CurveTo, Args: []Type1Point{type1Point(175, 100), type1Point(185, 90), type1Point(200, 90)}},
	}
	checkType1Segments(t, glyph.Segments, want)
	if !glyph.HasWidth || !t1Near(glyph.Advance, type1synth.AdvanceFlex) {
		t.Fatalf("flex advance = %v, has = %v", glyph.Advance, glyph.HasWidth)
	}
	// With a /WeightVector, OtherSubr 14 interpolates the argument.
	fnt := customFont(map[string][]byte{
		"blend": cs(10, 20, 2, 14, testOpEscape, testOpCallOther,
			testOpEscape, testOpPop, testOpHMoveTo, testOpEndChar),
	})
	fnt.weight = []float64{0.25, 0.75}
	glyph, err = fnt.Glyph("blend")
	if err != nil {
		t.Fatal(err)
	}
	blended := []Type1Segment{{Op: Type1MoveTo, Args: []Type1Point{type1Point(25, 0)}}}
	checkType1Segments(t, glyph.Segments, blended)
	// Without one, OtherSubr 14 keeps the stack balanced and the operands
	// are the pop results.
	fnt.weight = nil
	if _, err := fnt.Glyph("blend"); err != nil {
		t.Fatalf("blend without a WeightVector: %v", err)
	}
}

// TestType1Cipher locks the stream cipher against a vector computed
// independently from the format: the state updates with the ciphertext byte.
func TestType1Cipher(t *testing.T) {
	t.Parallel()
	cipher, err := hex.DecodeString("bd3726dbafe9cc803a69cea8")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(type1Decrypt(cipher, type1SeedEexec)); got != "dup /Private" {
		t.Fatalf("eexec decrypt = %q", got)
	}
	charstring, err := hex.DecodeString("118f14534706f562")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(type1Decrypt(charstring, type1SeedCharstring)); got != "\x01\x02\x03\x04hsbw" {
		t.Fatalf("charstring decrypt = %q", got)
	}
}

// cs is a local charstring encoder for hand-built test programs. Integers are
// numbers and bytes are operators.
func cs(parts ...any) []byte {
	var out []byte
	for _, part := range parts {
		switch value := part.(type) {
		case int:
			out = append(out, encodeTestInt(value)...)
		case byte:
			out = append(out, value)
		case []byte:
			out = append(out, value...)
		}
	}
	return out
}

// encodeTestInt encodes one Type 1 charstring number.
func encodeTestInt(value int) []byte {
	switch {
	case value >= -107 && value <= 107:
		return []byte{byte(value + 139)}
	case value >= 108 && value <= 1131:
		value -= 108
		return []byte{byte((value >> 8) + 247), byte(value)}
	case value >= -1131 && value <= -108:
		value = -value - 108
		return []byte{byte((value >> 8) + 251), byte(value)}
	default:
		return []byte{255, byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	}
}
