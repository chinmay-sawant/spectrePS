package pdf

import (
	"bytes"
	"context"
	"sort"
	"strconv"
)

const (
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
	keyPrev     = "Prev"
	keyObjStm   = "ObjStm"

	// xrefChainLimit caps one trailer /Prev chain. A longer chain is a loop
	// or a broken producer, so it is syntaxerror in xref.
	xrefChainLimit = 64

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
	src       []byte
	trailer   Value
	xref      map[int]XEntry
	cache     map[int]Value
	streams   map[int]map[int]stmItem
	pages     [][]byte
	resources []Value
	busy      map[int]bool
	scan      map[int]XEntry
	packed    map[int]XEntry
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
		// The offset named no cross-reference section. Rebuild the table from
		// the file's own object headers before reporting the failure.
		recovered, recoveredTrailer, ok := recoverCrossRef(src, offset)
		if !ok {
			return nil, err
		}
		entries, trailer = recovered, recoveredTrailer
	}
	if encrypted(trailer) {
		return nil, NewError(opEncrypt, errAccess)
	}
	file := &File{
		src:       src,
		trailer:   trailer,
		xref:      entries,
		cache:     map[int]Value{},
		streams:   map[int]map[int]stmItem{},
		pages:     nil,
		resources: nil,
		busy:      map[int]bool{},
		scan:      nil,
		packed:    nil,
	}
	leaves, err := file.walkRoot()
	if err != nil {
		return nil, err
	}
	file.pages = make([][]byte, len(leaves))
	file.resources = make([]Value, len(leaves))
	for i, leaf := range leaves {
		file.pages[i] = leaf.content
		file.resources[i] = leaf.resources
	}
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

// readCrossRef reads the xref section at offset and follows the trailer /Prev
// chain, newest section first. The newest section wins per object number and an
// older section fills only the gaps. Trailer entries inherit from older
// sections the same way, except /Prev and /Size, which describe one section.
// A /Prev cycle, a chain past xrefChainLimit, a malformed /Prev, and a missing
// section are syntaxerror in xref. A single section keeps the map the section
// reader returned, so the common path adds no allocation.
//
//nolint:cyclop // one branch per trailer /Prev section step.
func readCrossRef(src []byte, offset int) (map[int]XEntry, Value, error) {
	entries := map[int]XEntry(nil)
	trailer := NullVal()
	var seen map[int]bool
	current := offset
	for section := 0; ; section++ {
		if section >= xrefChainLimit {
			return nil, NullVal(), NewError(opXRef, errSyntax)
		}
		if current < 0 || current >= len(src) || seen[current] {
			return nil, NullVal(), NewError(opXRef, errSyntax)
		}
		sectionEntries, sectionTrailer, err := readXRefSection(src, current)
		if err != nil {
			// A /Prev link that names no section at all is a broken chain, not
			// a damaged section. The sections already read stand, and the rows
			// they leave out come from the file's own object headers. A /Prev
			// that names a section keeps its error, so a cycle, a past-end
			// link, and a damaged older section still refuse by name.
			if section > 0 && !xrefSectionAt(src, current) {
				fillEntries(entries, scanObjectHeaders(src))
				break
			}
			return nil, NullVal(), err
		}
		if entries == nil {
			entries = sectionEntries
			trailer = sectionTrailer
		} else {
			fillEntries(entries, sectionEntries)
			trailer = mergeTrailer(trailer, sectionTrailer)
		}
		prev, hasPrev, err := prevOffset(sectionTrailer)
		if err != nil {
			return nil, NullVal(), err
		}
		if !hasPrev {
			break
		}
		if seen == nil {
			seen = map[int]bool{current: true}
		} else {
			seen[current] = true
		}
		current = prev
	}
	delete(trailer.Dict, keyPrev)
	return entries, trailer, nil
}

