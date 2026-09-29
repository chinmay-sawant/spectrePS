package pdf

import "strconv"

// A document whose cross-reference table does not name the objects the file
// carries cannot be read through that table. Ghostscript reports the damage
// and rebuilds from the object headers, and this file is that rebuild, in the
// four shapes the batch2 corpus asked for:
//
//   - The startxref is missing, unreadable, or names nothing: the trailer and
//     the object headers are the whole document, so the rebuild runs.
//   - The section at the startxref is damaged, but its rows sit on the
//     specification's 20-byte grid: only individual fields are hurt, so the
//     grid reader keeps the row numbering and the damaged rows fall back to
//     the object headers.
//   - A row names the wrong bytes, or a referenced number has no row at all:
//     the object header, then the object streams, decide where the object is.
//   - A stream without a usable /Length reads to its endstream keyword.
//
// Every one of them needs a trailer carrying a /Root, and three pinned corpus
// refusals stay refused:
//
//   - structural/bad-xref.pdf names a classic table whose rows do not line up
//     with the 20-byte grid, so the mapping from row to object number is lost
//     and the reader reports the damage.
//   - images/UnknownFilter-xrefstm.pdf names an xref stream this reader cannot
//     decode, so the offset presents a section and its filter error stands.
//   - structural/parser_rebuildxref_error_notrailer.pdf has no trailer /Root
//     anywhere, so the scan finds objects and the reader still refuses.
//
// A file whose newest section reads but whose /Prev chain is broken, or whose
// startxref names a valid section, keeps its failure even when a table
// elsewhere in the same file would have parsed. That is the difference from a
// backward scan for a newer table, which lets a broken newest section through.

// recoverCrossRef rebuilds the cross-reference table and trailer for src, whose
// startxref offset is offset. It reports false when the recovery does not apply
// or does not produce a document, and the caller keeps its own error.
func recoverCrossRef(src []byte, offset int) (map[int]XEntry, Value, bool) {
	if xrefSectionAt(src, offset) {
		// The offset presents a section. When that section itself reads, the
		// caller's failure is in its /Prev chain: a cycle, a chain past the
		// limit, or a damaged older section. A broken chain is not a damaged
		// table, and it keeps its error, which structural/bug_xrefv4_loop.pdf
		// pins.
		if _, _, err := readXRefSection(src, offset); err == nil {
			return nil, NullVal(), false
		}
		// The section at the offset is damaged. When its rows sit on the
		// specification's 20-byte grid, only individual fields are hurt and
		// the section is still usable; a table whose rows do not line up
		// loses the mapping from row slot to object number, and its refusal
		// stands.
		return recoverClassicGrid(src, offset)
	}
	if entries, trailer, ok := recoverStreamXRef(src); ok {
		return entries, trailer, true
	}
	entries := scanObjectHeaders(src)
	if len(entries) == 0 {
		return nil, NullVal(), false
	}
	trailer, ok := lastRootTrailer(src)
	if !ok {
		return nil, NullVal(), false
	}
	return entries, trailer, true
}

// recoverClassicGrid re-reads a classic section at offset against the strict
// 20-byte row grid, then the trailer that follows it. It reports false unless
// a row parsed and the trailer carries a /Root, so a table whose rows never
// sit on the grid still refuses, and so do the two pinned classic shapes:
// structural/bad-xref.pdf has rows that do not line up, and
// parser_rebuildxref_error_notrailer.pdf has no trailer /Root.
//
// A row whose fields are damaged leaves its number out of use, and the
// per-object recovery then reads that object from the file's own header,
// which is the rebuild Ghostscript performs for a damaged entry.
func recoverClassicGrid(src []byte, offset int) (map[int]XEntry, Value, bool) {
	entries, next, err := parseClassicGrid(src, offset)
	if err != nil {
		return nil, NullVal(), false
	}
	trailer, err := trailerDict(src, next)
	if err != nil {
		return nil, NullVal(), false
	}
	if _, ok := trailer.ValueEntry(keyRoot); !ok {
		return nil, NullVal(), false
	}
	return entries, trailer, true
}

