package pdf

import (
	"bytes"
	"math"
)

// ParseValue reads one PDF value at offset. next is the first byte after that value.
// A dictionary is KindDict. This function does not consume a stream body.
func ParseValue(src []byte, offset int) (val Value, next int, err error) {
	lex, err := newLexer(src, offset)
	if err != nil {
		return NullVal(), 0, err
	}
	val, err = lex.parseValue()
	if err != nil {
		return NullVal(), 0, err
	}
	return val, lex.pos, nil
}

// ParseIndirect reads "num gen obj value endobj" at offset.
// A stream object is KindStream: Dict is the stream dictionary and Stream is the raw
// bytes between stream and endstream.
// /Length must be a direct integer. An indirect Length returns NewError("Length", "syntaxerror").
// next is the first byte after endobj.
func ParseIndirect(src []byte, offset int) (num int, gen int, val Value, next int, err error) {
	lex, err := newLexer(src, offset)
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	num, gen, err = lex.objectHeader()
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	val, err = lex.parseValue()
	if err != nil {
		return 0, 0, NullVal(), 0, err
	}
	if err = lex.attachStream(&val); err != nil {
		return 0, 0, NullVal(), 0, err
	}
	if err = lex.expectWord(wordEndObj); err != nil {
		return 0, 0, NullVal(), 0, err
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
	genTok, err := lex.take()
	if err != nil || genTok.kind != tokInt {
		lex.pos = mark
		return NullVal(), false, nil
	}
	word, err := lex.take()
	if err != nil || word.kind != tokWord || word.text != wordRef {
		lex.pos = mark
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
		if lex.pos >= len(lex.src) {
			return NullVal(), syntaxErr(">>")
		}
		if lex.atDictClose() {
			lex.pos += 2
			return DictVal(entries), nil
		}
		key, err := lex.parseValue()
		if err != nil {
			return NullVal(), err
		}
		if key.Kind != KindName {
			return NullVal(), syntaxErr("<<")
		}
		item, err := lex.parseValue()
		if err != nil {
			return NullVal(), err
		}
		entries[key.Name] = item
	}
}

func (lex *lexer) atDictClose() bool {
	if lex.pos+1 >= len(lex.src) {
		return false
	}
	return lex.src[lex.pos] == '>' && lex.src[lex.pos+1] == '>'
}

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

// streamBytes reads Length encoded bytes. It does not search that span for endstream.
func (lex *lexer) streamBytes(dict map[string]Value) ([]byte, error) {
	if !lex.skipOneEOL() {
		return nil, syntaxErr(wordStream)
	}
	length, err := directLength(dict)
	if err != nil {
		return nil, err
	}
	end, ok := streamEnd(lex.pos, length, len(lex.src))
	if !ok {
		return nil, syntaxErr(wordStream)
	}
	raw := bytes.Clone(lex.src[lex.pos:end])
	lex.pos = end
	if err = lex.expectWord(wordEndStream); err != nil {
		return nil, err
	}
	return raw, nil
}

func streamEnd(pos, length, size int) (int, bool) {
	end := pos + length
	if length < 0 || end < pos || end > size {
		return 0, false
	}
	return end, true
}

func directLength(dict map[string]Value) (int, error) {
	entry, ok := dict[wordLength]
	if !ok || entry.Kind != KindInt || entry.Int < 0 {
		return 0, syntaxErr(wordLength)
	}
	length, ok := fitInt(entry.Int)
	if !ok {
		return 0, syntaxErr(wordLength)
	}
	return length, nil
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