// readXRefSection reads one classic table or xref stream section. The returned
// value is the section trailer: the table's trailer dictionary or the xref
// stream dictionary. The name matches the section that fails.
func readXRefSection(src []byte, offset int) (map[int]XEntry, Value, error) {
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

// fillEntries adds rows from an older section. The newest section wins per
// object number, so a number that already has a row keeps it.
func fillEntries(entries, older map[int]XEntry) {
	for num, entry := range older {
		if _, ok := entries[num]; !ok {
			entries[num] = entry
		}
	}
}

// mergeTrailer folds an older section trailer under the newest one. The newest
// section wins per key and an older section fills only the keys the merged
// trailer lacks. /Prev and /Size describe one section, so an older one never
// supplies them.
func mergeTrailer(newest, older Value) Value {
	for key, entry := range older.Dict {
		if key == keyPrev || key == keySize {
			continue
		}
		if _, ok := newest.Dict[key]; !ok {
			newest.Dict[key] = entry
		}
	}
	return newest
}

// prevOffset reads one /Prev chain link. A missing /Prev ends the chain. A
// present /Prev that is not a non-negative integer is syntaxerror in xref.
func prevOffset(trailer Value) (int, bool, error) {
	entry, ok := trailer.ValueEntry(keyPrev)
	if !ok || entry.Kind == KindNull {
		return 0, false, nil
	}
	number, ok := intOf(entry)
	if !ok || number < 0 {
		return 0, false, NewError(opXRef, errSyntax)
	}
	return number, true, nil
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
	objNum, _, val, _, err := ParseIndirect(src, offset)
	if objNum < 0 {
		return nil, NullVal(), NewError(opXRef, errSyntax)
	}
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
		// An object stream carries its own index of the objects inside it, so a
		// number with no xref row but a place in a stream is still in the file.
		// A row that exists and is free is a deliberate deletion, and stays a
		// failure.
		if !ok {
			if value, found := file.packedObject(num); found {
				file.cache[num] = value
				return value, nil
			}
		}
		return NullVal(), NewError(opXRef, errSyntax)
	}
	file.busy[num] = true
	defer delete(file.busy, num)

	val, err := file.objectAt(num, entry)
	if err != nil {
		fixed, fixedErr := file.recoveredObject(num)
		if fixedErr != nil {
			return NullVal(), err
		}
		val = fixed
	}
	file.cache[num] = val
	return val, nil
}

