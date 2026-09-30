package pdf

import (
	"bytes"
	"strconv"
)

// A document whose cross-reference table does not name the objects the file
// carries cannot be read through that table. Ghostscript reports the damage
// and rebuilds from the object headers, and this file is that rebuild, in the
// shapes the batch2 corpus asked for:
//
//   - The startxref is missing, unreadable, or names nothing: the trailer and
//     the object headers are the whole document, so the rebuild runs.
//   - The section at the startxref is damaged: when its rows sit on the
//     specification's 20-byte grid only individual fields are hurt and the
//     grid reader keeps the row numbering, and when they do not line up the
//     rebuild falls through to the object headers.
//   - A /Prev link older than the newest section names nothing, points past
//     the end, or names a damaged section: the sections already read stand and
//     the rows they leave out come from the object headers. A /Prev cycle is
//     an infinite loop and keeps its refusal, which
//     structural/bug_xrefv4_loop.pdf pins.
//   - A row names the wrong bytes, or a referenced number has no row at all:
//     the object header, then the object streams, decide where the object is.
//   - A stream without a usable /Length reads to its endstream keyword.
//
// Every one of them needs a trailer carrying a /Root. The two corpus refusals
// that lack one still refuse for that reason:
//
//   - structural/parser_rebuildxref_error_notrailer.pdf has no trailer /Root
//     anywhere, so the scan finds objects and the reader still refuses.
//   - GHOSTSCRIPT-699115-0.pdf carries a damaged /Root name, so the same test
//     refuses it.
//
// Two more pinned shapes are opened by the product decision of 2026-09-29
// (Spectre PS opens what Ghostscript opens) and the manifest now records what
// the reader does with them:
//
//   - structural/bad-xref.pdf names a classic table whose rows do not line up
//     with the 20-byte grid; the rebuild opens it and the painter refuses the
//     non-embedded font, which is the fonts policy rather than the xref.
//   - images/UnknownFilter-xrefstm.pdf names an xref stream this reader cannot
//     decode; the rebuild reads the older table behind it and the page paints
//     without the image.
//
// A file whose newest section reads keeps its own error only when the caller
// cannot be repaired: readCrossRef owns the /Prev chain from there.

// recoverCrossRef rebuilds the cross-reference table and trailer for src, whose
// startxref offset is offset. It reports false when the recovery does not apply
// or does not produce a document, and the caller keeps its own error.
func recoverCrossRef(src []byte, offset int) (map[int]XEntry, Value, bool) {
	if xrefSectionAt(src, offset) {
		// The offset presents a section. When that section itself reads, the
		// caller's failure is in its /Prev chain: a cycle, a chain past the
		// limit, or a damaged older section. A broken chain is read by
		// readCrossRef, which keeps a cycle refused and rebuilds the rest, so
		// the caller's error stands and no rebuild runs here.
		if _, _, err := readXRefSection(src, offset); err == nil {
			return nil, NullVal(), false
		}
		// The section at the offset is damaged. When its rows sit on the
		// specification's 20-byte grid, only individual fields are hurt and
		// the section is still usable. A table whose rows do not line up
		// loses the mapping from row slot to object number, so the rebuild
		// falls through to the object headers, which is what Ghostscript does
		// after reporting the same damage.
		if entries, trailer, ok := recoverClassicGrid(src, offset); ok {
			return entries, trailer, true
		}
	}
	if entries, trailer, ok := recoverNearTable(src, offset); ok {
		return entries, trailer, true
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
		// The keyword is absent or damaged. The dictionary that carries the
		// root is still in the file, and a document is defined by its root
		// rather than by the punctuation in front of it, so look for the
		// dictionary itself.
		trailer, ok = lastRootDict(src)
	}
	if !ok {
		// No trailer at all. The file still has a catalog, and a catalog is
		// what a root names: synthesise a trailer that points at the object
		// whose /Type is /Catalog. This is what Ghostscript does with a file
		// whose trailer was lost, and 17 of the 36 corpus files in this shape
		// are under a kilobyte of fragments that still carry their catalog.
		trailer, ok = catalogTrailer(src, entries)
	}
	if !ok {
		return nil, NullVal(), false
	}
	return entries, trailer, true
}