// Classic row geometry: ten digits of offset, a space, five digits of
// generation, a space, the use flag, and a two-byte end of line.
const (
	gridRowLen  = 20
	gridFlagAt  = 17
	gridGenFrom = 11
	gridGenTo   = 16
	gridEOLFrom = 18
)

// parseClassicGrid reads a classic section at offset with the row geometry the
// specification fixes. Each declared row is read from its own 20-byte slot, so
// a damaged field fails one row instead of shifting every row after it. A
// parsed row is the only evidence the grid is real: a section whose rows do
// not sit on the grid is not this reader's to recover, and reports false.
func parseClassicGrid(src []byte, offset int) (map[int]XEntry, int, error) {
	if offset < 0 || offset > len(src) {
		return nil, 0, xrefSyntax()
	}
	cur := &xrefCursor{src: src, pos: offset}
	if err := cur.matchKeyword(opXRef); err != nil {
		return nil, 0, err
	}
	entries := make(map[int]XEntry)
	parsed, err := cur.readGridSections(entries)
	if err != nil {
		return nil, 0, err
	}
	if !parsed {
		// No row on the grid: the section's row numbering is not readable, so
		// nothing here is trustworthy. The caller keeps its own error.
		return nil, 0, xrefSyntax()
	}
	return entries, cur.pos, nil
}

// readGridSections walks every subsection of a classic section. It stops at
// the first token that is not a subsection header, which is where the trailer
// dictionary begins, and reports whether any row parsed.
func (cur *xrefCursor) readGridSections(entries map[int]XEntry) (bool, error) {
	parsed := false
	for {
		cur.skipSpace()
		if cur.pos >= len(cur.src) || !isDigit(cur.src[cur.pos]) {
			return parsed, nil
		}
		start, count, err := cur.readGridHeader()
		if err != nil {
			return false, err
		}
		origin := skipSpace(cur.src, cur.pos)
		spanParsed, err := gridSpan(cur.src, entries, start, count, origin)
		if err != nil {
			return false, err
		}
		parsed = parsed || spanParsed
		next, extraParsed := gridExtras(cur.src, entries, start, count, origin+count*gridRowLen)
		parsed = parsed || extraParsed
		cur.pos = next
	}
}

// readGridHeader reads the "start count" pair of one subsection.
func (cur *xrefCursor) readGridHeader() (int, int, error) {
	start, err := cur.readInt()
	if err != nil {
		return 0, 0, err
	}
	count, err := cur.readInt()
	if err != nil {
		return 0, 0, err
	}
	return start, count, nil
}

// gridSpan reads the declared rows of one subsection from their own grid
// slots. A damaged row leaves its number out of use, and the caller's
// per-object recovery reads that object from the file's own header.
func gridSpan(src []byte, entries map[int]XEntry, start, count, origin int) (bool, error) {
	parsed := false
	for i := range count {
		at := origin + i*gridRowLen
		if at < 0 || at+gridRowLen > len(src) {
			return false, xrefSyntax()
		}
		entry, ok := gridRow(src, at)
		if !ok {
			entries[start+i] = blankEntry()
			continue
		}
		entries[start+i] = entry
		parsed = true
	}
	return parsed, nil
}

// gridExtras reads rows that follow a subsection's declared count. The token
// after a row's offset and generation is a use flag, which a subsection header
// never carries, so a well-formed row here is a row the producer miscounted,
// not a new subsection. It returns the position after the last one.
func gridExtras(src []byte, entries map[int]XEntry, start, count, from int) (int, bool) {
	parsed := false
	for {
		entry, ok := gridRow(src, from)
		if !ok {
			return from, parsed
		}
		entries[start+count] = entry
		start++
		count++
		from += gridRowLen
		parsed = true
	}
}

