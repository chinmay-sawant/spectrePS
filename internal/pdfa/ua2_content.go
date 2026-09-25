package pdfa

// This file holds the content-side PDF/UA-2 coverage check: it scans the
// marked-content operators of every page and proves they pair and agree with
// the parent tree. It reads content bytes only; it paints nothing.

import (
	"strconv"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	ua2DictPairLen = 2
)

// ua2Item is one BDC marked-content sequence found in a page stream.
type ua2Item struct {
	mcid    int
	hasMCID bool
}

// ua2ContentRule proves every marked-content item pairs and every MCID agrees
// with the parent tree on both sides. An MCID in content with no parent tree
// claim is uncovered, and a parent tree claim with no content item is
// uncovered too. A rule failure is ua2-content.
func ua2ContentRule(file *pdf.File) error {
	tree, err := file.StructTree()
	if err != nil {
		return ua2TreeRule(err)
	}
	if tree == nil {
		return pdf.NewError(opPDFUA, ruleUA2StructTree)
	}
	covered, err := ua2CoveredKeys(file)
	if err != nil {
		return err
	}
	for key := range covered {
		if _, err := tree.Lookup(key); err != nil {
			return pdf.NewError(opPDFUA, ruleUA2Content)
		}
	}
	for _, key := range tree.Keys() {
		if !covered[key] {
			return pdf.NewError(opPDFUA, ruleUA2Content)
		}
	}
	return nil
}

// ua2CoveredKeys scans every page and returns the MCIDs its content carries.
// An unpaired marker is ua2-content.
func ua2CoveredKeys(file *pdf.File) (map[pdf.StructKey]bool, error) {
	covered := map[pdf.StructKey]bool{}
	for page := range file.PageCount() {
		content, err := file.Content(page)
		if err != nil {
			return nil, pdf.NewError(opPDFUA, ruleUA2Content)
		}
		items, paired := ua2ScanContent(content)
		if !paired {
			return nil, pdf.NewError(opPDFUA, ruleUA2Content)
		}
		for _, item := range items {
			if item.hasMCID {
				covered[pdf.StructKey{Page: page, MCID: item.mcid}] = true
			}
		}
	}
	return covered, nil
}

// ua2Operand is one parsed content operand. Only the parts the coverage check
// reads are kept: a name, an integer, and the /MCID of a dictionary.
type ua2Operand struct {
	isName  bool
	name    string
	intOK   bool
	intVal  int
	mcid    int
	hasMCID bool
}

// ua2Token is one operand or one operator word.
type ua2Token struct {
	op      string
	operand ua2Operand
}

// ua2Scan walks one decoded content stream.
type ua2Scan struct {
	src      []byte
	pos      int
	operands []ua2Operand
	items    []ua2Item
	open     int
	unpaired bool
}

// ua2ScanContent returns every BDC marked-content item in stream order and
// whether the markers pair. BMC opens without an MCID, MP and DP do not open,
// and a stray or unclosed EMC is unpaired.
func ua2ScanContent(src []byte) ([]ua2Item, bool) {
	scan := ua2Scan{
		src:      src,
		pos:      0,
		operands: nil,
		items:    nil,
		open:     0,
		unpaired: false,
	}
	for {
		scan.skipIgnored()
		if scan.pos >= len(src) {
			break
		}
		tok, ok := scan.next()
		if !ok {
			break
		}
		if tok.op == "" {
			scan.operands = append(scan.operands, tok.operand)
			continue
		}
		scan.operator(tok.op)
	}
	if scan.open != 0 {
		scan.unpaired = true
	}
	return scan.items, !scan.unpaired
}

// operator runs one operator word against the operand stack.
func (scan *ua2Scan) operator(word string) {
	switch word {
	case "BMC":
		scan.pop()
		scan.open++
	case "BDC":
		props := scan.pop()
		scan.pop()
		item := ua2Item{mcid: 0, hasMCID: false}
		if props.hasMCID {
			item.mcid = props.mcid
			item.hasMCID = true
		}
		scan.items = append(scan.items, item)
		scan.open++
	case "EMC":
		if scan.open == 0 {
			scan.unpaired = true
			return
		}
		scan.open--
	case "MP":
		scan.pop()
	case "DP":
		scan.pop()
		scan.pop()
	default:
		scan.operands = scan.operands[:0]
	}
}

// pop removes one operand. An empty stack returns an empty operand.
func (scan *ua2Scan) pop() ua2Operand {
	if len(scan.operands) == 0 {
		return otherOperand()
	}
	last := scan.operands[len(scan.operands)-1]
	scan.operands = scan.operands[:len(scan.operands)-1]
	return last
}

// next reads one operand or operator. The bool is false at the end of input.
func (scan *ua2Scan) next() (ua2Token, bool) {
	scan.skipIgnored()
	if scan.pos >= len(scan.src) {
		return zeroToken(), false
	}
	return scan.tokenAt(scan.src[scan.pos]), true
}

