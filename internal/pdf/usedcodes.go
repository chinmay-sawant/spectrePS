package pdf

import (
	"sort"
)

// maxUsedCodes caps the character codes one page and one font resource name
// reports. A font that reaches the cap is left whole by the subset writer, so
// the truncation can never zero a glyph the page needs.
const (
	maxUsedCodes = 1 << 12
	usedFormDeep = 8
	usedFormScan = 64
	itemSeed     = 8
)

// FontUsedCodes walks each page and returns the character codes each /Font
// resource name showed, sorted and unique. The outer index is the page, and
// the inner map is keyed by resource name. A page with no text has an empty
// map.
//
// Codes inside a reachable Form XObject count toward the page's font under
// the same resource name. A form that binds the name to a different font
// over-keeps the page font, which never drops a glyph the page paints.
// A resource name that shows more than maxUsedCodes distinct codes reports
// exactly maxUsedCodes of them, and the subset writer leaves that font whole.
// A malformed content stream and a page whose resources fail to resolve stop
// their own scan and keep what was found.
// A nil file returns nil.
func (file *File) FontUsedCodes() []map[string][]uint32 {
	if file == nil {
		return nil
	}
	out := make([]map[string][]uint32, len(file.pages))
	for idx := range out {
		out[idx] = file.pageUsedCodes(idx)
	}
	return out
}

// pageUsedCodes scans one page's content and forms.
func (file *File) pageUsedCodes(index int) map[string][]uint32 {
	res, err := file.PageResources(index)
	if err != nil {
		return map[string][]uint32{}
	}
	walk := &codeWalk{
		file:   file,
		sets:   map[string]*usedCodeSet{},
		budget: usedFormScan,
	}
	walk.scan(file.pages[index], res, 0)
	out := make(map[string][]uint32, len(walk.sets))
	for name, set := range walk.sets {
		out[name] = set.sorted()
	}
	return out
}

// usedCodeSet is one font resource name's codes.
type usedCodeSet struct {
	codes map[uint32]bool
}

func (set *usedCodeSet) add(code uint32) {
	if set.codes == nil {
		set.codes = map[uint32]bool{}
	}
	if len(set.codes) >= maxUsedCodes && !set.codes[code] {
		return
	}
	set.codes[code] = true
}

// sorted returns the codes in ascending order. An empty set returns nil.
func (set *usedCodeSet) sorted() []uint32 {
	if len(set.codes) == 0 {
		return nil
	}
	out := make([]uint32, 0, len(set.codes))
	for code := range set.codes {
		out = append(out, code)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// codeWalk scans content streams for the codes each font resource name shows.
type codeWalk struct {
	file   *File
	sets   map[string]*usedCodeSet
	budget int
}

// scan reads one content stream. The stack holds the operands since the last
// operator, which is all Tf, Tj, TJ, ', ", and Do need.
func (walk *codeWalk) scan(content []byte, res Resources, depth int) {
	if depth > usedFormDeep {
		return
	}
	lex := &scanner{src: content, pos: 0}
	stack := make([]item, 0, itemSeed)
	current := ""
	for {
		tok, ok, err := lex.next()
		if err != nil || !ok {
			return
		}
		if tok.kind != tokOperator {
			stack = append(stack, itemOf(tok))
			continue
		}
		walk.take(tok.text, res, &current, stack, depth)
		stack = stack[:0]
	}
}

// take reads one operator. Unknown operators drop their operands.
func (walk *codeWalk) take(opName string, res Resources, current *string, stack []item, depth int) {
	switch opName {
	case opTextFont:
		walk.setFont(current, stack)
	case opShow, opShowQuote, opShowDQuote:
		walk.addElement(res, *current, stackTop(stack))
	case opShowArray:
		walk.addArray(res, *current, stackTop(stack))
	case "Do":
		walk.form(stackTop(stack), res, depth)
	}
}

// setFont reads the name operand of Tf. The size operand sits above it.
func (walk *codeWalk) setFont(current *string, stack []item) {
	if len(stack) < tfOperands {
		return
	}
	name := stack[len(stack)-2]
	if name.kind == itemName {
		*current = name.name
	}
}

// stackTop returns the last operand, or an empty element when there is none.
func stackTop(stack []item) item {
	if len(stack) == 0 {
		return otherItem()
	}
	return stack[len(stack)-1]
}

// addArray reads the string elements of a TJ array.
func (walk *codeWalk) addArray(res Resources, name string, element item) {
	if element.kind != itemArray {
		return
	}
	for _, part := range element.arr {
		walk.addElement(res, name, part)
	}
}

// addElement records one string operand against the current font.
func (walk *codeWalk) addElement(res Resources, name string, element item) {
	if element.kind != itemString {
		return
	}
	fnt, ok := res.Font(name)
	if !ok {
		return
	}
	set := walk.set(name)
	if fnt.twoByteCodes() {
		walk.addTwoByte(set, element.str)
		return
	}
	for _, code := range element.str {
		set.add(uint32(code))
	}
}

// addTwoByte records two-byte codes and ignores a trailing odd byte.
func (walk *codeWalk) addTwoByte(set *usedCodeSet, raw []byte) {
	for idx := 0; idx+1 < len(raw); idx += cidPairLen {
		set.add(uint32(raw[idx])<<byteShift | uint32(raw[idx+1]))
	}
}

// set returns the code set for one resource name, creating it on first use.
func (walk *codeWalk) set(name string) *usedCodeSet {
	if set, ok := walk.sets[name]; ok {
		return set
	}
	set := &usedCodeSet{codes: nil}
	walk.sets[name] = set
	return set
}

// form follows one Form XObject through its own resources, under the scan
// budget. A non-form XObject, a decode error, and an exhausted budget stop
// the walk.
func (walk *codeWalk) form(element item, res Resources, depth int) {
	if element.kind != itemName || walk.budget <= 0 {
		return
	}
	entry, ok := res.XObjects[element.name]
	if !ok {
		return
	}
	if subtype, _ := entry.NameEntry(keySubtype); subtype != nameForm {
		return
	}
	body, err := decodeStream(entry)
	if err != nil {
		return
	}
	walk.budget--
	walk.scan(body, walk.formResources(entry, res), depth+1)
}

// formResources resolves a form's own /Resources, or keeps the parent's when
// the form inherits them.
func (walk *codeWalk) formResources(entry Value, res Resources) Resources {
	child, ok := entry.ValueEntry(keyResources)
	if !ok || child.Kind == KindNull {
		return res
	}
	resolved, err := walk.file.resolveResources(child)
	if err != nil {
		return res
	}
	return resolved
}
