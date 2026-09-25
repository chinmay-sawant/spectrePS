package pdf

import (
	"encoding/binary"
	"testing"
)

// The font and text tests embed a synthetic TrueType program instead of a
// third-party font file. The program is built here, so no font bytes are
// checked in and no license travels with the tests.
//
// The font has three glyphs: .notdef, A, and B. A is a rectangle from
// (100, 0) to (600, 700); B is a rectangle from (50, 0) to (350, 300).
// Glyph A advances 600 units and glyph B advances 400 units, both per 1000
// units per em.
const (
	u16Mask         = 0xFFFF
	u32Mask         = 0xFFFFFFFF
	synthUpem       = 1000
	synthGlyphA     = 1
	synthGlyphB     = 2
	synthAdvanceA   = 600
	synthAdvanceB   = 400
	synthNotdefAdv  = 500
	synthTableCount = 8
)

// synthFont returns the TrueType program.
func synthFont() []byte {
	builder := &sfntBuilder{}
	builder.add("cmap", synthCmap())
	builder.add("glyf", synthGlyf())
	builder.add("head", synthHead())
	builder.add("hhea", synthHhea())
	builder.add("hmtx", synthHmtx())
	builder.add("loca", synthLoca())
	builder.add("maxp", synthMaxp())
	builder.add("post", synthPost())
	return builder.bytes()
}

type sfntTable struct {
	tag  string
	data []byte
}

type sfntBuilder struct {
	tables []sfntTable
}

func (builder *sfntBuilder) add(tag string, data []byte) {
	builder.tables = append(builder.tables, sfntTable{tag: tag, data: data})
}

// bytes writes the font. The table directory is sorted by tag and every table
// is padded to four bytes. x/image/font/sfnt ignores checksums, but the
// builder writes them and the head adjustment so the fixture stays valid.
func (builder *sfntBuilder) bytes() []byte {
	count := len(builder.tables)
	searchRange := 16
	entrySelector := 0
	for searchRange*2 <= 16*count {
		searchRange *= 2
		entrySelector++
	}
	rangeShift := 16*count - searchRange
	total := 12 + 16*count
	offsets := make([]int, count)
	for idx, table := range builder.tables {
		offsets[idx] = total
		total += padded4(len(table.data))
	}
	out := make([]byte, total)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, count)
	putU16(out, 6, searchRange)
	putU16(out, 8, entrySelector)
	putU16(out, 10, rangeShift)
	for idx, table := range builder.tables {
		entry := out[12+16*idx:]
		copy(entry[0:4], table.tag)
		putU32(entry, 4, tableChecksum(table.data))
		putOffset(entry, 8, offsets[idx])
		putOffset(entry, 12, len(table.data))
		copy(out[offsets[idx]:], table.data)
	}
	adjust := uint32(0xB1B0AFBA) - tableChecksum(out)
	for idx, table := range builder.tables {
		if table.tag == "head" {
			putU32(out, offsets[idx]+8, adjust)
		}
	}
	return out
}

func padded4(size int) int {
	return (size + 3) &^ 3
}

