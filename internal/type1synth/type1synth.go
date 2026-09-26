// Package type1synth builds the synthetic Type 1 font programs the tests
// embed. Every byte is generated here, so no real font file travels with the
// module or its licenses. The font carries a .notdef, an A whose rectangle
// matches the synthetic TrueType A in internal/pdf, a B drawn from a Subrs
// entry, a seac C, a flex D, and a hint-replacement E.
package type1synth

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// Glyph names and character codes the synthetic font carries.
const (
	FontName  = "SynthType1"
	GlyphDef  = ".notdef"
	GlyphA    = "A"
	GlyphB    = "B"
	GlyphSeac = "C"
	GlyphFlex = "D"
	GlyphHint = "E"

	CodeA    = 65
	CodeB    = 66
	CodeSeac = 67
	CodeFlex = 68
	CodeHint = 69
)

// Advances, in 1/1000 em, and the rectangles the glyphs draw.
const (
	AdvanceNotdef = 500
	AdvanceA      = 600
	AdvanceB      = 400
	AdvanceSeac   = 600
	AdvanceFlex   = 500
	AdvanceHint   = 600

	AMinX = 100
	AMaxX = 600
	AMaxY = 700
	BMinX = 50
	BMaxX = 350
	BMaxY = 300
)

// Container and cipher constants of the Type 1 format.
const (
	eexecSeed      = 55665
	charstringSeed = 4330
	cipher1        = 52845
	cipher2        = 22719
	cipherShift    = 8
	trailerZeros   = 512
	pfbMarker      = 0x80
	pfbClearType   = 1
	pfbBinaryType  = 2
	pfbEndType     = 3
	pfbHeaderLen   = 6
	randomPrefix   = 4
)

// Type 1 charstring opcodes the programs use. The byte type keeps cs from
// encoding them as numbers.
const (
	opHStem           byte = 1
	opVStem           byte = 3
	opRLineTo         byte = 5
	opSeac            byte = 6
	opClosePath       byte = 9
	opCallSubr        byte = 10
	opReturn          byte = 11
	opEscape          byte = 12
	opHSBW            byte = 13
	opEndChar         byte = 14
	opRMoveTo         byte = 21
	opCallOtherSubr   byte = 16
	opPop             byte = 17
	opSetCurrentPoint byte = 33
)

// OtherSubrs entry numbers and the operands the flex and hint sequences pass.
const (
	otherSubrFlex  = 0
	otherSubrBegin = 1
	otherSubrPoint = 2
	otherSubrHints = 3

	flexPoints  = 3
	hintSubr    = 1
	hintCount   = 1
	hintOther   = 3
	seacOffset  = 150
	seacAccentY = 0
)

// Number encoding constants of the Type 1 charstring format.
const (
	numberBias   = 139
	numberBase2  = 247
	numberNeg    = 251
	numberOffset = 108
	numberByte   = 8
	numberByte2  = 16
	numberByte3  = 24
)

// Flex geometry from the example in the Type 1 specification, lifted into the
// em square: a start, a reference point, two control points, the joining
// point, two more control points, and the end point.
const (
	flexStartY = 90
	flexRefX   = 50
	flexBcp1X  = -35
	flexBcp2X  = 10
	flexBcp2Y  = 10
	flexJoinX  = 25
	flexBcp4X  = 10
	flexBcp4Y  = -10
	flexEndX   = 15
	flexHeight = 50
	flexFinalX = 200
	flexFinalY = 90
)

// Hint geometry of the hint replacement Subrs entry.
const (
	hintStemY  = 20
	hintStemX  = 30
	hintPoint1 = 0
)

// Options selects the container, the eexec encoding, and the charstring
// prefix of one program.
type Options struct {
	// PFB writes the PFB container instead of a PFA.
	PFB bool
	// Hex writes the eexec section as ASCII hex instead of binary bytes.
	Hex bool
	// LenIV is the number of random bytes before each charstring. 0 and 4
	// are the values the format uses.
	LenIV int
	// Broken flips the eexec ciphertext so LoadType1 cannot decrypt it.
	Broken bool
}

// Program returns one synthetic font and the /Length1, /Length2, and
// /Length3 values for a PDF stream. The lengths cover the clear text, the
// encrypted section as stored, and the trailer.
func Program(opt Options) ([]byte, [3]int) {
	clearText := []byte(clearText())
	private := privateText(opt.LenIV)
	cipher := encrypt(append(randomBytes(randomPrefix), private...), eexecSeed)
	if opt.Broken {
		for index := range cipher {
			cipher[index] ^= 0x5A
		}
	}
	if opt.PFB {
		return buildPFB(clearText, cipher), [3]int{len(clearText), len(cipher), trailerLen()}
	}
	return buildPFA(clearText, cipher, opt.Hex)
}

