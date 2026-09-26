package pdf

import "bytes"

// lengthResolver resolves one indirect /Length reference to a non-negative
// byte count. It reports false when the reference cannot be used, and the
// reader then scans for endstream.
type lengthResolver interface {
	resolveLength(ref Value) (int, bool)
}

// attachStream turns a parsed dictionary into a stream value when the reader
// is positioned at the stream keyword. A dictionary without stream is left
// alone.
func (lex *lexer) attachStream(val *Value) error {
	if val.Kind != KindDict {
		return nil
	}
	mark := lex.pos
	tok, err := lex.take()
	if err != nil {
		return err
	}
	if tok.kind != tokWord || tok.text != wordStream {
		lex.pos = mark
		return nil
	}
	raw, err := lex.streamBytes(val.Dict)
	if err != nil {
		return err
	}
	*val = StreamVal(val.Dict, raw)
	return nil
}

// streamBytes reads the bytes between stream and endstream.
// The declared /Length wins when its span ends at the keyword, with PDF
// whitespace allowed in between. When the length is missing, is indirect and
// unresolved, or does not reach endstream, the reader scans forward for the
// keyword. A missing length or a stream with no endstream is syntaxerror.
func (lex *lexer) streamBytes(dict map[string]Value) ([]byte, error) {
	if !lex.skipOneEOL() {
		return nil, syntaxErr(wordStream)
	}
	start := lex.pos
	length, declared, err := lex.declaredLength(dict)
	if err != nil {
		return nil, err
	}
	if declared {
		if lex.endstreamAfterDeclared(start, length) {
			return bytes.Clone(lex.src[start : start+length]), nil
		}
	}
	end, ok := scanEndstream(lex.src, start)
	if !ok {
		return nil, syntaxErr(wordEndStream)
	}
	lex.pos = end + len(wordEndStream)
	stop := trimStreamEOL(lex.src, start, end)
	return bytes.Clone(lex.src[start:stop]), nil
}

// declaredLength returns the /Length byte count. ok is false when the entry is
// an indirect reference this lexer cannot resolve, so the caller scans for
// endstream instead. A missing length or a non-integer one is syntaxerror.
func (lex *lexer) declaredLength(dict map[string]Value) (int, bool, error) {
	entry, ok := dict[wordLength]
	if !ok {
		return 0, false, syntaxErr(wordLength)
	}
	//nolint:exhaustive // a non-integer, non-reference length is syntaxerror.
	switch entry.Kind {
	case KindInt:
		length, fit := fitInt(entry.Int)
		if !fit || length < 0 {
			return 0, false, syntaxErr(wordLength)
		}
		return length, true, nil
	case KindRef:
		if lex.resolveLength == nil {
			return 0, false, nil
		}
		length, ok := lex.resolveLength.resolveLength(entry)
		if !ok {
			return 0, false, nil
		}
		return length, true, nil
	default:
		return 0, false, syntaxErr(wordLength)
	}
}

// endstreamAfterDeclared reads the keyword after the declared span with the
// normal token reader, whitespace aside. The reader rewinds to start when the
// keyword is not there, so the caller can scan for it instead.
func (lex *lexer) endstreamAfterDeclared(start, length int) bool {
	end := start + length
	if length < 0 || end < start || end > len(lex.src) {
		return false
	}
	lex.pos = end
	if err := lex.expectWord(wordEndStream); err != nil {
		lex.pos = start
		return false
	}
	return true
}

// scanEndstream finds the next endstream keyword at or after start and returns
// its offset. The keyword must be followed by a delimiter or the end of file.
func scanEndstream(src []byte, start int) (int, bool) {
	pos := start
	for pos < len(src) {
		found := bytes.Index(src[pos:], []byte(wordEndStream))
		if found < 0 {
			return 0, false
		}
		at := pos + found
		if endstreamAt(src, at) {
			return at, true
		}
		pos = at + 1
	}
	return 0, false
}

// endstreamAt reports whether the endstream keyword starts at pos.
func endstreamAt(src []byte, pos int) bool {
	end := pos + len(wordEndStream)
	if pos < 0 || end > len(src) || !bytes.Equal(src[pos:end], []byte(wordEndStream)) {
		return false
	}
	return end == len(src) || isDelim(src[end])
}

// trimStreamEOL drops one EOL that separates scanned data from the keyword.
func trimStreamEOL(src []byte, start, end int) int {
	if end <= start {
		return end
	}
	switch src[end-1] {
	case '\n':
		end--
		if end > start && src[end-1] == '\r' {
			end--
		}
	case '\r':
		end--
	}
	return end
}

// skipOneEOL consumes LF, CR, or CRLF. Any other byte is refused.
func (lex *lexer) skipOneEOL() bool {
	if lex.pos >= len(lex.src) {
		return false
	}
	cur := lex.src[lex.pos]
	if cur == '\n' {
		lex.pos++
		return true
	}
	if cur != '\r' {
		return false
	}
	lex.pos++
	lex.skipLF()
	return true
}

// resolveLength returns the declared byte count an indirect /Length reference
// names, using the file xref. A free or missing target, a non-integer value,
// and a negative count report false, and the reader then scans for endstream.
func (file *File) resolveLength(ref Value) (int, bool) {
	if ref.Kind != KindRef {
		return 0, false
	}
	value, err := file.deref(ref)
	if err != nil || value.Kind != KindInt || value.Int < 0 {
		return 0, false
	}
	length, ok := fitInt(value.Int)
	if !ok {
		return 0, false
	}
	return length, true
}

// resolvedStream reparses an uncompressed stream object with the xref-aware
// /Length resolver and caches the result. It reports false for a free or
// missing reference, a compressed object, a non-stream object, and a parse
// error, so the caller can fall back to the file cache.
func (file *File) resolvedStream(ref Value) (Value, bool) {
	if ref.Kind != KindRef {
		return NullVal(), false
	}
	entry, ok := file.xref[ref.RefNum]
	if !ok || !entry.InUse || entry.Compressed || !genOK(entry, ref.RefGen) {
		return NullVal(), false
	}
	//nolint:dogsled // parseIndirect returns five values; only the stream is used here.
	_, _, stream, _, err := parseIndirect(file.src, entry.Offset, file)
	if err != nil || stream.Kind != KindStream {
		return NullVal(), false
	}
	file.cache[ref.RefNum] = stream
	return stream, true
}