// catalogTrailer builds a trailer naming the object whose /Type is /Catalog.
// The search runs over the numbers the header scan found, newest last, so the
// last catalog in the file wins, which is the one an incremental update would
// have left. A file with no catalog anywhere still refuses.
func catalogTrailer(src []byte, entries map[int]XEntry) (Value, bool) {
	best := NullVal()
	chosen := 0
	found := false
	for num, entry := range entries {
		if !entry.InUse || entry.Compressed {
			continue
		}
		_, _, val, _, err := ParseIndirect(src, entry.Offset)
		if err != nil || val.Kind != KindDict {
			continue
		}
		typeName, ok := val.NameEntry(keyType)
		if !ok || typeName != "Catalog" {
			continue
		}
		if !found {
			best = DictVal(map[string]Value{keyRoot: RefVal(num, 0)})
			chosen = num
			found = true
			continue
		}
		if num > chosen {
			best = DictVal(map[string]Value{keyRoot: RefVal(num, 0)})
			chosen = num
		}
	}
	return best, found
}

// recoverClassicGrid re-reads a classic section at offset against the strict
// 20-byte row grid, then the trailer that follows it. It reports false unless
// a row parsed and the trailer carries a /Root, so a table whose rows never
// sit on the grid falls through to the object-header rebuild and
// parser_rebuildxref_error_notrailer.pdf keeps refusing for its missing
// /Root.
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
// stream, which is the only shape readStreamXRef accepts. A section that then
// fails to read falls through to the object-header rebuild in the caller, so
// a damaged classic table and a cross-reference stream this reader cannot
// decode are both attempts rather than verdicts. Only
// structural/parser_rebuildxref_error_notrailer.pdf's missing /Root has the
// last word, in recoverCrossRef.
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
	streamEntries := map[int]XEntry{}
	malformedEntries := map[int]XEntry{}
	for pos := 0; pos < len(src); {
		pos = scanOuterHeader(src, pos, entries, streamEntries, malformedEntries)
	}
	mergeHeaderEntries(entries, streamEntries)
	mergeHeaderEntries(entries, malformedEntries)
	return entries
}

func scanOuterHeader(src []byte, pos int, entries, streamEntries, malformedEntries map[int]XEntry) int {
	candidate, found := objectHeaderCandidateAt(src, pos)
	if !found {
		return pos + 1
	}
	if !candidate.parsed {
		keepHeader(malformedEntries, candidate.num, pos, candidate.gen)
		return pos + 1
	}
	keepHeader(entries, candidate.num, pos, candidate.gen)
	if candidate.body.Kind == KindStream && directLengthMismatch(candidate.body) {
		scanStreamHeaders(src, pos+1, candidate.next, streamEntries, malformedEntries)
	}
	if candidate.next > pos {
		return candidate.next
	}
	return pos + 1
}

// scanStreamHeaders searches a stream span after its direct /Length disagrees
// with the bytes ending at endstream. Some damaged files place real indirect
// objects before that keyword, so the ordinary scan would otherwise treat
// them as stream data. These entries stay secondary to objects found outside
// the stream.
func scanStreamHeaders(src []byte, start, end int, entries, malformed map[int]XEntry) {
	spans := []objectSpan{{start: start, end: end}}
	for len(spans) > 0 {
		last := len(spans) - 1
		current := spans[last]
		spans = spans[:last]
		spans = append(spans, scanStreamSpan(src, current, entries, malformed)...)
	}
}

type objectSpan struct{ start, end int }

type objectHeaderCandidate struct {
	num    int
	gen    int
	next   int
	body   Value
	parsed bool
}

func objectHeaderCandidateAt(src []byte, pos int) (objectHeaderCandidate, bool) {
	num, gen, ok := objectHeaderAt(src, pos)
	if !ok {
		return objectHeaderCandidate{
			num:    0,
			gen:    0,
			next:   0,
			body:   NullVal(),
			parsed: false,
		}, false
	}
	got, gotGen, body, next, err := ParseIndirect(src, pos)
	return objectHeaderCandidate{
		num:    num,
		gen:    gen,
		next:   next,
		body:   body,
		parsed: err == nil && got == num && gotGen == gen,
	}, true
}