// clearText is the readable head of the font program.
func clearText() string {
	lines := []string{
		"%!PS-AdobeFont-1.0: " + FontName + " 1.0",
		"11 dict begin",
		"/FontName /" + FontName + " def",
		"/FontType 1 def",
		"/FontMatrix [0.001 0 0 0.001 0 0] readonly def",
		"/FontBBox [-100 -200 700 800] readonly def",
		"/Encoding 256 array",
		"0 1 255 {1 index exch /.notdef put} for",
		fmt.Sprintf("dup %d /%s put", CodeA, GlyphA),
		fmt.Sprintf("dup %d /%s put", CodeB, GlyphB),
		fmt.Sprintf("dup %d /%s put", CodeSeac, GlyphSeac),
		fmt.Sprintf("dup %d /%s put", CodeFlex, GlyphFlex),
		fmt.Sprintf("dup %d /%s put", CodeHint, GlyphHint),
		"readonly def",
		"currentdict end",
		"currentfile eexec",
		"",
	}
	return strings.Join(lines, "\n")
}

// privateText is the decrypted Private dictionary with its RD byte runs.
func privateText(lenIV int) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "dup /Private 12 dict dup begin\n")
	fmt.Fprintf(&buf, "/RD {string currentfile exch readstring pop} executeonly def\n")
	fmt.Fprintf(&buf, "/ND {noaccess def} executeonly def\n")
	fmt.Fprintf(&buf, "/NP {noaccess put} executeonly def\n")
	fmt.Fprintf(&buf, "/lenIV %d def\n", lenIV)
	fmt.Fprintf(&buf, "/Subrs 2 array\n")
	writeCharstring(&buf, "dup 0", subrB(), lenIV, "NP")
	writeCharstring(&buf, "dup 1", subrHints(), lenIV, "NP")
	fmt.Fprintf(&buf, "readonly def\n")
	fmt.Fprintf(&buf, "2 index /CharStrings 6 dict dup begin\n")
	writeCharstring(&buf, "/"+GlyphDef, charstringNotdef(), lenIV, "ND")
	writeCharstring(&buf, "/"+GlyphA, charstringA(), lenIV, "ND")
	writeCharstring(&buf, "/"+GlyphB, charstringB(), lenIV, "ND")
	writeCharstring(&buf, "/"+GlyphSeac, charstringSeac(), lenIV, "ND")
	writeCharstring(&buf, "/"+GlyphFlex, charstringFlex(), lenIV, "ND")
	writeCharstring(&buf, "/"+GlyphHint, charstringHint(), lenIV, "ND")
	fmt.Fprintf(&buf, "end\nreadonly put\nnoaccess put\n")
	fmt.Fprintf(&buf, "dup /FontName get exch definefont pop\nmark currentfile closefile\n")
	return buf.Bytes()
}

// writeCharstring writes one "prefix <length> RD <bytes> <end>" line. Subrs
// entries end with NP and CharStrings entries with ND, as real fonts do.
func writeCharstring(buf *bytes.Buffer, prefix string, body []byte, lenIV int, end string) {
	cipher := encrypt(append(randomBytes(lenIV), body...), charstringSeed)
	fmt.Fprintf(buf, "%s %d RD\n", prefix, len(cipher))
	buf.Write(cipher)
	fmt.Fprintf(buf, "%s\n", end)
}

// randomBytes returns the deterministic bytes that stand in for the random
// prefix of a charstring or eexec section.
func randomBytes(count int) []byte {
	values := []byte{0x28, 0xBF, 0x4E, 0x5B}
	out := make([]byte, count)
	for index := range out {
		out[index] = values[index%len(values)]
	}
	return out
}

// encrypt runs the Type 1 stream cipher with one seed. The state update uses
// the ciphertext byte, as the format defines.
func encrypt(plain []byte, seed uint16) []byte {
	state := seed
	out := make([]byte, len(plain))
	for index, value := range plain {
		cipher := value ^ byte(state>>cipherShift)
		out[index] = cipher
		state = (state+uint16(cipher))*cipher1 + cipher2
	}
	return out
}

// buildPFA writes the clear text, eexec section, and trailer.
func buildPFA(clearBytes, cipher []byte, hexEexec bool) ([]byte, [3]int) {
	var encrypted []byte
	if hexEexec {
		encrypted = []byte(hex.EncodeToString(cipher) + "\n")
	} else {
		encrypted = append(append([]byte{}, cipher...), '\n')
	}
	trailer := []byte(strings.Repeat("0", trailerZeros) + "\ncleartomark\n")
	data := make([]byte, 0, len(clearBytes)+len(encrypted)+len(trailer))
	data = append(data, clearBytes...)
	data = append(data, encrypted...)
	data = append(data, trailer...)
	return data, [3]int{len(clearBytes), len(encrypted), len(trailer)}
}

// buildPFB writes the three segments of a PFB file.
func buildPFB(clearBytes, cipher []byte) []byte {
	trailer := []byte(strings.Repeat("0", trailerZeros) + "\ncleartomark\n")
	var out []byte
	out = appendSegment(out, pfbClearType, clearBytes)
	out = appendSegment(out, pfbBinaryType, cipher)
	out = appendSegment(out, pfbEndType, trailer)
	return out
}

