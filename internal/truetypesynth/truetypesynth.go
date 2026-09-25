// Package truetypesynth builds the synthetic TrueType programs the tests
// embed. Every byte is generated here, so no real font file travels with the
// module or its licenses. The font carries a .notdef, an A rectangle, a B
// rectangle, and a composite C that references A and B.
package truetypesynth

import (
	"encoding/binary"
)

// Font shape constants the tests assert against.
const (
	Upem       = 1000
	NumGlyphs  = 4
	GIDNotdef  = 0
	GIDA       = 1
	GIDB       = 2
	GIDC       = 3
	CodeA      = 65
	CodeB      = 66
	CodeC      = 67
	AdvanceNot = 500
	AdvanceA   = 600
	AdvanceB   = 400
	AdvanceC   = 600
)

// Composite constants of glyph C.
const (
	compositeContours = -1
	moreComponents    = 0x0020
	argWords          = 0x0001
	argsAreXY         = 0x0002
	glyfHeaderLen     = 10
	compHeaderLen     = 4
	compOffsetLen     = 4
)

// GlyphMinX values of the A and B rectangles, in font units.
const (
	AMinX = 100
	AMaxX = 600
	AMaxY = 700
	BMinX = 50
	BMaxX = 350
	BMaxY = 300
)

// Program returns the TrueType program.
func Program() []byte {
	builder := &builder{tables: nil}
	builder.add("cmap", cmapTable())
	builder.add("glyf", glyfTable())
	builder.add("head", headTable())
	builder.add("hhea", hheaTable())
	builder.add("hmtx", hmtxTable())
	builder.add("loca", locaTable())
	builder.add("maxp", maxpTable())
	builder.add("post", postTable())
	return builder.bytes()
}

type table struct {
	tag  string
	data []byte
}

type builder struct {
	tables []table
}

func (b *builder) add(tag string, data []byte) {
	b.tables = append(b.tables, table{tag: tag, data: data})
}

// bytes writes the font. The directory is sorted by tag, every table is
// padded to four bytes, and the head checkSumAdjustment is written.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func (b *builder) bytes() []byte {
	count := len(b.tables)
	searchRange, entrySelector := searchParams(count)
	rangeShift := 16*count - searchRange
	total := 12 + 16*count
	offsets := make([]int, count)
	lengths := make([]int, count)
	for idx, item := range b.tables {
		offsets[idx] = total
		lengths[idx] = len(item.data)
		total += padded4(len(item.data))
	}
	out := make([]byte, total)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, count)
	putU16(out, 6, searchRange)
	putU16(out, 8, entrySelector)
	putU16(out, 10, rangeShift)
	for idx, item := range b.tables {
		entry := out[12+16*idx:]
		copy(entry[0:4], item.tag)
		putU32(entry, 4, Checksum(item.data))
		putOffset(entry, 8, offsets[idx])
		putOffset(entry, 12, lengths[idx])
		copy(out[offsets[idx]:], item.data)
	}
	adjust := uint32(0xB1B0AFBA) - Checksum(out)
	for idx, item := range b.tables {
		if item.tag == "head" {
			putU32(out, offsets[idx]+8, adjust)
		}
	}
	return out
}

// searchParams returns the binary-search header fields for one table count.
func searchParams(count int) (int, int) {
	searchRange := 16
	entrySelector := 0
	for searchRange*2 <= 16*count {
		searchRange *= 2
		entrySelector++
	}
	return searchRange, entrySelector
}

