package pdf

import (
	"math"
	"strconv"
)

const (
	offsetDigits   = 10
	xrefWidthCount = 3
	byteShift      = 8

	rowFree   = 0
	rowPlain  = 1
	rowPacked = 2
)

// XEntry is one xref row.
// Compressed rows point at an object stream number and an index inside it.
type XEntry struct {
	Offset     int
	Gen        int
	InUse      bool
	Compressed bool
	StreamNum  int
	StreamIdx  int
}

type xrefCursor struct {
	src []byte
	pos int
}

type rowReader struct {
	body     []byte
	widths   []int
	rowWidth int
	entries  map[int]XEntry
	pos      int
}

func blankEntry() XEntry {
	return XEntry{
		Offset: 0, Gen: 0, InUse: false, Compressed: false, StreamNum: 0, StreamIdx: 0,
	}
}

func freeEntry(offset, gen int) XEntry {
	return XEntry{
		Offset: offset, Gen: gen, InUse: false, Compressed: false, StreamNum: 0, StreamIdx: 0,
	}
}

func plainEntry(offset, gen int) XEntry {
	return XEntry{
		Offset: offset, Gen: gen, InUse: true, Compressed: false, StreamNum: 0, StreamIdx: 0,
	}
}

func packedEntry(streamNum, streamIdx int) XEntry {
	return XEntry{
		Offset: 0, Gen: 0, InUse: true, Compressed: true, StreamNum: streamNum, StreamIdx: streamIdx,
	}
}

func xrefSyntax() error {
	return NewError(opXRef, errSyntax)
}

// ParseClassic reads a classic xref table. offset points at the x of xref.
// The map is keyed by object number. next is the offset of the next token, usually trailer.
// Accept both 20-byte entries and lines that still contain a 10-digit offset, a generation, and n or f.
func ParseClassic(src []byte, offset int) (map[int]XEntry, int, error) {
	if offset < 0 || offset > len(src) {
		return nil, 0, xrefSyntax()
	}
	cur := &xrefCursor{src: src, pos: offset}
	if err := cur.matchKeyword(opXRef); err != nil {
		return nil, 0, err
	}
	entries := make(map[int]XEntry)
	for {
		cur.skipSpace()
		if cur.pos >= len(cur.src) || !isDigit(cur.src[cur.pos]) {
			return entries, cur.pos, nil
		}
		if err := cur.readSubsection(entries); err != nil {
			return nil, 0, err
		}
	}
}

func (cur *xrefCursor) matchKeyword(word string) error {
	end := cur.pos + len(word)
	if end > len(cur.src) || string(cur.src[cur.pos:end]) != word {
		return xrefSyntax()
	}
	if end < len(cur.src) && !isPDFSpace(cur.src[end]) {
		return xrefSyntax()
	}
	cur.pos = end
	return nil
}

func (cur *xrefCursor) readSubsection(entries map[int]XEntry) error {
	start, err := cur.readInt()
	if err != nil {
		return err
	}
	count, err := cur.readInt()
	if err != nil {
		return err
	}
	if count > 0 && start > math.MaxInt-count {
		return xrefSyntax()
	}
	for range count {
		entry, readErr := cur.readEntry()
		if readErr != nil {
			return readErr
		}
		entries[start] = entry
		start++
	}
	return nil
}

func (cur *xrefCursor) readEntry() (XEntry, error) {
	cur.skipSpace()
	offset, err := cur.readFixedDigits(offsetDigits)
	if err != nil {
		return blankEntry(), err
	}
	gen, err := cur.readInt()
	if err != nil {
		return blankEntry(), err
	}
	inUse, err := cur.readUseFlag()
	if err != nil {
		return blankEntry(), err
	}
	if inUse {
		return plainEntry(offset, gen), nil
	}
	return freeEntry(offset, gen), nil
}

func (cur *xrefCursor) readUseFlag() (bool, error) {
	cur.skipSpace()
	if cur.pos >= len(cur.src) {
		return false, xrefSyntax()
	}
	flag := cur.src[cur.pos]
	cur.pos++
	switch flag {
	case 'n':
		return true, nil
	case 'f':
		return false, nil
	default:
		return false, xrefSyntax()
	}
}

func (cur *xrefCursor) readFixedDigits(width int) (int, error) {
	end := cur.pos + width
	if cur.pos < 0 || width < 1 || end < cur.pos || end > len(cur.src) {
		return 0, xrefSyntax()
	}
	chunk := cur.src[cur.pos:end]
	for _, digit := range chunk {
		if !isDigit(digit) {
			return 0, xrefSyntax()
		}
	}
	number, err := strconv.Atoi(string(chunk))
	if err != nil {
		return 0, xrefSyntax()
	}
	cur.pos = end
	return number, nil
}

