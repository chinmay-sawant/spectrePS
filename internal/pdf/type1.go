package pdf

import (
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// glyphName resolves a character code to a glyph name. The PDF encoding wins;
// a symbolic font falls back to the built-in encoding of its Type 1 program.
func (f *Font) glyphName(code uint32) string {
	if f == nil {
		return ""
	}
	if name := f.encoding[byte(code)]; name != "" {
		return name
	}
	return f.builtinEncoding[byte(code)]
}

// type1Outline interprets one code through the Type 1 program and converts
// the outline to sfnt.Segments at outlinePPEM. A name the charstrings
// dictionary does not carry returns false, which paints as invalidfont.
func (f *Font) type1Outline(code uint32) (sfnt.Segments, bool) {
	name := f.glyphName(code)
	if name == "" || !f.type1.HasGlyph(name) {
		return nil, false
	}
	glyph, err := f.type1.Glyph(name)
	if err != nil {
		return nil, false
	}
	return type1Segments(f.type1.FontMatrix, glyph.Segments), true
}

// type1Advance returns the charstring width of one code in 1/1000 em.
func (f *Font) type1Advance(code uint32) (float64, bool) {
	name := f.glyphName(code)
	if name == "" || !f.type1.HasGlyph(name) {
		return 0, false
	}
	glyph, err := f.type1.Glyph(name)
	if err != nil || !glyph.HasWidth {
		return 0, false
	}
	return glyph.Advance, true
}

// type1Segments converts a charstring outline to the sfnt shape at
// outlinePPEM. The FontMatrix maps glyph space to em units and the Y axis
// flips, matching what sfnt.LoadGlyph returns for TrueType.
func type1Segments(matrix [6]float64, segments []font.Type1Segment) sfnt.Segments {
	out := make(sfnt.Segments, 0, len(segments))
	for _, segment := range segments {
		op, ok := type1SegmentOp(segment.Op)
		if !ok {
			continue
		}
		converted := sfnt.Segment{Op: op, Args: [3]fixed.Point26_6{}}
		for index, point := range segment.Args {
			converted.Args[index] = type1Point(matrix, point)
		}
		out = append(out, converted)
	}
	return out
}

// type1SegmentOp maps one outline command to the sfnt form.
func type1SegmentOp(op font.Type1Op) (sfnt.SegmentOp, bool) {
	switch op {
	case font.Type1MoveTo:
		return sfnt.SegmentOpMoveTo, true
	case font.Type1LineTo:
		return sfnt.SegmentOpLineTo, true
	case font.Type1CurveTo:
		return sfnt.SegmentOpCubeTo, true
	default:
		return sfnt.SegmentOpMoveTo, false
	}
}

// type1Point scales one glyph-space point through the FontMatrix, flips Y,
// and rounds to 26.6 at outlinePPEM, the same rounding sfnt applies.
func type1Point(matrix [6]float64, point font.Type1Point) fixed.Point26_6 {
	emX := matrix[0]*point.X + matrix[2]*point.Y + matrix[4]
	emY := matrix[1]*point.X + matrix[3]*point.Y + matrix[5]
	scale := float64(outlinePPEM) * fixedShift
	return fixed.Point26_6{
		X: fixed.Int26_6(math.Round(emX * scale)),
		Y: fixed.Int26_6(math.Round(-emY * scale)),
	}
}
