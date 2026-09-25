package font

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"golang.org/x/image/font/sfnt"
)

// TrueType composite glyph flags and the sfnt tables the subset needs.
const (
	compArgWords = 0x0001
	compMore     = 0x0020
	compScale    = 0x0008
	compXYScale  = 0x0040
	compTwoTwo   = 0x0080

	glyfHeader    = 10
	compRecordLen = 4
	compGlyphOff  = 2
	compositeSeed = 2
	locaLongEntry = 4
	locaShortHalf = 2
	shortLocaMax  = 0x1FFFE
	subsetTagLen  = 6
	subsetTagBase = 26
	maxpCountOff  = 4
	maxpCountEnd  = 6
)

var (
	// errSFNTNoGlyf reports an sfnt program with no TrueType outlines.
	errSFNTNoGlyf = errors.New("font: sfnt program has no glyf table")
	// errSubsetGlyph reports a glyph index outside the program.
	errSubsetGlyph = errors.New("font: subset glyph outside the program")
)

// SubsetTag returns the six uppercase letters a subset font takes from the
// SHA-256 digest of its program bytes. The digest carries no date, so two
// runs over equal bytes return the same tag.
func SubsetTag(program []byte) string {
	sum := sha256.Sum256(program)
	tag := make([]byte, subsetTagLen)
	for idx := range tag {
		tag[idx] = 'A' + sum[idx]%subsetTagBase
	}
	return string(tag)
}

// Subset returns a TrueType program with every glyph outside keep zeroed.
// Glyph indices do not change, so a cmap, a /CIDToGIDMap, a /Differences
// array, and a content stream stay valid without a re-encode. Glyph 0 stays,
// and the components of every kept composite glyph are kept too.
// The cmap, hmtx, and maxp tables are copied unchanged; loca is rebuilt; head
// gets a new indexToLocFormat when the rebuilt offsets need the long form.
// A program with no glyf table, a bad loca, or a glyph index outside the
// program returns an error and a nil program.
func Subset(program []byte, keep []sfnt.GlyphIndex) ([]byte, error) {
	tables, err := ParseTables(program)
	if err != nil {
		return nil, err
	}
	glyf, okGlyf := TableData(tables, "glyf")
	loca, okLoca := TableData(tables, "loca")
	head, okHead := TableData(tables, "head")
	maxp, okMaxp := TableData(tables, "maxp")
	if !okGlyf || !okLoca || !okHead || !okMaxp {
		return nil, errSFNTNoGlyf
	}
	numGlyphs, err := glyphCount(maxp)
	if err != nil {
		return nil, err
	}
	offsets, err := parseLoca(loca, head, numGlyphs, len(glyf))
	if err != nil {
		return nil, err
	}
	used, err := usedGlyphs(glyf, offsets, keep)
	if err != nil {
		return nil, err
	}
	newGlyf, newOffsets := zeroGlyphs(glyf, offsets, used)
	newLoca, long := encodeLoca(newOffsets)
	newHead := headClone(head)
	setLocaFormat(newHead, long)
	tables = putTable(tables, "glyf", newGlyf)
	tables = putTable(tables, "loca", newLoca)
	tables = putTable(tables, "head", newHead)
	return WriteTables(tables)
}

// glyphCount reads /maxp numGlyphs.
func glyphCount(maxp []byte) (int, error) {
	if len(maxp) < maxpCountEnd {
		return 0, errSFNTSyntax
	}
	count := int(binary.BigEndian.Uint16(maxp[maxpCountOff:]))
	if count < 1 {
		return 0, errSFNTSyntax
	}
	return count, nil
}

// parseLoca returns the numGlyphs+1 glyph offsets. The head indexToLocFormat
// entry chooses the short or long entry width.
func parseLoca(loca, head []byte, numGlyphs, glyfLen int) ([]int, error) {
	if len(head) < headLocField+2 {
		return nil, errSFNTSyntax
	}
	long := binary.BigEndian.Uint16(head[headLocField:]) != 0
	offsets := make([]int, numGlyphs+1)
	if long {
		if len(loca) < locaLongEntry*(numGlyphs+1) {
			return nil, errSFNTSyntax
		}
		for idx := range offsets {
			offsets[idx] = int(binary.BigEndian.Uint32(loca[locaLongEntry*idx:]))
		}
	} else {
		if len(loca) < locaShortHalf*(numGlyphs+1) {
			return nil, errSFNTSyntax
		}
		for idx := range offsets {
			offsets[idx] = locaShortHalf * int(binary.BigEndian.Uint16(loca[locaShortHalf*idx:]))
		}
	}
	for idx := 1; idx <= numGlyphs; idx++ {
		if offsets[idx] < offsets[idx-1] || offsets[idx] > glyfLen {
			return nil, errSFNTSyntax
		}
	}
	return offsets, nil
}

