package pdf

import "unicode/utf16"

// CMap token kinds local to the ToUnicode parser.
const (
	cmapEnd = iota
	cmapWord
	cmapHex
	cmapArrayOpen
	cmapArrayClose
	cmapName
)

// maxCMapRange caps one bfrange so a hostile range cannot allocate forever.
const (
	maxCMapRange = 1 << 16
	cmapPairLen  = 2
)

type cmapToken struct {
	kind  int
	text  string
	bytes []byte
}

type cmapLexer struct {
	src []byte
	pos int
}

// parseToUnicode reads the beginbfchar and beginbfrange sections of a
// /ToUnicode CMap. Malformed entries are skipped. An empty result is not an
// error; the encoding remains the fallback.
func parseToUnicode(src []byte) map[uint32]string {
	out := map[uint32]string{}
	lex := &cmapLexer{src: src, pos: 0}
	for {
		tok := lex.next()
		if tok.kind == cmapEnd {
			return out
		}
		if tok.kind != cmapWord {
			continue
		}
		switch tok.text {
		case "beginbfchar":
			lex.readBFChar(out)
		case "beginbfrange":
			lex.readBFRange(out)
		}
	}
}

func (lex *cmapLexer) readBFChar(out map[uint32]string) {
	for {
		key := lex.next()
		if lex.sectionEnd(key, "endbfchar") {
			return
		}
		val := lex.next()
		if key.kind != cmapHex || val.kind != cmapHex {
			continue
		}
		code, ok := cmapCode(key.bytes)
		if !ok {
			continue
		}
		out[code] = cmapUnicode(val.bytes)
	}
}

func (lex *cmapLexer) readBFRange(out map[uint32]string) {
	for {
		firstTok := lex.next()
		if lex.sectionEnd(firstTok, "endbfrange") {
			return
		}
		lastTok := lex.next()
		first, last, ok := rangePair(firstTok, lastTok)
		if !ok {
			continue
		}
		lex.readRangeBody(out, first, last)
	}
}

// sectionEnd reports the end of a bfchar or bfrange section.
func (lex *cmapLexer) sectionEnd(tok cmapToken, name string) bool {
	return tok.kind == cmapEnd || (tok.kind == cmapWord && tok.text == name)
}

// rangePair validates one bfrange bound pair.
func rangePair(firstTok, lastTok cmapToken) (uint32, uint32, bool) {
	if firstTok.kind != cmapHex || lastTok.kind != cmapHex {
		return 0, 0, false
	}
	first, okFirst := cmapCode(firstTok.bytes)
	last, okLast := cmapCode(lastTok.bytes)
	if !okFirst || !okLast || last < first || last-first > maxCMapRange {
		return 0, 0, false
	}
	return first, last, true
}

func (lex *cmapLexer) readRangeBody(out map[uint32]string, first, last uint32) {
	dest := lex.next()
	switch dest.kind {
	case cmapHex:
		units, ok := cmapUnits(dest.bytes)
		if !ok {
			return
		}
		for code := first; code <= last; code++ {
			out[code] = cmapIncrement(units, code-first)
		}
	case cmapArrayOpen:
		for code := first; code <= last; code++ {
			item := lex.next()
			if item.kind == cmapArrayClose || item.kind == cmapEnd {
				return
			}
			if item.kind != cmapHex {
				continue
			}
			out[code] = cmapUnicode(item.bytes)
		}
	}
}

func (lex *cmapLexer) next() cmapToken {
	lex.skipIgnored()
	if lex.pos >= len(lex.src) {
		return cmapToken{kind: cmapEnd, text: "", bytes: nil}
	}
	cur := lex.src[lex.pos]
	if token, ok := lex.punctuation(cur); ok {
		return token
	}
	switch cur {
	case '<':
		return lex.less()
	case '/':
		return cmapToken{kind: cmapName, text: lex.cmapName(), bytes: nil}
	case '(':
		return cmapToken{kind: cmapWord, text: lex.literal(), bytes: nil}
	default:
		return cmapToken{kind: cmapWord, text: lex.word(), bytes: nil}
	}
}