// Checksum returns the sfnt checksum of data: the sum of its big-endian
// 32-bit words, with a short final word zero-padded.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func Checksum(data []byte) uint32 {
	var sum uint32
	for idx := 0; idx < len(data); idx += 4 {
		var word [4]byte
		copy(word[:], data[idx:min(idx+4, len(data))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func padded4(size int) int {
	return (size + 3) &^ 3
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func headTable() []byte {
	out := make([]byte, 54)
	putU32(out, 0, 0x00010000)
	putU32(out, 4, 0x00010000)
	putU32(out, 12, 0x5F0F3CF5)
	putU16(out, 16, 0)
	putU16(out, 18, Upem)
	putI16(out, 36, BMinX)
	putI16(out, 38, 0)
	putI16(out, 40, AMaxX)
	putI16(out, 42, AMaxY)
	putU16(out, 44, 0)
	putU16(out, 46, 8)
	putI16(out, 48, 2)
	putI16(out, 50, 0)
	putI16(out, 52, 0)
	return out
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func hheaTable() []byte {
	out := make([]byte, 36)
	putU32(out, 0, 0x00010000)
	putI16(out, 4, 800)
	putI16(out, 6, -200)
	putI16(out, 8, 0)
	putU16(out, 10, AdvanceA)
	putI16(out, 12, 0)
	putI16(out, 14, 0)
	putI16(out, 16, 600)
	putI16(out, 18, 1)
	putI16(out, 20, 0)
	putI16(out, 22, 0)
	putI16(out, 32, 0)
	putU16(out, 34, NumGlyphs)
	return out
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func maxpTable() []byte {
	out := make([]byte, 32)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, NumGlyphs)
	putU16(out, 6, 4)
	putU16(out, 8, 2)
	putU16(out, 10, 8)
	putU16(out, 12, 2)
	putU16(out, 14, 2)
	putU16(out, 16, 0)
	putU16(out, 28, 0)
	putU16(out, 30, 0)
	return out
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func hmtxTable() []byte {
	out := make([]byte, 16)
	putU16(out, 0, AdvanceNot)
	putI16(out, 2, 0)
	putU16(out, 4, AdvanceA)
	putI16(out, 6, AMinX)
	putU16(out, 8, AdvanceB)
	putI16(out, 10, BMinX)
	putU16(out, 12, AdvanceC)
	putI16(out, 14, AMinX)
	return out
}

// locaTable is the short-format location table: five offsets in halfwords,
// for .notdef, A, B, and C.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func locaTable() []byte {
	out := make([]byte, 10)
	putU16(out, 0, 0)
	putU16(out, 2, 0)
	putU16(out, 4, rectGlyphLen/2)
	putU16(out, 6, 2*rectGlyphLen/2)
	putU16(out, 8, (2*rectGlyphLen+compositeLen)/2)
	return out
}

const rectGlyphLen = 34

// compositeGlyphLen is the byte length of the C composite.
const compositeLen = glyfHeaderLen + 2*(compHeaderLen+compOffsetLen)

func glyfTable() []byte {
	out := make([]byte, 0, 2*rectGlyphLen+compositeLen)
	out = append(out, rectGlyph(AMinX, 0, AMaxX, AMaxY)...)
	out = append(out, rectGlyph(BMinX, 0, BMaxX, BMaxY)...)
	out = append(out, compositeGlyph()...)
	return out
}

// rectGlyph is one contour of four on-curve points.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func rectGlyph(minX, minY, maxX, maxY int16) []byte {
	out := make([]byte, rectGlyphLen)
	putI16(out, 0, 1)
	putI16(out, 2, minX)
	putI16(out, 4, minY)
	putI16(out, 6, maxX)
	putI16(out, 8, maxY)
	putU16(out, 10, 3)
	putU16(out, 12, 0)
	out[14], out[15], out[16], out[17] = 0x01, 0x01, 0x01, 0x01
	putI16(out, 18, minX)
	putI16(out, 20, maxX-minX)
	putI16(out, 22, 0)
	putI16(out, 24, minX-maxX)
	putI16(out, 26, minY)
	putI16(out, 28, 0)
	putI16(out, 30, maxY-minY)
	putI16(out, 32, 0)
	return out
}

// compositeGlyph is one composite record: component A first, component B
// last, both with word-sized x and y offsets of zero.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func compositeGlyph() []byte {
	out := make([]byte, compositeLen)
	putI16(out, 0, compositeContours)
	putI16(out, 2, BMinX)
	putI16(out, 4, 0)
	putI16(out, 6, AMaxX)
	putI16(out, 8, AMaxY)
	putComponent(out, glyfHeaderLen, GIDA, argWords|argsAreXY|moreComponents)
	putComponent(out, glyfHeaderLen+8, GIDB, argWords|argsAreXY)
	return out
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func putComponent(out []byte, offset, gid, flags int) {
	putU16(out, offset, flags)
	putU16(out, offset+2, gid)
	putI16(out, offset+4, 0)
	putI16(out, offset+6, 0)
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func cmapTable() []byte {
	sub := cmapFormat4()
	out := make([]byte, 12+len(sub))
	putU16(out, 0, 0)
	putU16(out, 2, 1)
	putU16(out, 4, 3)
	putU16(out, 6, 1)
	putU32(out, 8, 12)
	copy(out[12:], sub)
	return out
}

// cmapFormat4 maps A, B, and C to glyphs 1, 2, and 3.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func cmapFormat4() []byte {
	type segment struct {
		start uint16
		end   uint16
		delta uint16
	}
	segments := []segment{
		{start: CodeA, end: CodeC, delta: cmapDelta(CodeA, GIDA)},
		{start: 0xFFFF, end: 0xFFFF, delta: 1},
	}
	count := len(segments)
	out := make([]byte, 16+8*count)
	putU16(out, 0, 4)
	putU16(out, 2, len(out))
	putU16(out, 4, 0)
	putU16(out, 6, count*2)
	putU16(out, 8, 4)
	putU16(out, 10, 1)
	putU16(out, 12, 2)
	offset := 14
	for _, seg := range segments {
		putU16(out, offset, int(seg.end))
		offset += 2
	}
	putU16(out, offset, 0)
	offset += 2
	for _, seg := range segments {
		putU16(out, offset, int(seg.start))
		offset += 2
	}
	for _, seg := range segments {
		putU16(out, offset, int(seg.delta))
		offset += 2
	}
	return out
}

// cmapDelta returns the idDelta that maps one run of codes to one run of
// glyphs, so A, B, and C land on glyphs 1, 2, and 3.
func cmapDelta(code, gid uint16) uint16 {
	return gid - code
}

// postTable uses version 2 names from the standard Macintosh order: glyph A
// is index 36, B is 37, and C is 38.
//
//nolint:mnd // the sfnt table offsets and masks are the fixture data
func postTable() []byte {
	out := make([]byte, 44)
	putU32(out, 0, 0x00020000)
	putI32(out, 4, 0)
	putI16(out, 8, -100)
	putI16(out, 10, 50)
	putU16(out, 32, NumGlyphs)
	putU16(out, 34, 0)
	putU16(out, 36, 0)
	putU16(out, 38, 36)
	putU16(out, 40, 37)
	putU16(out, 42, 38)
	return out
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func putU16(data []byte, offset, value int) {
	binary.BigEndian.PutUint16(data[offset:], uint16(value&0xFFFF)) //nolint:gosec // the mask bounds the value to 16 bits
}

func putI16(data []byte, offset int, value int16) {
	binary.BigEndian.PutUint16(data[offset:], uint16(value)) //nolint:gosec // int16 and uint16 are both 16 bits
}

func putU32(data []byte, offset int, value uint32) {
	binary.BigEndian.PutUint32(data[offset:], value)
}

func putI32(data []byte, offset int, value int32) {
	binary.BigEndian.PutUint32(data[offset:], uint32(value)) //nolint:gosec // int32 and uint32 are both 32 bits
}

//nolint:mnd // the sfnt table offsets and masks are the fixture data
func putOffset(data []byte, offset, value int) {
	binary.BigEndian.PutUint32(data[offset:], uint32(value&0xFFFFFFFF)) //nolint:gosec // the mask bounds the conversion
}