// tokenAt reads the token that starts with one byte.
func (scan *ua2Scan) tokenAt(cur byte) ua2Token {
	switch {
	case cur == '/':
		operand := otherOperand()
		operand.isName = true
		operand.name = scan.readName()
		return operandToken(operand)
	case cur == '(':
		scan.skipLiteral()
		return operandToken(otherOperand())
	case cur == '<':
		if scan.pair('<') {
			return operandToken(scan.dict())
		}
		scan.skipHex()
		return operandToken(otherOperand())
	case cur == '[':
		scan.skipArray()
		return operandToken(otherOperand())
	case ua2Delim(cur):
		scan.pos++
		return operandToken(otherOperand())
	default:
		return scan.wordToken()
	}
}

// ua2Delim reports whether one byte is a bare content delimiter.
func ua2Delim(cur byte) bool {
	switch cur {
	case ']', '>', ')', '}', '{':
		return true
	default:
		return false
	}
}

// wordToken reads one bare word: a number, a keyword operand, or an operator.
func (scan *ua2Scan) wordToken() ua2Token {
	word := scan.word()
	if word == "true" || word == "false" || word == "null" {
		return operandToken(otherOperand())
	}
	if num, err := strconv.ParseFloat(word, 64); err == nil {
		whole := int(num)
		if float64(whole) != num {
			return operandToken(otherOperand())
		}
		operand := otherOperand()
		operand.intOK = true
		operand.intVal = whole
		return operandToken(operand)
	}
	return ua2Token{op: word, operand: otherOperand()}
}

// dict reads one dictionary operand and keeps its /MCID when it has one.
func (scan *ua2Scan) dict() ua2Operand {
	scan.pos += ua2DictPairLen
	out := otherOperand()
	key := ""
	expectValue := false
	for {
		scan.skipIgnored()
		if scan.pos >= len(scan.src) {
			return out
		}
		if scan.pair('>') {
			scan.pos += ua2DictPairLen
			return out
		}
		tok, ok := scan.next()
		if !ok {
			return out
		}
		if tok.op != "" {
			continue
		}
		if !expectValue {
			if tok.operand.isName {
				key = tok.operand.name
				expectValue = true
			}
			continue
		}
		if key == "MCID" && tok.operand.intOK {
			out.mcid = tok.operand.intVal
			out.hasMCID = true
		}
		key = ""
		expectValue = false
	}
}

// skipIgnored skips whitespace and comments.
func (scan *ua2Scan) skipIgnored() {
	for scan.pos < len(scan.src) {
		cur := scan.src[scan.pos]
		if ua2Space(cur) {
			scan.pos++
			continue
		}
		if cur != '%' {
			return
		}
		for scan.pos < len(scan.src) && scan.src[scan.pos] != '\n' && scan.src[scan.pos] != '\r' {
			scan.pos++
		}
	}
}

// readName reads one name and returns its text without the slash.
func (scan *ua2Scan) readName() string {
	scan.pos++
	start := scan.pos
	for scan.pos < len(scan.src) && !ua2Break(scan.src[scan.pos]) {
		scan.pos++
	}
	return string(scan.src[start:scan.pos])
}

// skipLiteral skips one literal string with its escapes and nesting.
func (scan *ua2Scan) skipLiteral() {
	scan.pos++
	depth := 1
	for scan.pos < len(scan.src) && depth > 0 {
		cur := scan.src[scan.pos]
		scan.pos++
		switch cur {
		case '\\':
			if scan.pos < len(scan.src) {
				scan.pos++
			}
		case '(':
			depth++
		case ')':
			depth--
		}
	}
}

// skipHex skips one hex string.
func (scan *ua2Scan) skipHex() {
	scan.pos++
	for scan.pos < len(scan.src) {
		cur := scan.src[scan.pos]
		scan.pos++
		if cur == '>' {
			return
		}
	}
}

// skipArray skips one array, including nested values.
func (scan *ua2Scan) skipArray() {
	scan.pos++
	for scan.pos < len(scan.src) {
		scan.skipIgnored()
		if scan.pos >= len(scan.src) {
			return
		}
		if scan.src[scan.pos] == ']' {
			scan.pos++
			return
		}
		if _, ok := scan.next(); !ok {
			return
		}
	}
}

// word reads one bare word.
func (scan *ua2Scan) word() string {
	start := scan.pos
	for scan.pos < len(scan.src) && !ua2Break(scan.src[scan.pos]) {
		scan.pos++
	}
	return string(scan.src[start:scan.pos])
}

// pair reports whether the byte at pos starts a doubled delimiter.
func (scan *ua2Scan) pair(mark byte) bool {
	next := scan.pos + 1
	return next < len(scan.src) && scan.src[scan.pos] == mark && scan.src[next] == mark
}

// otherOperand returns an operand with no value.
func otherOperand() ua2Operand {
	return ua2Operand{
		isName: false, name: "",
		intOK: false, intVal: 0, mcid: 0, hasMCID: false,
	}
}

// operandToken wraps one operand.
func operandToken(operand ua2Operand) ua2Token {
	return ua2Token{op: "", operand: operand}
}

// zeroToken returns the empty token.
func zeroToken() ua2Token {
	return ua2Token{op: "", operand: otherOperand()}
}

// ua2Break reports whether one byte ends a bare word.
func ua2Break(cur byte) bool {
	if ua2Space(cur) {
		return true
	}
	switch cur {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

// ua2Space reports whether one byte is PDF whitespace.
func ua2Space(cur byte) bool {
	switch cur {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}
