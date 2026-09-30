package pdf

import (
	"math"
)

// recoverHeaderSlack bounds the search for the next object header when an
// object's endobj keyword is missing. The corpus shape is one stray token
// between the value and the next header, so a small window is enough and a
// longer gap keeps the syntax error.
const recoverHeaderSlack = 64

// ParseValue reads one PDF value at offset. next is the first byte after that value.
// A dictionary is KindDict. This function does not consume a stream body.
func ParseValue(src []byte, offset int) (Value, int, error) {
	lex, err := newLexer(src, offset)
	if err != nil {
		return NullVal(), 0, err
	}
	val, err := lex.parseValue()
	if err != nil {
		return NullVal(), 0, err
	}
	return val, lex.pos, nil
}

// ParseIndirect reads "num gen obj value endobj" at offset.
// A stream object is KindStream: Dict is the stream dictionary and Stream is the raw
// bytes between stream and endstream.
// /Length may be a direct integer or an indirect reference. This function
// cannot resolve a reference, so it scans for endstream; parseIndirect with a
// resolver reads the declared span first and scans when it disagrees.
// next is the first byte after endobj.
func ParseIndirect(src []byte, offset int) (int, int, Value, int, error) {
	return parseIndirect(src, offset, nil)
}

// parseIndirect reads one indirect object with an optional /Length resolver.
func parseIndirect(src []byte, offset int, resolve lengthResolver) (int, int, Value, int, error) {
	lex, err := newLexer(src, offset)
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	lex.resolveLength = resolve
	num, gen, err := lex.objectHeader()
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	val, err := lex.parseValue()
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	if err = lex.attachStream(&val); err != nil {
		return 0, 0, NullVal(), 0, err
	}
	mark := lex.pos
	if err = lex.expectWord(wordEndObj); err != nil {
		// A missing endobj in front of the next object header is a repair
		// Ghostscript reports as "Encountered 'obj' while expecting 'endobj'".
		// The value is already complete, so a header close behind it ends this
		// object. Anything further away keeps the error. The caller still sees
		// the end of the value as next, because no endobj was consumed.
		if objectHeaderNear(src, mark, recoverHeaderSlack) < 0 {
			return 0, 0, NullVal(), 0, err
		}
		return num, gen, val, mark, nil
	}
	return num, gen, val, lex.pos, nil
}

func (lex *lexer) parseValue() (Value, error) {
	tok, err := lex.take()
	if err != nil {
		return NullVal(), err
	}
	return lex.valueFrom(tok)
}

func (lex *lexer) valueFrom(tok token) (Value, error) {
	switch tok.kind {
	case tokInt:
		return lex.intOrRef(tok.num)
	case tokReal:
		return RealVal(tok.real), nil
	case tokName:
		return NameVal(tok.text), nil
	case tokString:
		return StringVal(string(tok.raw)), nil
	case tokWord:
		return keywordValue(tok.text)
	case tokArrayOpen:
		return lex.parseArray()
	case tokDictOpen:
		return lex.parseDict()
	case tokEnd, tokArrayClose, tokDictClose:
		return NullVal(), syntaxErr(opScan)
	default:
		return NullVal(), syntaxErr(opScan)
	}
}

func keywordValue(text string) (Value, error) {
	switch text {
	case wordTrue:
		return BoolVal(true), nil
	case wordFalse:
		return BoolVal(false), nil
	case wordNull:
		return NullVal(), nil
	default:
		return NullVal(), syntaxErr(text)
	}
}

func (lex *lexer) intOrRef(num int64) (Value, error) {
	ref, ok, err := lex.takeRef(num)
	if err != nil {
		return NullVal(), err
	}
	if ok {
		return ref, nil
	}
	return IntVal(num), nil
}

// takeRef reads "gen R" after an integer, or rewinds so that integer stands alone.
func (lex *lexer) takeRef(num int64) (Value, bool, error) {
	mark := lex.pos
	genTok, ok := lex.peekRef()
	if !ok {
		// "num R" with the generation left out. The specification writes
		// "num gen R", and a producer that dropped the middle number still
		// meant a reference to generation zero. GHOSTSCRIPT-701876-0.pdf
		// carries "/OCGs [+ 0 R]" and paints a full page.
		if lex.peekBareRef() {
			objNum, fits := fitInt(num)
			if !fits {
				return NullVal(), false, syntaxErr(wordRef)
			}
			return RefVal(objNum, 0), true, nil
		}
		return NullVal(), false, nil
	}
	if num < 0 || genTok.num < 0 {
		lex.pos = mark
		return NullVal(), false, nil
	}
	objNum, ok := fitInt(num)
	if !ok {
		return NullVal(), false, syntaxErr(wordRef)
	}
	gen, ok := fitInt(genTok.num)
	if !ok {
		return NullVal(), false, syntaxErr(wordRef)
	}
	return RefVal(objNum, gen), true, nil
}

// peekBareRef reads the "R" of a reference whose generation was left out, and
// reports generation zero. It is only consulted after peekRef has already
// failed, so the three-token form is never affected.
//
// The test is on the bytes rather than on a taken token. Taking a token costs
// an allocation for every standalone integer in the document, and the level-2
// rewrite allocation count is a gate: this shape appeared as 715 before the
// tolerance and 723 after, and the eight allocations were all here.
func (lex *lexer) peekBareRef() bool {
	position := lex.pos
	for position < len(lex.src) && isSpace(lex.src[position]) {
		position++
	}
	if position >= len(lex.src) || lex.src[position] != 'R' {
		return false
	}
	end := position + 1
	if end < len(lex.src) && !isDelim(lex.src[end]) {
		return false
	}
	lex.pos = end
	return true
}