// glyphData returns the glyf bytes of one glyph. A glyph outside the table
// returns an empty slice.
func glyphData(glyf []byte, offsets []int, gid int) []byte {
	start := offsets[gid]
	end := offsets[gid+1]
	return glyf[start:end]
}

// usedGlyphs returns the kept glyph set: glyph 0, every requested glyph, and
// every component a kept composite glyph references, followed transitively.
func usedGlyphs(glyf []byte, offsets []int, keep []sfnt.GlyphIndex) (map[int]bool, error) {
	count := len(offsets) - 1
	used := map[int]bool{0: true}
	queue := make([]int, 0, len(keep))
	add := func(gid int) {
		if gid >= 0 && gid < count && !used[gid] {
			used[gid] = true
			queue = append(queue, gid)
		}
	}
	for _, gid := range keep {
		if int(gid) >= count {
			return nil, errSubsetGlyph
		}
		add(int(gid))
	}
	for len(queue) > 0 {
		gid := queue[0]
		queue = queue[1:]
		for _, ref := range compositeRefs(glyphData(glyf, offsets, gid)) {
			add(ref)
		}
	}
	return used, nil
}

// compositeRefs returns the component glyph indices of one composite glyph.
// A simple glyph, a truncated record, and a malformed flag run stop the walk.
func compositeRefs(data []byte) []int {
	//nolint:gosec // the signed contour count is the composite marker
	if len(data) < glyfHeader || int16(binary.BigEndian.Uint16(data)) >= 0 {
		return nil
	}
	refs := make([]int, 0, compositeSeed)
	pos := glyfHeader
	for pos+compRecordLen <= len(data) {
		flags := binary.BigEndian.Uint16(data[pos:])
		refs = append(refs, int(binary.BigEndian.Uint16(data[pos+compGlyphOff:])))
		pos += compRecordLen
		next, ok := componentEnd(flags, pos, len(data))
		if !ok {
			return refs
		}
		pos = next
		if flags&compMore == 0 {
			return refs
		}
	}
	return refs
}

// componentEnd skips one component record's arguments and scale and returns
// the position of the next record.
func componentEnd(flags uint16, pos, end int) (int, bool) {
	if flags&compArgWords != 0 {
		pos += 4
	} else {
		pos += 2
	}
	switch {
	case flags&compScale != 0:
		pos += 2
	case flags&compXYScale != 0:
		pos += 4
	case flags&compTwoTwo != 0:
		pos += 8
	}
	return pos, pos <= end
}

// zeroGlyphs copies every kept glyph and drops the rest, then returns the new
// glyf table and the numGlyphs+1 offsets that describe it.
func zeroGlyphs(glyf []byte, offsets []int, used map[int]bool) ([]byte, []int) {
	count := len(offsets) - 1
	out := make([]byte, 0, len(glyf))
	newOffsets := make([]int, count+1)
	for gid := range count {
		newOffsets[gid] = len(out)
		if !used[gid] {
			continue
		}
		out = append(out, glyphData(glyf, offsets, gid)...)
	}
	newOffsets[count] = len(out)
	return out, newOffsets
}

// encodeLoca writes the location table and reports whether the long form was
// needed. The short form stores each offset halved, so an odd offset or one
// past the 16-bit halfword limit promotes the table.
func encodeLoca(offsets []int) ([]byte, bool) {
	long := false
	for _, offset := range offsets {
		if offset%2 != 0 || offset > shortLocaMax {
			long = true
			break
		}
	}
	if !long {
		out := make([]byte, locaShortHalf*len(offsets))
		for idx, offset := range offsets {
			//nolint:gosec // the shortLocaMax guard bounds the value
			binary.BigEndian.PutUint16(out[locaShortHalf*idx:], uint16(offset/locaShortHalf))
		}
		return out, false
	}
	out := make([]byte, locaLongEntry*len(offsets))
	for idx, offset := range offsets {
		binary.BigEndian.PutUint32(out[locaLongEntry*idx:], uint32(offset)) //nolint:gosec // a glyf table cannot pass 4 GiB
	}
	return out, true
}

// headClone copies the head table so the subset can edit indexToLocFormat.
func headClone(head []byte) []byte {
	out := make([]byte, len(head))
	copy(out, head)
	return out
}

// setLocaFormat writes head indexToLocFormat.
func setLocaFormat(head []byte, long bool) {
	if len(head) < headLocField+2 {
		return
	}
	value := uint16(0)
	if long {
		value = 1
	}
	binary.BigEndian.PutUint16(head[headLocField:], value)
}