// objectAt reads object num through one xref row.
func (file *File) objectAt(num int, entry XEntry) (Value, error) {
	if entry.Compressed {
		if value, ok := file.objStreamValue(num, entry.StreamNum, entry.StreamIdx); ok {
			return value, nil
		}
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return file.plainAt(num, entry.Offset, entry.Gen)
}

// recoveredObject reads object num from where the file itself says it is,
// ignoring the row: the object header scan first, then the objects an object
// stream carries. It is the reader's answer to a row whose bytes are not the
// object, the damage Ghostscript reports as an invalid xref entry and repairs
// by rebuilding the table.
func (file *File) recoveredObject(num int) (Value, error) {
	row, ok := file.recoveredRow(num)
	if !ok {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return file.objectAt(num, row)
}

// readable returns the object at num, and false when the xref row for num names
// an object the file does not carry. Such a row is a dead object number: the
// offset is out of range, the bytes there are not that object, or the row never
// existed. A survey that walks every in-use number uses this, so one dead number
// does not veto the whole survey. A reference to the same number still fails,
// because deref and every content path resolve strictly.
func (file *File) readable(num int) (Value, bool) {
	val, err := file.resolve(num)
	if err != nil {
		return NullVal(), false
	}
	return val, true
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

// recoveredRow returns a row for num rebuilt from the file itself: the object
// header scan first, then the objects an object stream carries. A compressed
// row is the answer when the object has no header of its own.
func (file *File) recoveredRow(num int) (XEntry, bool) {
	if row, ok := file.scannedEntry(num); ok {
		return row, true
	}
	return file.packedEntry(num)
}

// packedEntry returns the object stream row that carries num, or false. The
// object stream index is built once, on the first row that needs it.
func (file *File) packedEntry(num int) (XEntry, bool) {
	if file.packed == nil {
		file.packed = file.indexObjectStreams()
	}
	entry, ok := file.packed[num]
	return entry, ok
}

// packedObject reads object num out of the object stream that carries it. It is
// the fallback for a number the xref never indexed but an object stream holds.
func (file *File) packedObject(num int) (Value, bool) {
	row, ok := file.packedEntry(num)
	if !ok {
		return NullVal(), false
	}
	value, ok := file.objStreamValue(num, row.StreamNum, row.StreamIdx)
	return value, ok
}

// indexObjectStreams reads every object stream the header scan found and maps
// each object it carries to a compressed row. It is the recovery the reader
// uses when a row names no object and no plain header carries the number.
func (file *File) indexObjectStreams() map[int]XEntry {
	rows := map[int]XEntry{}
	if file.scan == nil {
		file.scan = scanObjectHeaders(file.src)
	}
	for _, streamNum := range file.sortedScanNums() {
		objects, ok := file.objectStreamObjects(file.scan[streamNum])
		if !ok {
			continue
		}
		for num, item := range objects {
			if _, seen := rows[num]; !seen {
				rows[num] = packedEntry(streamNum, item.index)
			}
		}
	}
	return rows
}

// sortedScanNums returns the scanned object numbers that are not themselves
// compressed, sorted so the object stream index is built the same way twice.
func (file *File) sortedScanNums() []int {
	streams := make([]int, 0, len(file.scan))
	for streamNum, entry := range file.scan {
		if !entry.Compressed {
			streams = append(streams, streamNum)
		}
	}
	sort.Ints(streams)
	return streams
}

// objectStreamObjects parses one object stream and returns the objects it
// carries, keyed by object number. A non-stream, a stream of another type, a
// missing /N or /First, a decode failure, and a malformed header each report
// false.
func (file *File) objectStreamObjects(entry XEntry) (map[int]stmItem, bool) {
	//nolint:dogsled // ParseIndirect returns five values; only the value is used here.
	_, _, val, _, err := ParseIndirect(file.src, entry.Offset)
	if err != nil || val.Kind != KindStream {
		return nil, false
	}
	if typeName, _ := val.NameEntry(keyType); typeName != keyObjStm {
		return nil, false
	}
	count, first, ok := streamBounds(val)
	if !ok {
		return nil, false
	}
	body, err := decodeStream(val)
	if err != nil {
		return nil, false
	}
	objects, err := splitObjStream(body, count, first)
	return objects, err == nil
}

// plainAt reads object num at offset and requires the header there to carry
// that number and generation.
func (file *File) plainAt(num, offset, gen int) (Value, error) {
	if offset < 0 || offset >= len(file.src) {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	got, gotGen, val, _, err := ParseIndirect(file.src, offset)
	if err != nil {
		return NullVal(), err
	}
	if got != num || gotGen != gen {
		return NullVal(), NewError(opXRef, errSyntax)
	}
	return val, nil
}

// scannedEntry returns the object-header scan row for num, building the scan on
// first use. It is the fallback for a row whose own offset is wrong.
func (file *File) scannedEntry(num int) (XEntry, bool) {
	if file.scan == nil {
		file.scan = scanObjectHeaders(file.src)
	}
	entry, ok := file.scan[num]
	return entry, ok
}

// objStreamValue reads object num from the object stream numbered streamNum and
// requires it at index. A missing stream, a bad header, a decode failure, and a
// different index all report false.
func (file *File) objStreamValue(num, streamNum, index int) (Value, bool) {
	objects, err := file.loadObjStream(streamNum)
	if err != nil {
		return NullVal(), false
	}
	item, ok := objects[num]
	if !ok || item.index != index {
		return NullVal(), false
	}
	return item.value, true
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
	if pos > 0 && !fileDelim(src[pos-1]) {
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
	return fileDelim(src[end])
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

func fileDelim(cur byte) bool {
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
