package pdf

import (
	"bytes"
	"context"
	"strconv"
)

const (
	errSyntax = "syntaxerror"
	errAccess = "invalidaccess"
	errType   = "typecheck"

	opPDF     = "pdf"
	opXRef    = "xref"
	opEncrypt = "Encrypt"

	keyType     = "Type"
	keyXRef     = "XRef"
	keyEncrypt  = "Encrypt"
	keyRoot     = "Root"
	keyFilter   = "Filter"
	keyParms    = "DecodeParms"
	keyW        = "W"
	keySize     = "Size"
	keyIndex    = "Index"
	keyObjCount = "N"
	keyFirst    = "First"

	wordXRef    = "xref"
	wordTrailer = "trailer"
	wordStart   = "startxref"
	pdfHeader   = "%PDF-"
	panicNilCtx = "pdf: nil context"

	indexPairLen = 2
)

// File is one open PDF subset.
// Page count is the number of leaves walked from the page tree.
type File struct {
	src     []byte
	trailer Value
	xref    map[int]XEntry
	cache   map[int]Value
	streams map[int]map[int]stmItem
	pages   [][]byte
	busy    map[int]bool
}

type objPos struct {
	num    int
	offset int
}

type stmItem struct {
	value Value
	index int
}

// Open reads a PDF subset. Page count is the number of page leaves walked from the page tree, not the /Count field.
func Open(ctx context.Context, src []byte) (*File, error) {
	if ctx == nil {
		panic(panicNilCtx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !bytes.Contains(src, []byte(pdfHeader)) {
		return nil, NewError(opPDF, errSyntax)
	}
	offset, err := startOffset(src)
	if err != nil {
		return nil, err
	}
	entries, trailer, err := readCrossRef(src, offset)
	if err != nil {
		return nil, err
	}
	if encrypted(trailer) {
		return nil, NewError(opEncrypt, errAccess)
	}
	file := &File{
		src:     src,
		trailer: trailer,
		xref:    entries,
		cache:   map[int]Value{},
		streams: map[int]map[int]stmItem{},
		pages:   nil,
		busy:    map[int]bool{},
	}
	pages, err := file.walkRoot()
	if err != nil {
		return nil, err
	}
	file.pages = pages
	return file, nil
}

func startOffset(src []byte) (int, error) {
	marker := []byte(wordStart)
	from := len(src)
	for from > 0 {
		found := bytes.LastIndex(src[:from], marker)
		if found < 0 {
			return 0, NewError(opXRef, errSyntax)
		}
		if keywordHere(src, found, wordStart) {
			number, _, ok := pdfInt(src, found+len(wordStart))
			if ok && number >= 0 {
				return number, nil
			}
		}
		from = found
	}
	return 0, NewError(opXRef, errSyntax)
}

func readCrossRef(src []byte, offset int) (map[int]XEntry, Value, error) {
	if offset < 0 || offset >= len(src) {
		return nil, NullVal(), NewError(opXRef, errSyntax)
	}
	pos := skipSpace(src, offset)
	if pos >= len(src) {
		return nil, NullVal(), NewError(opXRef, errSyntax)
	}
	if hasKeyword(src, pos, wordXRef) {
		return readClassic(src, pos)
	}
	return readStreamXRef(src, pos)
}

func readClassic(src []byte, offset int) (map[int]XEntry, Value, error) {
	entries, next, err := ParseClassic(src, offset)
	if err != nil {
		return nil, NullVal(), err
	}
	if entries == nil {
		entries = map[int]XEntry{}
	}
	trailer, err := trailerDict(src, next)
	if err != nil {
		return nil, NullVal(), err
	}
	return entries, trailer, nil
}

func trailerDict(src []byte, offset int) (Value, error) {
	pos := skipSpace(src, offset)
	if hasKeyword(src, pos, wordTrailer) {
		pos = skipSpace(src, pos+len(wordTrailer))
	}
	if pos >= len(src) || src[pos] != '<' {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	val, _, err := ParseValue(src, pos)
	if err != nil {
		return NullVal(), err
	}
	if val.Kind != KindDict {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return val, nil
}

func readStreamXRef(src []byte, offset int) (map[int]XEntry, Value, error) {
	_, _, val, _, err := ParseIndirect(src, offset)
	if err != nil {
		return nil, NullVal(), err
	}
	if !xrefStream(val) {
		return nil, NullVal(), NewError(opXRef, errSyntax)
	}
	body, err := decodeStream(val)
	if err != nil {
		return nil, NullVal(), err
	}
	widths, err := intList(val, keyW)
	if err != nil {
		return nil, NullVal(), err
	}
	size, ok := val.IntEntry(keySize)
	if !ok {
		return nil, NullVal(), NewError(opXRef, errSyntax)
	}
	pairs, err := indexList(val)
	if err != nil {
		return nil, NullVal(), err
	}
	entries, err := ParseStreamRows(body, widths, size, pairs)
	if err != nil {
		return nil, NullVal(), err
	}
	if entries == nil {
		entries = map[int]XEntry{}
	}
	return entries, val, nil
}

func xrefStream(val Value) bool {
	if val.Kind != KindStream {
		return false
	}
	if typeName, ok := val.NameEntry(keyType); ok && typeName == keyXRef {
		return true
	}
	_, ok := val.ArrayEntry(keyW)
	return ok
}

func intList(val Value, key string) ([]int, error) {
	items, ok := val.ArrayEntry(key)
	if !ok || len(items) == 0 {
		return nil, NewError(opXRef, errSyntax)
	}
	return valueInts(items)
}

func indexList(val Value) ([]int, error) {
	items, ok := val.ArrayEntry(keyIndex)
	if !ok {
		var pairs []int
		return pairs, nil
	}
	if len(items) == 0 || len(items)%indexPairLen != 0 {
		return nil, NewError(opXRef, errSyntax)
	}
	return valueInts(items)
}

func valueInts(items []Value) ([]int, error) {
	numbers := make([]int, 0, len(items))
	for _, item := range items {
		number, ok := intOf(item)
		if !ok {
			return nil, NewError(opXRef, errSyntax)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func intOf(val Value) (int, bool) {
	if val.Kind == KindInt {
		return int(val.Int), true
	}
	if val.Kind != KindReal {
		return 0, false
	}
	whole := int64(val.Real)
	if float64(whole) != val.Real {
		return 0, false
	}
	return int(whole), true
}

func encrypted(trailer Value) bool {
	entry, ok := trailer.ValueEntry(keyEncrypt)
	if !ok {
		return false
	}
	return entry.Kind != KindNull
}

func (file *File) resolve(num int) (Value, error) {
	if val, ok := file.cache[num]; ok {
		return val, nil
	}
	if file.busy[num] {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	entry, ok := file.xref[num]
	if !ok || !entry.InUse {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	file.busy[num] = true
	defer delete(file.busy, num)

	var (
		val Value
		err error
	)
	if entry.Compressed {
		val, err = file.compressed(num, entry)
	} else {
		val, err = file.plain(num, entry)
	}
	if err != nil {
		return NullVal(), err
	}
	file.cache[num] = val
	return val, nil
}

func (file *File) deref(val Value) (Value, error) {
	if val.Kind != KindRef {
		return val, nil
	}
	entry, ok := file.xref[val.RefNum]
	if !ok || !entry.InUse || !genOK(entry, val.RefGen) {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return file.resolve(val.RefNum)
}

func genOK(entry XEntry, gen int) bool {
	if entry.Compressed {
		return gen == 0
	}
	return entry.Gen == gen
}

func (file *File) plain(num int, entry XEntry) (Value, error) {
	if entry.Offset < 0 || entry.Offset >= len(file.src) {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	got, gen, val, _, err := ParseIndirect(file.src, entry.Offset)
	if err != nil {
		return NullVal(), err
	}
	if got != num || gen != entry.Gen {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return val, nil
}

func (file *File) compressed(num int, entry XEntry) (Value, error) {
	objects, err := file.loadObjStream(entry.StreamNum)
	if err != nil {
		return NullVal(), err
	}
	item, ok := objects[num]
	if !ok || item.index != entry.StreamIdx {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return item.value, nil
}

func (file *File) loadObjStream(streamNum int) (map[int]stmItem, error) {
	if objects, ok := file.streams[streamNum]; ok {
		return objects, nil
	}
	val, err := file.resolve(streamNum)
	if err != nil {
		return nil, err
	}
	count, first, ok := streamBounds(val)
	if !ok {
		return nil, NewError(opXRef, errSyntax)
	}
	body, err := decodeStream(val)
	if err != nil {
		return nil, err
	}
	objects, err := splitObjStream(body, count, first)
	if err != nil {
		return nil, err
	}
	file.streams[streamNum] = objects
	return objects, nil
}

func streamBounds(val Value) (int, int, bool) {
	count, ok := val.IntEntry(keyObjCount)
	if !ok {
		return 0, 0, false
	}
	first, ok := val.IntEntry(keyFirst)
	if !ok {
		return 0, 0, false
	}
	return count, first, true
}

func splitObjStream(body []byte, count, first int) (map[int]stmItem, error) {
	if count < 0 || first < 0 || first > len(body) {
		return nil, NewError(opXRef, errSyntax)
	}
	pairs, err := headerPairs(body[:first], count)
	if err != nil {
		return nil, err
	}
	objects := make(map[int]stmItem, count)
	for idx, pair := range pairs {
		if pair.offset < 0 || pair.offset >= len(body)-first {
			return nil, NewError(opXRef, errSyntax)
		}
		val, _, err := ParseValue(body, first+pair.offset)
		if err != nil {
			return nil, err
		}
		objects[pair.num] = stmItem{value: val, index: idx}
	}
	return objects, nil
}

func headerPairs(header []byte, count int) ([]objPos, error) {
	if count < 0 || count > len(header) {
		return nil, NewError(opXRef, errSyntax)
	}
	pairs := make([]objPos, 0, count)
	pos := 0
	for len(pairs) < count {
		objNum, objOff, next, ok := pairAt(header, pos)
		if !ok {
			return nil, NewError(opXRef, errSyntax)
		}
		pairs = append(pairs, objPos{num: objNum, offset: objOff})
		pos = next
	}
	return pairs, nil
}

func pairAt(header []byte, pos int) (int, int, int, bool) {
	objNum, next, ok := pdfInt(header, pos)
	if !ok {
		return 0, 0, pos, false
	}
	objOff, next, ok := pdfInt(header, next)
	if !ok {
		return 0, 0, pos, false
	}
	return objNum, objOff, next, true
}

func keywordHere(src []byte, pos int, word string) bool {
	if pos > 0 && !isDelim(src[pos-1]) {
		return false
	}
	return hasKeyword(src, pos, word)
}

func hasKeyword(src []byte, pos int, word string) bool {
	end := pos + len(word)
	if end > len(src) || !bytes.Equal(src[pos:end], []byte(word)) {
		return false
	}
	if end == len(src) {
		return true
	}
	return isDelim(src[end])
}

func pdfInt(src []byte, pos int) (int, int, bool) {
	pos = skipSpace(src, pos)
	if pos >= len(src) {
		return 0, pos, false
	}
	start := pos
	if src[pos] == '+' || src[pos] == '-' {
		pos++
	}
	digits := pos
	for pos < len(src) && src[pos] >= '0' && src[pos] <= '9' {
		pos++
	}
	if pos == digits {
		return 0, start, false
	}
	number, err := strconv.Atoi(string(src[start:pos]))
	if err != nil {
		return 0, start, false
	}
	return number, pos, true
}

func skipSpace(src []byte, pos int) int {
	for pos < len(src) {
		if isSpaceByte(src[pos]) {
			pos++
			continue
		}
		if src[pos] != '%' {
			return pos
		}
		pos = skipComment(src, pos+1)
	}
	return pos
}

func skipComment(src []byte, pos int) int {
	for pos < len(src) {
		cur := src[pos]
		pos++
		if cur == '\n' || cur == '\r' {
			return pos
		}
	}
	return pos
}

func isSpaceByte(cur byte) bool {
	switch cur {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func isDelim(cur byte) bool {
	if isSpaceByte(cur) {
		return true
	}
	switch cur {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}