// punctuation reads the one-byte CMap delimiters.
func (lex *cmapLexer) punctuation(cur byte) (cmapToken, bool) {
	switch cur {
	case '>':
		lex.pos++
		return cmapToken{kind: cmapWord, text: ">>", bytes: nil}, true
	case '[':
		lex.pos++
		return cmapToken{kind: cmapArrayOpen, text: "", bytes: nil}, true
	case ']':
		lex.pos++
		return cmapToken{kind: cmapArrayClose, text: "", bytes: nil}, true
	default:
		return cmapToken{kind: cmapEnd, text: "", bytes: nil}, false
	}
}

// less reads a hex string or the start of a dictionary.
func (lex *cmapLexer) less() cmapToken {
	if lex.pos+1 < len(lex.src) && lex.src[lex.pos+1] == '<' {
		lex.pos += cmapPairLen
		return cmapToken{kind: cmapWord, text: "<<", bytes: nil}
	}
	return cmapToken{kind: cmapHex, text: "", bytes: lex.hex()}
}

func (lex *cmapLexer) skipIgnored() {
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		if contentSpace(cur) {
			lex.pos++
			continue
		}
		if cur != '%' {
			return
		}
		for lex.pos < len(lex.src) && lex.src[lex.pos] != '\n' && lex.src[lex.pos] != '\r' {
			lex.pos++
		}
	}
}

func (lex *cmapLexer) word() string {
	start := lex.pos
	for lex.pos < len(lex.src) && !isBreak(lex.src[lex.pos]) {
		lex.pos++
	}
	if start == lex.pos {
		lex.pos++
	}
	return string(lex.src[start:lex.pos])
}

func (lex *cmapLexer) cmapName() string {
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '/' {
		lex.pos++
	}
	start := lex.pos
	for lex.pos < len(lex.src) && !isBreak(lex.src[lex.pos]) {
		lex.pos++
	}
	return string(lex.src[start:lex.pos])
}

// hex reads one hex string, ignoring whitespace. An odd nibble is padded.
func (lex *cmapLexer) hex() []byte {
	lex.pos++
	out := make([]byte, 0)
	high := byte(0)
	odd := false
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		if cur == '>' {
			lex.pos++
			if odd {
				out = append(out, high<<nibbleShift)
			}
			return out
		}
		lex.pos++
		if contentSpace(cur) {
			continue
		}
		nib, ok := hexValue(cur)
		if !ok {
			continue
		}
		if !odd {
			high = nib
			odd = true
			continue
		}
		out = append(out, high<<nibbleShift|nib)
		odd = false
	}
	return out
}

// literal reads a parenthesized string and returns its raw text. ToUnicode
// destinations are hex strings, so escapes are not decoded.
func (lex *cmapLexer) literal() string {
	lex.pos++
	start := lex.pos
	depth := 1
	for lex.pos < len(lex.src) && depth > 0 {
		cur := lex.src[lex.pos]
		lex.pos++
		switch cur {
		case '\\':
			if lex.pos < len(lex.src) {
				lex.pos++
			}
		case '(':
			depth++
		case ')':
			depth--
		}
	}
	if lex.pos > start {
		return string(lex.src[start : lex.pos-1])
	}
	return ""
}

// cmapCode reads a character code from one hex string, most significant byte
// first. A code is one to four bytes.
func cmapCode(raw []byte) (uint32, bool) {
	if len(raw) == 0 || len(raw) > 4 {
		return 0, false
	}
	var code uint32
	for _, cur := range raw {
		code = code<<byteShift | uint32(cur)
	}
	return code, true
}

func cmapUnits(raw []byte) ([]uint16, bool) {
	if len(raw) == 0 || len(raw)%cmapPairLen != 0 {
		return nil, false
	}
	units := make([]uint16, 0, len(raw)/cmapPairLen)
	for idx := 0; idx < len(raw); idx += cmapPairLen {
		units = append(units, uint16(raw[idx])<<byteShift|uint16(raw[idx+1]))
	}
	return units, true
}

func cmapUnicode(raw []byte) string {
	units, ok := cmapUnits(raw)
	if !ok {
		return ""
	}
	return string(utf16.Decode(units))
}

// cmapIncrement adds delta to the last UTF-16 code unit.
func cmapIncrement(units []uint16, delta uint32) string {
	copied := make([]uint16, len(units))
	copy(copied, units)
	copied[len(copied)-1] += uint16(delta & cidMask) //nolint:gosec // the mask bounds the value to 16 bits
	return string(utf16.Decode(copied))
}
