// Package ps scans the PostScript subset.
package ps

import (
	"math"
	"strconv"
)

type (
	// TokKind classifies one scanned token.
	TokKind int

	// Token is one PostScript token from Scan.
	Token struct {
		Kind  TokKind
		Int   int32
		Real  float64
		Text  string
		Bytes []byte
	}

	scanner struct {
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
	// TokInt is a signed integer that fits in int32.
	TokInt TokKind = iota
	// TokReal is a real number stored as float64.
	TokReal
	// TokName is an executable name, including "[", "]", "<<", and ">>".
	TokName
	// TokLiteral is a literal name. Text does not include the slash.
	TokLiteral
	// TokString is a parenthesis string or a hex string.
	TokString
	// TokLBrace is '{', not a name.
	TokLBrace
	// TokRBrace is '}', not a name.
	TokRBrace

	nameSyntax = "syntaxerror"
	nameRange  = "rangecheck"
	opScan     = "scan"

	decBase     = 10
	octalBase   = 8
	octalExtra  = 2
	nibbleShift = 4
	hexDigitA   = 10
	byteMask    = 0xFF
)

// Scan tokenizes PostScript source.
// Whitespace and comments are dropped. An integer outside int32 is rangecheck.
// A byte sequence that is not a token is syntaxerror.
func Scan(src []byte) ([]Token, error) {
	lex := scanner{src: src, pos: 0}
	out := make([]Token, 0, len(src))
	for {
		lex.skipIgnored()
		if lex.pos >= len(lex.src) {
			return out, nil
		}
		tok, err := lex.one()
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
}

func (sc *scanner) one() (Token, error) {
	cur := sc.src[sc.pos]
	switch cur {
	case '{', '}', '[', ']':
		return sc.mark(cur), nil
	case '(':
		return sc.scanParen()
	case '<':
		return sc.scanAngle()
	case '>':
		return sc.scanGreater()
	case '/':
		return sc.scanLiteral()
	case ')':
		return zeroToken(), syntaxErr()
	default:
		return sc.scanWord()
	}
}

func (sc *scanner) mark(cur byte) Token {
	sc.pos++
	switch cur {
	case '{':
		return tokenBrace(TokLBrace)
	case '}':
		return tokenBrace(TokRBrace)
	case '[':
		return tokenName("[")
	default:
		return tokenName("]")
	}
}

func (sc *scanner) skipIgnored() {
	for sc.pos < len(sc.src) {
		cur := sc.src[sc.pos]
		if isSpace(cur) {
			sc.pos++
			continue
		}
		if cur != '%' {
			return
		}
		sc.skipComment()
	}
}

func (sc *scanner) skipComment() {
	sc.pos++
	for sc.pos < len(sc.src) {
		cur := sc.src[sc.pos]
		sc.pos++
		if cur == '\n' || cur == '\r' {
			return
		}
	}
}

func (sc *scanner) scanLiteral() (Token, error) {
	sc.pos++
	if sc.pos >= len(sc.src) || isDelim(sc.src[sc.pos]) {
		return zeroToken(), syntaxErr()
	}
	return tokenLiteral(sc.readWord()), nil
}

func (sc *scanner) scanWord() (Token, error) {
	text := sc.readWord()
	if text == "" {
		return zeroToken(), syntaxErr()
	}
	return classify(text)
}

func (sc *scanner) readWord() string {
	start := sc.pos
	for sc.pos < len(sc.src) && !isDelim(sc.src[sc.pos]) {
		sc.pos++
	}
	return string(sc.src[start:sc.pos])
}

func classify(text string) (Token, error) {
	if intSyntax(text) {
		return intFrom(text)
	}
	if realSyntax(text) {
		return realFrom(text)
	}
	return tokenName(text), nil
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

func intFrom(text string) (Token, error) {
	n, ok := parseInt32(text)
	if !ok {
		return zeroToken(), rangeErr()
	}
	return tokenInt(n), nil
}

func parseInt32(text string) (int32, bool) {
	idx := 0
	neg := false
	if hasSign(text, idx) {
		neg = text[idx] == '-'
		idx++
	}
	var n int32
	for idx < len(text) {
		digit := int32(text[idx] - '0')
		next, ok := applyDigit(n, digit, neg)
		if !ok {
			return 0, false
		}
		n = next
		idx++
	}
	return n, true
}

func applyDigit(n, digit int32, neg bool) (int32, bool) {
	if neg {
		return addNegDigit(n, digit)
	}
	return addPosDigit(n, digit)
}

func addPosDigit(n, digit int32) (int32, bool) {
	lim := int32(math.MaxInt32 / decBase)
	tail := int32(math.MaxInt32 % decBase)
	if n > lim || (n == lim && digit > tail) {
		return 0, false
	}
	return n*decBase + digit, true
}

func addNegDigit(n, digit int32) (int32, bool) {
	lim := int32(math.MinInt32 / decBase)
	tail := int32(-(math.MinInt32 % decBase))
	if n < lim || (n == lim && digit > tail) {
		return 0, false
	}
	return n*decBase - digit, true
}

func realSyntax(text string) bool {
	idx, sawDot, sawDigit := readMantissa(text)
	if !sawDigit {
		return false
	}
	if idx == len(text) {
		return sawDot
	}
	return exponentAt(text, idx)
}

func readMantissa(text string) (int, bool, bool) {
	idx := 0
	if hasSign(text, idx) {
		idx++
	}
	sawDot := false
	sawDigit := false
	for idx < len(text) {
		if isDigit(text[idx]) {
			sawDigit = true
			idx++
			continue
		}
		if text[idx] != '.' || sawDot {
			break
		}
		sawDot = true
		idx++
	}
	return idx, sawDot, sawDigit
}

func exponentAt(text string, idx int) bool {
	if idx >= len(text) || !isExp(text[idx]) {
		return false
	}
	idx++
	if hasSign(text, idx) {
		idx++
	}
	start := idx
	for idx < len(text) && isDigit(text[idx]) {
		idx++
	}
	return idx > start && idx == len(text)
}

func realFrom(text string) (Token, error) {
	val, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
		return zeroToken(), rangeErr()
	}
	return tokenReal(val), nil
}

// Parentheses nest. An escaped parenthesis does not change the depth.
func (sc *scanner) scanParen() (Token, error) {
	sc.pos++
	buf := make([]byte, 0)
	depth := 1
	for depth > 0 {
		if sc.pos >= len(sc.src) {
			return zeroToken(), syntaxErr()
		}
		cur := sc.src[sc.pos]
		sc.pos++
		next, err := sc.parenStep(cur, depth, &buf)
		if err != nil {
			return zeroToken(), err
		}
		depth = next
	}
	return tokenString(buf), nil
}

func (sc *scanner) parenStep(cur byte, depth int, buf *[]byte) (int, error) {
	if cur == '\\' {
		return sc.parenEscape(depth, buf)
	}
	if cur == '(' {
		*buf = append(*buf, cur)
		return depth + 1, nil
	}
	if cur == ')' {
		return closeParen(depth, buf), nil
	}
	*buf = append(*buf, cur)
	return depth, nil
}

func closeParen(depth int, buf *[]byte) int {
	depth--
	if depth > 0 {
		*buf = append(*buf, ')')
	}
	return depth
}

func (sc *scanner) parenEscape(depth int, buf *[]byte) (int, error) {
	got, keep, err := sc.readEscape()
	if err != nil {
		return depth, err
	}
	if keep {
		*buf = append(*buf, got)
	}
	return depth, nil
}

// A backslash before LF or CRLF continues the string and adds no byte.
func (sc *scanner) readEscape() (byte, bool, error) {
	if sc.pos >= len(sc.src) {
		return 0, false, syntaxErr()
	}
	cur := sc.src[sc.pos]
	sc.pos++
	if lit, ok := namedEscape(cur); ok {
		return lit, true, nil
	}
	switch cur {
	case '\n':
		return 0, false, nil
	case '\r':
		return sc.escapeCR()
	default:
		if isOctal(cur) {
			return sc.readOctal(cur), true, nil
		}
		return cur, true, nil
	}
}

func namedEscape(cur byte) (byte, bool) {
	switch cur {
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case '\\', '(', ')':
		return cur, true
	default:
		return 0, false
	}
}

func (sc *scanner) escapeCR() (byte, bool, error) {
	if sc.pos < len(sc.src) && sc.src[sc.pos] == '\n' {
		sc.pos++
		return 0, false, nil
	}
	return '\r', true, nil
}

func (sc *scanner) readOctal(first byte) byte {
	val := int(first - '0')
	extra := octalExtra
	for extra > 0 && sc.pos < len(sc.src) && isOctal(sc.src[sc.pos]) {
		val = val*octalBase + int(sc.src[sc.pos]-'0')
		sc.pos++
		extra--
	}
	return maskedByte(val)
}

func maskedByte(val int) byte {
	return byte(val & byteMask)
}

func (sc *scanner) scanAngle() (Token, error) {
	sc.pos++
	if sc.pos < len(sc.src) && sc.src[sc.pos] == '<' {
		sc.pos++
		return tokenName("<<"), nil
	}
	return sc.scanHex()
}

func (sc *scanner) scanGreater() (Token, error) {
	sc.pos++
	if sc.pos < len(sc.src) && sc.src[sc.pos] == '>' {
		sc.pos++
		return tokenName(">>"), nil
	}
	return zeroToken(), syntaxErr()
}

func (sc *scanner) scanHex() (Token, error) {
	var acc hexAcc
	for sc.pos < len(sc.src) {
		cur := sc.src[sc.pos]
		sc.pos++
		if cur == '>' {
			acc.finish()
			return tokenString(acc.buf), nil
		}
		if isSpace(cur) {
			continue
		}
		nib, ok := hexValue(cur)
		if !ok {
			return zeroToken(), syntaxErr()
		}
		acc.push(nib)
	}
	return zeroToken(), syntaxErr()
}

func (hx *hexAcc) push(nib byte) {
	if !hx.odd {
		hx.high = nib
		hx.odd = true
		return
	}
	hx.buf = append(hx.buf, hx.high<<nibbleShift|nib)
	hx.odd = false
}

func (hx *hexAcc) finish() {
	if !hx.odd {
		return
	}
	hx.buf = append(hx.buf, hx.high<<nibbleShift)
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

func tokenInt(v int32) Token {
	return makeToken(TokInt, v, 0, "", nil)
}

func tokenReal(v float64) Token {
	return makeToken(TokReal, 0, v, "", nil)
}

func tokenName(text string) Token {
	return makeToken(TokName, 0, 0, text, nil)
}

func tokenLiteral(text string) Token {
	return makeToken(TokLiteral, 0, 0, text, nil)
}

func tokenString(buf []byte) Token {
	return makeToken(TokString, 0, 0, "", buf)
}

func tokenBrace(kind TokKind) Token {
	return makeToken(kind, 0, 0, "", nil)
}

func makeToken(kind TokKind, n int32, val float64, text string, raw []byte) Token {
	return Token{Kind: kind, Int: n, Real: val, Text: text, Bytes: raw}
}

func zeroToken() Token {
	var tok Token
	return tok
}

func syntaxErr() error {
	return errOf(nameSyntax, opScan)
}

func rangeErr() error {
	return errOf(nameRange, opScan)
}

func isSpace(cur byte) bool {
	switch cur {
	case ' ', '\t', '\n', '\r', '\f', '\x00':
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

func isExp(cur byte) bool {
	return cur == 'e' || cur == 'E'
}

func hasSign(text string, idx int) bool {
	if idx >= len(text) {
		return false
	}
	return text[idx] == '+' || text[idx] == '-'
}
