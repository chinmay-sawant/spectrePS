package pdf

// A document whose startxref offset holds no cross-reference section cannot be
// read from that offset. Ghostscript recovers by scanning the file for object
// headers and building a fresh table. This is that recovery.
//
// The scan runs only when the offset names no section at all, and it needs a
// trailer carrying a /Root on top of that. Both conditions exist to hold three
// pinned corpus refusals in place:
//
//   - structural/bad-xref.pdf names a real table whose rows are damaged, so the
//     offset presents a section and the reader reports the damage.
//   - images/UnknownFilter-xrefstm.pdf names an xref stream this reader cannot
//     decode, so the offset presents an indirect object and the reader reports
//     the unknown filter.
//   - structural/parser_rebuildxref_error_notrailer.pdf has no trailer /Root, so
//     the scan finds objects and the reader still refuses.
//
// A file that presents a broken section keeps its failure even when a table
// elsewhere in the same file would have parsed. That is the difference from a
// backward scan for a newer table, which lets a broken newest section through.

// recoverCrossRef rebuilds the cross-reference table and trailer for src, whose
// startxref offset is offset. It reports false when the recovery does not apply
// or does not produce a document, and the caller keeps its own error.
func recoverCrossRef(src []byte, offset int) (map[int]XEntry, Value, bool) {
	if xrefSectionAt(src, offset) {
		return nil, NullVal(), false
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

// xrefSectionAt reports whether readXRefSection would recognise a section at
// offset. The keyword test and the indirect object test mirror the dispatch in
// readXRefSection, so a file that presents a table or an object there never
// reaches the scan.
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
	return indirectParses(src, pos)
}

// indirectParses reports whether an indirect object parses at pos. It exists so
// the five-value ParseIndirect result does not become a dogsled at the call
// site, where only the error matters.
func indirectParses(src []byte, pos int) bool {
	number, generation, body, _, err := ParseIndirect(src, pos)
	if err != nil {
		return false
	}
	return number >= 0 && generation >= 0 && body.Kind > KindNull
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
