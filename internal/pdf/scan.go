package pdf

import (
	"math"
	"strconv"
)

type (
	tokKind int

	token struct {
		kind tokKind
		num  int64
		real float64
		text string
		raw  []byte
	}

	lexer struct {
		src []byte
		pos int
	}

	hexAcc struct {
		buf  []byte
		high byte
		odd  bool
	}
)

const (
	tokEnd tokKind = iota
	tokInt
	tokReal
	tokName
	tokString
	tokWord
	tokArrayOpen
	tokArrayClose
	tokDictOpen
	tokDictClose

	nameSyntax = "syntaxerror"
	opScan     = "scan"

	wordTrue      = "true"
	wordFalse     = "false"
	wordNull      = "null"
	wordRef       = "R"
	wordObj       = "obj"
	wordEndObj    = "endobj"
	wordStream    = "stream"
	wordEndStream = "endstream"
	wordLength    = "Length"

	decBase     = 10
	octalBase   = 8
	octalMax    = 3
	nibbleShift = 4
	hexDigitA   = 10
	byteMask    = 0xFF
	bitSize     = 64
)

func syntaxErr(opName string) error {
	return NewError(opName, nameSyntax)
}

func newLexer(src []byte, offset int) (*lexer, error) {
	if offset < 0 || offset > len(src) {
		return nil, syntaxErr(opScan)
	}
	return &lexer{src: src, pos: offset}, nil
}

func (lex *lexer) skipIgnored() {
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		if isSpace(cur) {
			lex.pos++
			continue
		}
		if cur != '%' {
			return
		}
		lex.skipComment()
	}
}

func (lex *lexer) skipComment() {
	lex.pos++
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		lex.pos++
		if cur == '\n' || cur == '\r' {
			return
		}
	}
}

func (lex *lexer) take() (token, error) {
	lex.skipIgnored()
	if lex.pos >= len(lex.src) {
		return endToken(), nil
	}
	switch lex.src[lex.pos] {
	case '[':
		return lex.mark(tokArrayOpen), nil
	case ']':
		return lex.mark(tokArrayClose), nil
	case '<':
		return lex.scanLess()
	case '>':
		return lex.scanGreater()
	case '(':
		return lex.scanLiteral()
	case '/':
		return lex.scanName()
	default:
		return lex.scanOther()
	}
}

func (lex *lexer) mark(kind tokKind) token {
	lex.pos++
	return newToken(kind, 0, 0, "", nil)
}

func (lex *lexer) scanOther() (token, error) {
	if isDelim(lex.src[lex.pos]) {
		return endToken(), syntaxErr(opScan)
	}
	return lex.scanWord()
}

func (lex *lexer) scanLess() (token, error) {
	lex.pos++
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '<' {
		lex.pos++
		return newToken(tokDictOpen, 0, 0, "", nil), nil
	}
	return lex.scanHex()
}

func (lex *lexer) scanGreater() (token, error) {
	lex.pos++
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '>' {
		lex.pos++
		return newToken(tokDictClose, 0, 0, "", nil), nil
	}
	return endToken(), syntaxErr(opScan)
}

func (lex *lexer) scanHex() (token, error) {
	var acc hexAcc
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		lex.pos++
		if cur == '>' {
			acc.finish()
			return newToken(tokString, 0, 0, "", acc.buf), nil
		}
		if isSpace(cur) {
			continue
		}
		nib, ok := hexValue(cur)
		if !ok {
			return endToken(), syntaxErr(opScan)
		}
		acc.push(nib)
	}
	return endToken(), syntaxErr(opScan)
}

func (acc *hexAcc) push(nib byte) {
	if !acc.odd {
		acc.high = nib
		acc.odd = true
		return
	}
	acc.buf = append(acc.buf, acc.high<<nibbleShift|nib)
	acc.odd = false
}

// An odd final nibble is stored as if a 0 followed it.
func (acc *hexAcc) finish() {
	if !acc.odd {
		return
	}
	acc.buf = append(acc.buf, acc.high<<nibbleShift)
}

func hexValue(cur byte) (byte, bool) {
	switch {
	case cur >= '0' && cur <= '9':
		return cur - '0', true
	case cur >= 'a' && cur <= 'f':
		return cur - 'a' + hexDigitA, true
	case cur >= 'A' && cur <= 'F':
		return cur - 'A' + hexDigitA, true
	default:
		return 0, false
	}
}

func (lex *lexer) scanLiteral() (token, error) {
	lex.pos++
	buf := make([]byte, 0)
	depth := 1
	for depth > 0 {
		if lex.pos >= len(lex.src) {
			return endToken(), syntaxErr(opScan)
		}
		cur := lex.src[lex.pos]
		lex.pos++
		next, err := lex.literalStep(cur, depth, &buf)
		if err != nil {
			return endToken(), err
		}
		depth = next
	}
	return newToken(tokString, 0, 0, "", buf), nil
}

func (lex *lexer) literalStep(cur byte, depth int, buf *[]byte) (int, error) {
	if cur == '\\' {
		return lex.literalEscape(depth, buf)
	}
	if cur == '(' {
		*buf = append(*buf, cur)
		return depth + 1, nil
	}
	if cur == ')' {
		return closeLiteral(depth, buf), nil
	}
	return lex.literalByte(cur, depth, buf)
}