func (lex *lexer) peekRef() (token, bool) {
	mark := lex.pos
	genTok, err := lex.take()
	if err != nil || genTok.kind != tokInt {
		lex.pos = mark
		return endToken(), false
	}
	word, err := lex.take()
	if err != nil || word.kind != tokWord || word.text != wordRef {
		lex.pos = mark
		return endToken(), false
	}
	return genTok, true
}

func fitInt(num int64) (int, bool) {
	if num > int64(math.MaxInt) || num < int64(math.MinInt) {
		return 0, false
	}
	return int(num), true
}

func (lex *lexer) parseArray() (Value, error) {
	items := []Value{}
	for {
		lex.skipIgnored()
		if lex.pos >= len(lex.src) {
			return NullVal(), syntaxErr("]")
		}
		if lex.src[lex.pos] == ']' {
			lex.pos++
			return ArrayVal(items), nil
		}
		lex.skipStrayPlus()
		item, err := lex.parseValue()
		if err != nil {
			return NullVal(), err
		}
		items = append(items, item)
	}
}

func (lex *lexer) parseDict() (Value, error) {
	entries := map[string]Value{}
	for {
		lex.skipIgnored()
		lex.keyMark = lex.pos
		if lex.pos >= len(lex.src) {
			return NullVal(), syntaxErr(">>")
		}
		if lex.atDictClose() {
			lex.pos += 2
			return DictVal(entries), nil
		}
		key, err := lex.parseDictKey()
		if err != nil {
			return NullVal(), err
		}
		item, err := lex.parseValue()
		if err != nil {
			return NullVal(), err
		}
		entries[key.Name] = item
	}
}

// parseDictKey reads a dictionary key. The specification writes it as a name,
// and a producer that dropped the leading slash still wrote a key: Ghostscript
// reads +AF as the key AF rather than refusing the dictionary.
// GHOSTSCRIPT-701801-0.pdf carries "+AF [2 0 R]" where "/AF" was meant.
func (lex *lexer) parseDictKey() (Value, error) {
	key, err := lex.parseValue()
	if err == nil {
		if key.Kind != KindName {
			return NullVal(), syntaxErr("<<")
		}
		return key, nil
	}
	// The value failed. A bare word in this position is the key itself.
	lex.pos = lex.keyMark
	lex.skipIgnored()
	tok, err := lex.take()
	if err != nil || tok.kind != tokWord {
		return NullVal(), err
	}
	return NameVal(tok.text), nil
}

func (lex *lexer) atDictClose() bool {
	if lex.pos+1 >= len(lex.src) {
		return false
	}
	return lex.src[lex.pos] == '>' && lex.src[lex.pos+1] == '>'
}

// skipStrayPlus steps over a lone "+" in an array element position. A producer
// that meant "0 0 R" and wrote "+ 0 R" still wrote the element, and Ghostscript
// drops the sign rather than refusing the array. GHOSTSCRIPT-701876-0.pdf
// carries "/OCGs [+ 0 R]" and paints a full page. The sign is skipped only when
// whitespace follows it, so "+5" and "+AF" are untouched.
func (lex *lexer) skipStrayPlus() {
	if lex.pos >= len(lex.src) || lex.src[lex.pos] != '+' {
		return
	}
	next := lex.pos + 1
	if next < len(lex.src) && !isDelim(lex.src[next]) {
		return
	}
	lex.pos = next
	lex.skipIgnored()
}

// objectHeaderNear returns the offset of the first object header within slack
// bytes at or after from, or -1. It is the repair window for a value whose
// endobj is missing: the next object header follows within a stray token or
// two, and a longer gap is not evidence of a missing endobj.
func objectHeaderNear(src []byte, from, slack int) int {
	limit := min(from+slack, len(src))
	for pos := max(from, 0); pos < limit; pos++ {
		if _, _, ok := objectHeaderAt(src, pos); ok {
			return pos
		}
	}
	return -1
}

func (lex *lexer) expectWord(word string) error {
	tok, err := lex.take()
	if err != nil {
		return err
	}
	if tok.kind != tokWord || tok.text != word {
		return syntaxErr(word)
	}
	return nil
}

func (lex *lexer) objectHeader() (int, int, error) {
	numTok, err := lex.take()
	if err != nil {
		return 0, 0, err
	}
	genTok, err := lex.take()
	if err != nil {
		return 0, 0, err
	}
	if numTok.kind != tokInt || genTok.kind != tokInt || numTok.num < 0 || genTok.num < 0 {
		return 0, 0, syntaxErr(wordObj)
	}
	if err = lex.expectWord(wordObj); err != nil {
		return 0, 0, err
	}
	num, ok := fitInt(numTok.num)
	if !ok {
		return 0, 0, syntaxErr(wordObj)
	}
	gen, ok := fitInt(genTok.num)
	if !ok {
		return 0, 0, syntaxErr(wordObj)
	}
	return num, gen, nil
}