func scanStreamSpan(src []byte, current objectSpan, entries, malformed map[int]XEntry) []objectSpan {
	nested := []objectSpan{}
	for pos := current.start; pos < current.end; {
		next, child, hasChild := scanStreamHeaderAt(src, pos, current.end, entries, malformed)
		if hasChild {
			nested = append(nested, child)
		}
		pos = next
	}
	return nested
}

func scanStreamHeaderAt(src []byte, pos, end int, entries, malformed map[int]XEntry) (int, objectSpan, bool) {
	candidate, found := objectHeaderCandidateAt(src, pos)
	if !found {
		return pos + 1, objectSpan{start: 0, end: 0}, false
	}
	if !candidate.parsed {
		keepHeader(malformed, candidate.num, pos, candidate.gen)
		return pos + 1, objectSpan{start: 0, end: 0}, false
	}
	keepHeader(entries, candidate.num, pos, candidate.gen)
	isMismatchedStream := candidate.body.Kind == KindStream && directLengthMismatch(candidate.body)
	isInSpan := candidate.next > pos && candidate.next <= end
	if isMismatchedStream && isInSpan {
		return candidate.next, objectSpan{start: pos + 1, end: candidate.next}, true
	}
	if isInSpan {
		return candidate.next, objectSpan{start: 0, end: 0}, false
	}
	return pos + 1, objectSpan{start: 0, end: 0}, false
}

func mergeHeaderEntries(entries, candidates map[int]XEntry) {
	for num, entry := range candidates {
		if _, seen := entries[num]; !seen {
			entries[num] = entry
		}
	}
}

func keepHeader(entries map[int]XEntry, num, pos, gen int) {
	if num == 0 {
		return
	}
	if _, seen := entries[num]; !seen {
		entries[num] = plainEntry(pos, gen)
	}
}

func directLengthMismatch(value Value) bool {
	if value.Kind != KindStream {
		return false
	}
	length, ok := value.Dict[wordLength]
	if !ok || length.Kind != KindInt {
		return false
	}
	return length.Int != int64(len(value.Stream))
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

// lastRootDict returns the last dictionary in src that carries a /Root, with or
// without a `trailer` keyword in front of it. A producer that damaged the
// keyword still wrote the dictionary, and Ghostscript reads the file by finding
// that dictionary, so the rebuild does too: a document's root is what makes it
// readable, and the keyword is punctuation.
func lastRootDict(src []byte) (Value, bool) {
	best := NullVal()
	found := false
	for pos := 0; pos+1 < len(src); pos++ {
		if src[pos] != '<' || src[pos+1] != '<' {
			continue
		}
		val, _, err := ParseValue(src, pos)
		if err != nil || val.Kind != KindDict {
			continue
		}
		if _, ok := val.ValueEntry(keyRoot); !ok {
			continue
		}
		best = val
		found = true
	}
	return best, found
}

// nearTableWindow is how far before a startxref offset the reader looks for the
// table keyword it should have named. Measured on the batch2 corpus, two files
// name an offset that lands inside their own table, 23 and 55 bytes past the
// keyword, which is a producer counting from a different base rather than a
// damaged file. The window is small on purpose: this finds the table the offset
// almost named, not a different table elsewhere in the file.
const nearTableWindow = 256

// recoverNearTable reads the classic table whose keyword sits just before
// offset. It is the case where startxref points a few bytes into the table
// rather than at its keyword.
func recoverNearTable(src []byte, offset int) (map[int]XEntry, Value, bool) {
	if offset <= 0 || offset > len(src) {
		return nil, NullVal(), false
	}
	low := offset - nearTableWindow
	if low < 0 {
		low = 0
	}
	xrefOffset := bytes.LastIndex(src[low:offset], []byte(wordXRef))
	if xrefOffset < 0 {
		return nil, NullVal(), false
	}
	xrefOffset += low
	if !keywordHere(src, xrefOffset, wordXRef) {
		return nil, NullVal(), false
	}
	entries, trailer, err := readCrossRef(src, xrefOffset)
	if err != nil || len(entries) == 0 {
		return nil, NullVal(), false
	}
	return entries, trailer, true
}