// trailerLen is the stored length of the PFA trailer.
func trailerLen() int {
	return trailerZeros + len("\ncleartomark\n")
}

// appendSegment writes one 0x80 header and its body.
func appendSegment(out []byte, kind byte, body []byte) []byte {
	header := make([]byte, pfbHeaderLen)
	header[0] = pfbMarker
	header[1] = kind
	binary.LittleEndian.PutUint32(header[2:], uint32(len(body))) //nolint:gosec // the fixture body is small
	return append(append(out, header...), body...)
}

// cs encodes a charstring from integer operands and byte operators.
func cs(parts ...any) []byte {
	var out []byte
	for _, part := range parts {
		switch value := part.(type) {
		case int:
			out = append(out, encodeInt(value)...)
		case byte:
			out = append(out, value)
		case []byte:
			out = append(out, value...)
		}
	}
	return out
}

// esc returns the two-byte escape form of a charstring operator.
func esc(op byte) []byte {
	return []byte{opEscape, op}
}

// encodeInt encodes one Type 1 charstring number.
func encodeInt(value int) []byte {
	switch {
	case value >= -107 && value <= 107:
		return []byte{byte(value + numberBias)}
	case value >= 108 && value <= 1131:
		value -= numberOffset
		high, low := byte((value>>numberByte)+numberBase2), byte(value)
		return []byte{high, low}
	case value >= -1131 && value <= -108:
		value = -value - numberOffset
		return []byte{byte((value >> numberByte) + numberNeg), byte(value)}
	default:
		return []byte{
			255,
			byte(value >> numberByte3),
			byte(value >> numberByte2),
			byte(value >> numberByte),
			byte(value),
		}
	}
}

// charstringNotdef draws nothing. The width is the only data it carries.
func charstringNotdef() []byte {
	return cs(0, AdvanceNotdef, opHSBW, opEndChar)
}

// charstringA draws the rectangle the synthetic TrueType A draws.
func charstringA() []byte {
	return cs(
		AMinX, AdvanceA, opHSBW,
		AMaxX-AMinX, 0, opRLineTo,
		0, AMaxY, opRLineTo,
		AMinX-AMaxX, 0, opRLineTo,
		opClosePath,
		opEndChar)
}

// subrB draws the B rectangle. It is Subrs entry 0.
func subrB() []byte {
	return cs(
		BMaxX-BMinX, 0, opRLineTo,
		0, BMaxY, opRLineTo,
		BMinX-BMaxX, 0, opRLineTo,
		opClosePath,
		opReturn)
}

// charstringB calls Subrs entry 0 for its outline.
func charstringB() []byte {
	return cs(BMinX, AdvanceB, opHSBW, 0, opCallSubr, opEndChar)
}

// charstringSeac builds the C glyph from A as the base and B as the accent.
// The asb operand is B's own sidebearing, as the format requires.
func charstringSeac() []byte {
	return cs(
		AMinX, AdvanceSeac, opHSBW,
		BMinX, seacOffset, seacAccentY, CodeA, CodeB, esc(opSeac),
		opEndChar)
}

// charstringFlex builds the D glyph from the flex example in the Type 1
// specification, moved up into the em square. It inlines the OtherSubrs calls
// instead of going through Subrs, because the font keeps two entries for B and
// the hint replacement.
func charstringFlex() []byte {
	return cs(
		AMinX, AdvanceFlex, opHSBW,
		0, flexStartY, opRMoveTo,
		0, otherSubrBegin, esc(opCallOtherSubr),
		flexRefX, 0, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexBcp1X, 0, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexBcp2X, flexBcp2Y, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexJoinX, 0, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexJoinX, 0, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexBcp4X, flexBcp4Y, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexEndX, 0, opRMoveTo, 0, otherSubrPoint, esc(opCallOtherSubr),
		flexHeight, flexFinalX, flexFinalY, flexPoints, otherSubrFlex,
		esc(opCallOtherSubr), esc(opPop), esc(opPop), esc(opSetCurrentPoint),
		opEndChar)
}

// subrHints carries only stem hints. It is the hint replacement target.
func subrHints() []byte {
	return cs(hintPoint1, hintStemY, opHStem, hintPoint1, hintStemX, opVStem, opReturn)
}

// charstringHint runs the hint replacement sequence and then draws the A
// rectangle.
func charstringHint() []byte {
	return cs(
		AMinX, AdvanceHint, opHSBW,
		hintSubr, hintCount, hintOther, esc(opCallOtherSubr), esc(opPop), opCallSubr,
		0, 0, opRMoveTo,
		AMaxX-AMinX, 0, opRLineTo,
		0, AMaxY, opRLineTo,
		AMinX-AMaxX, 0, opRLineTo,
		opClosePath,
		opEndChar)
}
