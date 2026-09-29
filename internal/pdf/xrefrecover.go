package pdf

// A document whose startxref offset holds no cross-reference section cannot be
// read from that offset. Ghostscript recovers by scanning the file for object
// headers and building a fresh table. This is that recovery.
//
// The scan runs when the file names no section at all: no usable startxref, an
// offset that lands on something that is not a table and not an xref stream, or
// a parsed section that held zero rows. It always needs a trailer carrying a
// /Root on top of that. The gates exist to hold three pinned corpus refusals in
// place:
//
//   - structural/bad-xref.pdf names a real table whose rows are damaged, so the
//     offset presents a section and the reader reports the damage.
//   - images/UnknownFilter-xrefstm.pdf names an xref stream this reader cannot
//     decode, so the offset presents a stream and the reader reports the
//     unknown filter.
//   - structural/parser_rebuildxref_error_notrailer.pdf has no trailer /Root, so
//     the scan finds objects and the reader still refuses.
//
// A file that presents a broken section keeps its failure even when a table
// elsewhere in the same file would have parsed. That is the difference from a
// backward scan for a newer table, which lets a broken newest section through.
//
// A fourth repair sits next to this one: a table that never mentions an object
// number the file does carry can still resolve that number from its header, in
// entryFor below. The row it invents is the one the file wrote, not a row from
// an older table.

// recoverCrossRef rebuilds the cross-reference table and trailer for src, whose
// startxref offset is offset. It reports false when the recovery does not apply
// or does not produce a document, and the caller keeps its own error.
func recoverCrossRef(src []byte, offset int) (map[int]XEntry, Value, bool) {
	if xrefSectionAt(src, offset) {
		return nil, NullVal(), false
	}
	if entries, trailer, ok := recoverStreamXRef(src); ok {
		return entries, trailer, true
	}
	return scanRecover(src)
}

// scanRecover rebuilds the table from the file's own object headers. It is the
// body of recoverCrossRef, split out for the callers that already know no
// section stands at the offset: a missing startxref and a section that parsed
// to zero rows. A document still needs a trailer carrying a /Root.
func scanRecover(src []byte) (map[int]XEntry, Value, bool) {
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

// xrefSectionAt reports whether readXRefSection would find a cross-reference
// section at offset. A classic table keyword counts. An indirect object counts
// only when it is an xref stream, so a startxref that names an ordinary object
// or lands in the header does not pin the reader to the wrong bytes. The
// keyword test and the stream test mirror the dispatch in readXRefSection, so a
// file that presents a table or an xref stream there never reaches the scan.
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

// entryFor returns the xref row for num. A number the table never mentions is
// repaired once from the file's own object headers: a producer that stopped
// writing rows still wrote the object, and Ghostscript reads it. A number whose
// row is present keeps that row whatever it says, so a genuinely referenced
// dead row is still a hard failure. The scan runs at most once per file.
func (file *File) entryFor(num int) (XEntry, bool) {
	if entry, ok := file.xref[num]; ok {
		return entry, true
	}
	if !file.headersScanned {
		file.headersScanned = true
		for n, entry := range scanObjectHeaders(file.src) {
			if _, exists := file.xref[n]; !exists {
				file.xref[n] = entry
			}
		}
	}
	entry, ok := file.xref[num]
	return entry, ok
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