// gridRow reads one classic row from its 20-byte slot. Both field separators,
// the flag, and the end of line have to sit exactly where the specification
// puts them, and a damaged row reports false.
func gridRow(src []byte, at int) (XEntry, bool) {
	if at < 0 || at+gridRowLen > len(src) {
		return blankEntry(), false
	}
	row := src[at : at+gridRowLen]
	if !gridRowShape(row) {
		return blankEntry(), false
	}
	offset, ok := gridInt(row[:10])
	if !ok {
		return blankEntry(), false
	}
	gen, ok := gridInt(row[gridGenFrom:gridGenTo])
	if !ok {
		return blankEntry(), false
	}
	switch row[gridFlagAt] {
	case 'n':
		return plainEntry(offset, gen), true
	case 'f':
		return freeEntry(offset, gen), true
	default:
		return blankEntry(), false
	}
}

// gridRowShape reports whether the separators and the end of line of one
// 20-byte slot are where the specification puts them.
func gridRowShape(row []byte) bool {
	if row[10] != ' ' || row[16] != ' ' {
		return false
	}
	for pos := gridEOLFrom; pos < gridRowLen; pos++ {
		if !isPDFSpace(row[pos]) {
			return false
		}
	}
	return true
}

// gridInt parses a run of ASCII digits.
func gridInt(field []byte) (int, bool) {
	if len(field) == 0 {
		return 0, false
	}
	for _, digit := range field {
		if !isDigit(digit) {
			return 0, false
		}
	}
	number, err := strconv.Atoi(string(field))
	if err != nil {
		return 0, false
	}
	return number, true
}

// recoverStreamXRef reads every cross-reference stream the file carries, newest
// first, and fills the gaps from the object headers. A document whose startxref
// names nothing often still has whole xref streams elsewhere, and those streams
// are the file's own tables: they carry the trailer /Root and the rows for
// compressed objects the header scan cannot see. Newest section wins per
// object number, exactly as a /Prev chain would merge them.
func recoverStreamXRef(src []byte) (map[int]XEntry, Value, bool) {
	ats := xrefStreamOffsets(src)
	if len(ats) == 0 {
		return nil, NullVal(), false
	}
	var entries map[int]XEntry
	trailer := NullVal()
	for i := len(ats) - 1; i >= 0; i-- {
		section, sectionTrailer, err := readStreamXRef(src, ats[i])
		if err != nil {
			continue
		}
		if entries == nil {
			entries, trailer = section, sectionTrailer
			continue
		}
		fillEntries(entries, section)
		trailer = mergeTrailer(trailer, sectionTrailer)
	}
	if entries == nil {
		return nil, NullVal(), false
	}
	fillEntries(entries, scanObjectHeaders(src))
	if _, ok := trailer.ValueEntry(keyRoot); !ok {
		return nil, NullVal(), false
	}
	return entries, trailer, true
}

// xrefStreamOffsets returns the offset of every indirect object whose value is
// a cross-reference stream, in file order. The last one in the file is the
// newest, the same order an incremental update writes. The walk skips the body
// of every object it parses, so a byte pattern inside a stream is not a
// candidate.
func xrefStreamOffsets(src []byte) []int {
	var ats []int
	for pos := 0; pos < len(src); {
		num, gen, ok := objectHeaderAt(src, pos)
		if !ok {
			pos++
			continue
		}
		got, gotGen, val, next, err := ParseIndirect(src, pos)
		if err != nil || got != num || gotGen != gen || num == 0 {
			pos++
			continue
		}
		if xrefStream(val) {
			ats = append(ats, pos)
		}
		if next > pos {
			pos = next
			continue
		}
		pos++
	}
	return ats
}