func (cur *xrefCursor) readInt() (int, error) {
	cur.skipSpace()
	if cur.pos >= len(cur.src) || !isDigit(cur.src[cur.pos]) {
		return 0, xrefSyntax()
	}
	start := cur.pos
	for cur.pos < len(cur.src) && isDigit(cur.src[cur.pos]) {
		cur.pos++
	}
	if cur.pos-start > offsetDigits {
		return 0, xrefSyntax()
	}
	number, err := strconv.Atoi(string(cur.src[start:cur.pos]))
	if err != nil {
		return 0, xrefSyntax()
	}
	return number, nil
}

func (cur *xrefCursor) skipSpace() {
	for cur.pos < len(cur.src) && isPDFSpace(cur.src[cur.pos]) {
		cur.pos++
	}
}

func isPDFSpace(one byte) bool {
	switch one {
	case ' ', '\t', '\n', '\r', '\f', 0:
		return true
	default:
		return false
	}
}

// ParseStreamRows reads a decoded xref stream body.
// widths is /W. size is /Size. indexPairs is /Index as start,count pairs.
// A nil or empty indexPairs means object 0 for size entries.
// A field width of 0 uses the default: type 1 when the first field is absent, generation 0 when the third is absent.
// Type 0 is free. Type 1 is an uncompressed byte offset.
// Type 2 sets Compressed, StreamNum from field 2, and StreamIdx from field 3.
// Multi-byte integers are big-endian.
// A short body or a bad width returns NewError("xref", "syntaxerror").
func ParseStreamRows(body []byte, widths []int, size int, indexPairs []int) (map[int]XEntry, error) {
	rowWidth, err := rowByteWidth(widths)
	if err != nil {
		return nil, err
	}
	pairs, err := indexPairsOrSize(size, indexPairs)
	if err != nil {
		return nil, err
	}
	rows := &rowReader{
		body: body, widths: widths, rowWidth: rowWidth, entries: make(map[int]XEntry), pos: 0,
	}
	for pair := 0; pair < len(pairs); pair += 2 {
		if err := rows.readSpan(pairs[pair], pairs[pair+1]); err != nil {
			return nil, err
		}
	}
	return rows.entries, nil
}

func rowByteWidth(widths []int) (int, error) {
	if len(widths) != xrefWidthCount {
		return 0, xrefSyntax()
	}
	total := 0
	for _, width := range widths {
		if width < 0 || total > math.MaxInt-width {
			return 0, xrefSyntax()
		}
		total += width
	}
	return total, nil
}

func indexPairsOrSize(size int, indexPairs []int) ([]int, error) {
	if len(indexPairs) == 0 {
		if size < 0 {
			return nil, xrefSyntax()
		}
		return []int{0, size}, nil
	}
	if len(indexPairs)%2 != 0 {
		return nil, xrefSyntax()
	}
	for _, number := range indexPairs {
		if number < 0 {
			return nil, xrefSyntax()
		}
	}
	return indexPairs, nil
}

func (rows *rowReader) readSpan(start, count int) error {
	if count > 0 && start > math.MaxInt-count {
		return xrefSyntax()
	}
	for range count {
		if rows.pos > len(rows.body)-rows.rowWidth {
			return xrefSyntax()
		}
		entry, next, err := readOneRow(rows.body, rows.widths, rows.pos)
		if err != nil {
			return err
		}
		rows.entries[start] = entry
		start++
		rows.pos = next
	}
	return nil
}

func readOneRow(body []byte, widths []int, pos int) (XEntry, int, error) {
	kind, pos, err := readTypedField(body, pos, widths[0], rowPlain)
	if err != nil {
		return blankEntry(), 0, err
	}
	second, pos, err := readTypedField(body, pos, widths[1], 0)
	if err != nil {
		return blankEntry(), 0, err
	}
	third, pos, err := readTypedField(body, pos, widths[2], 0)
	if err != nil {
		return blankEntry(), 0, err
	}
	entry, err := entryFromRow(kind, second, third)
	if err != nil {
		return blankEntry(), 0, err
	}
	return entry, pos, nil
}

func readTypedField(body []byte, pos, width, fallback int) (int, int, error) {
	if width == 0 {
		return fallback, pos, nil
	}
	return readBigEndian(body, pos, width)
}

func readBigEndian(body []byte, pos, width int) (int, int, error) {
	if err := checkSpan(pos, width, len(body)); err != nil {
		return 0, pos, err
	}
	number := 0
	for _, one := range body[pos : pos+width] {
		if number > (math.MaxInt >> byteShift) {
			return 0, pos, xrefSyntax()
		}
		number = (number << byteShift) | int(one)
	}
	return number, pos + width, nil
}

func checkSpan(pos, width, length int) error {
	if width < 0 || pos < 0 || pos > length || width > length-pos {
		return xrefSyntax()
	}
	return nil
}

func entryFromRow(kind, second, third int) (XEntry, error) {
	switch kind {
	case rowFree:
		return freeEntry(second, third), nil
	case rowPlain:
		return plainEntry(second, third), nil
	case rowPacked:
		return packedEntry(second, third), nil
	default:
		return blankEntry(), xrefSyntax()
	}
}