func tableChecksum(data []byte) uint32 {
	var sum uint32
	for idx := 0; idx < len(data); idx += 4 {
		var word [4]byte
		copy(word[:], data[idx:min(idx+4, len(data))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}

func synthHead() []byte {
	out := make([]byte, 54)
	putU32(out, 0, 0x00010000)
	putU32(out, 4, 0x00010000)
	putU32(out, 12, 0x5F0F3CF5)
	putU16(out, 16, 0)
	putU16(out, 18, synthUpem)
	putI16(out, 36, 100)
	putI16(out, 38, 0)
	putI16(out, 40, 600)
	putI16(out, 42, 700)
	putU16(out, 44, 0)
	putU16(out, 46, 8)
	putI16(out, 48, 2)
	putI16(out, 50, 0)
	putI16(out, 52, 0)
	return out
}

func synthHhea() []byte {
	out := make([]byte, 36)
	putU32(out, 0, 0x00010000)
	putI16(out, 4, 800)
	putI16(out, 6, -200)
	putI16(out, 8, 0)
	putU16(out, 10, synthAdvanceA)
	putI16(out, 12, 0)
	putI16(out, 14, 0)
	putI16(out, 16, 600)
	putI16(out, 18, 1)
	putI16(out, 20, 0)
	putI16(out, 22, 0)
	putI16(out, 32, 0)
	putU16(out, 34, 3)
	return out
}

func synthMaxp() []byte {
	out := make([]byte, 32)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, 3)
	putU16(out, 6, 4)
	putU16(out, 8, 1)
	putU16(out, 10, 0)
	putU16(out, 12, 0)
	putU16(out, 14, 2)
	return out
}

func synthHmtx() []byte {
	out := make([]byte, 12)
	putU16(out, 0, synthNotdefAdv)
	putI16(out, 2, 0)
	putU16(out, 4, synthAdvanceA)
	putI16(out, 6, 100)
	putU16(out, 8, synthAdvanceB)
	putI16(out, 10, 50)
	return out
}

func synthLoca() []byte {
	out := make([]byte, 8)
	putU16(out, 0, 0)
	putU16(out, 2, 0)
	putU16(out, 4, 17)
	putU16(out, 6, 34)
	return out
}

func synthGlyf() []byte {
	var out []byte
	out = append(out, rectGlyph(100, 0, 600, 700)...)
	out = append(out, rectGlyph(50, 0, 350, 300)...)
	return out
}

// rectGlyph is one contour of four on-curve points.
func rectGlyph(minX, minY, maxX, maxY int16) []byte {
	out := make([]byte, 34)
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

func synthCmap() []byte {
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

// cmapFormat4 maps A to glyph 1, B to glyph 2, and 0xFFFF to .notdef.
func cmapFormat4() []byte {
	type segment struct {
		start uint16
		end   uint16
		delta uint16
	}
	segments := []segment{
		{start: 'A', end: 'A', delta: cmapDelta('A', 1)},
		{start: 'B', end: 'B', delta: cmapDelta('B', 2)},
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

// cmapDelta returns the idDelta that maps one code to one glyph.
func cmapDelta(code, gid uint16) uint16 {
	return gid - code
}

// synthPost uses version 2 names. Indices 36 and 37 in the standard
// Macintosh order are A and B.
func synthPost() []byte {
	out := make([]byte, 40)
	putU32(out, 0, 0x00020000)
	putI32(out, 4, 0)
	putI16(out, 8, -100)
	putI16(out, 10, 50)
	putU16(out, 32, 3)
	putU16(out, 34, 0)
	putU16(out, 36, 36)
	putU16(out, 38, 37)
	return out
}

func putU16(data []byte, offset, value int) {
	binary.BigEndian.PutUint16(data[offset:], uint16(value&u16Mask)) //nolint:gosec // the mask bounds the value to 16 bits
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

// putOffset writes a file offset or length. The mask bounds the conversion.
func putOffset(data []byte, offset, value int) {
	binary.BigEndian.PutUint32(data[offset:], uint32(value&u32Mask)) //nolint:gosec // the mask bounds the value to 32 bits
}

// synthFontStream wraps the program in a Flate stream body for a font dict.
func synthFontStream(t *testing.T, extra string) string {
	t.Helper()
	return streamBody("/Filter /FlateDecode "+extra, flateRaw(t, synthFont()))
}

// synthTextPage is one page whose /F1 is the synthetic embedded TrueType font
// and whose content stream is content.
func synthTextPage(t *testing.T, content string) *File {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(streamBody("", []byte(content)))
	doc.object("<< /Type /Font /Subtype /TrueType /BaseFont /Synth " +
		"/FirstChar 65 /Widths [600] /FontDescriptor 6 0 R >>")
	doc.object("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>")
	doc.object(synthFontStream(t, ""))
	return mustOpen(t, doc.classic(""))
}