func (lex *lexer) literalByte(cur byte, depth int, buf *[]byte) (int, error) {
	if cur != '\r' {
		*buf = append(*buf, cur)
		return depth, nil
	}
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '\n' {
		lex.pos++
	}
	*buf = append(*buf, '\n')
	return depth, nil
}

func closeLiteral(depth int, buf *[]byte) int {
	depth--
	if depth > 0 {
		*buf = append(*buf, ')')
	}
	return depth
}

func (lex *lexer) literalEscape(depth int, buf *[]byte) (int, error) {
	got, keep, err := lex.readEscape()
	if err != nil {
		return depth, err
	}
	if keep {
		*buf = append(*buf, got)
	}
	return depth, nil
}

func (lex *lexer) readEscape() (byte, bool, error) {
	if lex.pos >= len(lex.src) {
		return 0, false, syntaxErr(opScan)
	}
	cur := lex.src[lex.pos]
	lex.pos++
	if lit, ok := namedEscape(cur); ok {
		return lit, true, nil
	}
	if cur == '\n' {
		return 0, false, nil
	}
	if cur == '\r' {
		lex.skipLF()
		return 0, false, nil
	}
	if isOctal(cur) {
		return lex.readOctal(cur), true, nil
	}
	return cur, true, nil
}

func namedEscape(cur byte) (byte, bool) {
	switch cur {
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case '\\', '(', ')':
		return cur, true
	default:
		return 0, false
	}
}

func (lex *lexer) skipLF() {
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '\n' {
		lex.pos++
	}
}

func (lex *lexer) readOctal(first byte) byte {
	val := int(first - '0')
	digits := 1
	for digits < octalMax && lex.pos < len(lex.src) && isOctal(lex.src[lex.pos]) {
		val = val*octalBase + int(lex.src[lex.pos]-'0')
		lex.pos++
		digits++
	}
	return byte(val & byteMask)
}

func (lex *lexer) scanName() (token, error) {
	lex.pos++
	buf := make([]byte, 0)
	for lex.pos < len(lex.src) && !isDelim(lex.src[lex.pos]) {
		cur := lex.src[lex.pos]
		lex.pos++
		if cur != '#' {
			buf = append(buf, cur)
			continue
		}
		got, err := lex.nameByte()
		if err != nil {
			return endToken(), err
		}
		buf = append(buf, got)
	}
	return newToken(tokName, 0, 0, string(buf), nil), nil
}

func (lex *lexer) nameByte() (byte, error) {
	if lex.pos+1 >= len(lex.src) {
		return 0, syntaxErr(opScan)
	}
	high, ok := hexValue(lex.src[lex.pos])
	if !ok {
		return 0, syntaxErr(opScan)
	}
	low, ok := hexValue(lex.src[lex.pos+1])
	if !ok {
		return 0, syntaxErr(opScan)
	}
	lex.pos += 2
	return high<<nibbleShift | low, nil
}

func (lex *lexer) scanWord() (token, error) {
	start := lex.pos
	for lex.pos < len(lex.src) && !isDelim(lex.src[lex.pos]) {
		lex.pos++
	}
	text := string(lex.src[start:lex.pos])
	if text == "" {
		return endToken(), syntaxErr(opScan)
	}
	return classifyWord(text)
}

func classifyWord(text string) (token, error) {
	if intSyntax(text) {
		return intToken(text)
	}
	if realSyntax(text) {
		return realToken(text)
	}
	return newToken(tokWord, 0, 0, text, nil), nil
}

func intSyntax(text string) bool {
	idx := 0
	if hasSign(text, idx) {
		idx++
	}
	if idx >= len(text) {
		return false
	}
	for idx < len(text) {
		if !isDigit(text[idx]) {
			return false
		}
		idx++
	}
	return true
}

func realSyntax(text string) bool {
	idx := 0
	if hasSign(text, idx) {
		idx++
	}
	sawDot := false
	sawDigit := false
	for idx < len(text) {
		cur := text[idx]
		if isDigit(cur) {
			sawDigit = true
			idx++
			continue
		}
		if cur != '.' || sawDot {
			return false
		}
		sawDot = true
		idx++
	}
	return sawDot && sawDigit
}

func intToken(text string) (token, error) {
	num, err := strconv.ParseInt(text, decBase, bitSize)
	if err != nil {
		return endToken(), syntaxErr(opScan)
	}
	return newToken(tokInt, num, 0, "", nil), nil
}

func realToken(text string) (token, error) {
	num, err := strconv.ParseFloat(text, bitSize)
	if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
		return endToken(), syntaxErr(opScan)
	}
	return newToken(tokReal, 0, num, "", nil), nil
}

func hasSign(text string, idx int) bool {
	if idx >= len(text) {
		return false
	}
	return text[idx] == '+' || text[idx] == '-'
}

func isSpace(cur byte) bool {
	switch cur {
	case ' ', '\t', '\n', '\r', '\f', 0:
		return true
	default:
		return false
	}
}

func isDelim(cur byte) bool {
	if isSpace(cur) {
		return true
	}
	switch cur {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

func isDigit(cur byte) bool {
	return cur >= '0' && cur <= '9'
}

func isOctal(cur byte) bool {
	return cur >= '0' && cur <= '7'
}

func newToken(kind tokKind, num int64, real float64, text string, raw []byte) token {
	return token{kind: kind, num: num, real: real, text: text, raw: raw}
}

func endToken() token {
	return newToken(tokEnd, 0, 0, "", nil)
}