// xrefSectionAt reports whether readXRefSection would read a cross-reference
// section at offset. The keyword test mirrors the classic dispatch, and the
// stream test requires the indirect object there to be a cross-reference
// stream, which is the only shape readStreamXRef accepts. Both conditions hold
// three pinned corpus refusals in place:
//   - structural/bad-xref.pdf names a classic table whose rows are damaged, so
//     the keyword is there and its parse error is reported.
//   - images/UnknownFilter-xrefstm.pdf names a cross-reference stream this
//     reader cannot decode, so the offset presents one and the reader reports
//     the unknown filter.
//   - structural/parser_rebuildxref_error_notrailer.pdf has no trailer /Root,
//     so the scan finds objects and the reader still refuses.
//
// An ordinary indirect object at the offset is not a section, so recovery is
// free to rebuild the table there.
func xrefSectionAt(src []byte, offset int) bool {
	if offset < 0 || offset >= len(src) {
		return false
	}
	pos := skipSpace(src, offset)
	if pos >= len(src) {
		return false
	}
	if hasKeyword(src, pos, wordXRef) {
		return true
	}
	return xrefStreamParses(src, pos)
}

// xrefStreamParses reports whether an indirect cross-reference stream parses at
// pos. It exists so the five-value ParseIndirect result does not become a
// dogsled at the call site, where only the shape matters.
func xrefStreamParses(src []byte, pos int) bool {
	number, generation, body, _, err := ParseIndirect(src, pos)
	if err != nil {
		return false
	}
	return number >= 0 && generation >= 0 && xrefStream(body)
}

// scanObjectHeaders walks src for "num gen obj" headers and records the offset
// of the first complete object for each number. A candidate has to parse end to
// end to count, and a false positive from bytes inside a stream body is far
// likelier to follow the real object than to precede it, so the first one wins.
// The scan then resumes after the object it just read, which keeps the body of
// every stream it accepts out of the search.
func scanObjectHeaders(src []byte) map[int]XEntry {
	entries := map[int]XEntry{}
	for pos := 0; pos < len(src); {
		num, gen, ok := objectHeaderAt(src, pos)
		if !ok {
			pos++
			continue
		}
		got, gotGen, _, next, err := ParseIndirect(src, pos)
		if err != nil || got != num || gotGen != gen || num == 0 {
			pos++
			continue
		}
		if _, seen := entries[num]; !seen {
			entries[num] = plainEntry(pos, gen)
		}
		if next > pos {
			pos = next
			continue
		}
		pos++
	}
	return entries
}

// objectHeaderAt reports the object number, the generation, and whether src[pos:]
// is "num gen obj" with pos at the first digit of the number. Object number zero
// never counts, so the free head of a table is not a row.
func objectHeaderAt(src []byte, pos int) (int, int, bool) {
	if pos >= len(src) || !isDigit(src[pos]) {
		return 0, 0, false
	}
	if pos > 0 && isDigit(src[pos-1]) {
		return 0, 0, false
	}
	num, next, ok := pdfInt(src, pos)
	if !ok || num <= 0 {
		return 0, 0, false
	}
	gen, after, ok := pdfInt(src, next)
	if !ok || gen < 0 {
		return 0, 0, false
	}
	if !hasKeyword(src, skipSpace(src, after), wordObj) {
		return 0, 0, false
	}
	return num, gen, true
}

// lastRootTrailer returns the last trailer dictionary that carries a /Root. The
// last one wins because an incremental update writes its trailer last.
func lastRootTrailer(src []byte) (Value, bool) {
	best := NullVal()
	found := false
	for pos := 0; pos+len(wordTrailer) <= len(src); pos++ {
		if !keywordHere(src, pos, wordTrailer) {
			continue
		}
		val, ok := trailerValueAt(src, pos+len(wordTrailer))
		if !ok {
			continue
		}
		best = val
		found = true
	}
	return best, found
}

func trailerValueAt(src []byte, from int) (Value, bool) {
	pos := skipSpace(src, from)
	if pos >= len(src) || src[pos] != '<' {
		return NullVal(), false
	}
	val, _, err := ParseValue(src, pos)
	if err != nil || val.Kind != KindDict {
		return NullVal(), false
	}
	if _, ok := val.ValueEntry(keyRoot); !ok {
		return NullVal(), false
	}
	return val, true
}
