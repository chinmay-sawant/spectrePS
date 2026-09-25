package pdf

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"unicode/utf16"

	"github.com/chinmay-sawant/spectrePS/internal/font"
	"golang.org/x/image/font/sfnt"
)

// toUnicode limits from the PDF CMap specification: one bfchar or bfrange
// section carries at most 100 entries.
const (
	keyLength          = "Length"
	keyLastChar        = "LastChar"
	cmapSectionLimit   = 100
	simpleCodeDigits   = 2
	cidCodeDigits      = 4
	tfOperands         = 2
	widthEntriesPerCID = 2
	cmapHeaderLines    = "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\nbegincmap\n" +
		"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n" +
		"/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n"
	cmapTrailer = "endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n"
)

// SubsetFontObjects returns complete replacement bodies for the font objects
// a subsetting rewrite can shrink, plus the bodies it appends after them.
// firstNum is the object number of the first appended body. A caller hands
// the result to pdfout.CopyOptions: the overrides replace font dictionaries
// by number and the appended bodies add the new programs and /ToUnicode
// streams.
//
// Only a /FontFile2 TrueType program is subsetted. Glyph indices do not
// change, so content streams, /Widths, /Differences, /Encoding, and
// /CIDToGIDMap stay valid without a page re-encode. A /FontFile3 /OpenType
// program is copied whole, and a font with no embedded program, a Type 1
// /FontFile program, a font whose codes reach the collector cap, an unused
// font, and a font with no subsettable glyf outlines are copied unchanged.
// The overrides are deterministic: equal input returns equal output bytes.
// A canceled context returns ctx.Err() and nil.
// A nil context panics with "pdf: nil context".
func (file *File) SubsetFontObjects(
	ctx context.Context,
	firstNum int,
) (map[int][]byte, [][]byte, error) {
	if ctx == nil {
		panic(panicNilCtx)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if file == nil || firstNum < 1 {
		return nil, nil, nil
	}
	plans := file.subsetPlans()
	if len(plans) == 0 {
		return nil, nil, nil
	}
	writer := &subsetWriter{
		file:      file,
		overrides: map[int][]byte{},
		appended:  nil,
		first:     firstNum,
	}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		writer.font(plan)
	}
	return writer.overrides, writer.appended, nil
}

// subsetFont is one font object the writer may replace: its number, its
// dictionary, the loaded font, and the union of the codes every page showed
// for it.
type subsetFont struct {
	num   int
	dict  Value
	font  *Font
	codes []uint32
}

// fontRef is one page /Font resource name: the font object number and the
// loaded font. An inline font dictionary has number 0 and is never replaced.
type fontRef struct {
	num  int
	font *Font
}

// subsetPlans returns the font objects to subset, in object-number order.
func (file *File) subsetPlans() []*subsetFont {
	byNum := map[int]*subsetFont{}
	order := make([]int, 0)
	for page, names := range file.FontUsedCodes() {
		file.planPage(page, names, byNum, &order)
	}
	sort.Ints(order)
	out := make([]*subsetFont, 0, len(order))
	for _, num := range order {
		val, ok, err := file.ObjectValue(num)
		if err != nil || !ok || val.Kind != KindDict {
			continue
		}
		plan := byNum[num]
		plan.dict = val
		out = append(out, plan)
	}
	return out
}

// planPage adds one page's used codes to the plan for each named font. A name
// that resolves to an inline dictionary, a missing font, an unused name, and
// a name the collector capped contribute nothing.
func (file *File) planPage(
	page int,
	names map[string][]uint32,
	byNum map[int]*subsetFont,
	order *[]int,
) {
	refs := file.pageFontObjects(page)
	for name, codes := range names {
		ref, ok := refs[name]
		if !ok || ref.num <= 0 || ref.font == nil || len(codes) == 0 || len(codes) >= maxUsedCodes {
			continue
		}
		plan, ok := byNum[ref.num]
		if !ok {
			plan = &subsetFont{num: ref.num, dict: NullVal(), font: ref.font, codes: nil}
			byNum[ref.num] = plan
			*order = append(*order, ref.num)
		}
		plan.codes = mergeUsedCodes(plan.codes, codes)
	}
}

// pageFontObjects reads one page's /Font names, object numbers, and loaded
// fonts. A page whose resources fail to resolve returns nil.
func (file *File) pageFontObjects(index int) map[string]fontRef {
	res, err := file.PageResources(index)
	if err != nil {
		return nil
	}
	nums := file.fontObjectNums(index)
	out := make(map[string]fontRef, len(res.Fonts))
	for name, fnt := range res.Fonts {
		out[name] = fontRef{num: nums[name], font: fnt}
	}
	return out
}

// fontObjectNums returns the object number of every indirect /Font entry.
// An inline font dictionary contributes no number.
func (file *File) fontObjectNums(index int) map[string]int {
	out := map[string]int{}
	sub, ok := file.pageFontDict(index)
	if !ok {
		return out
	}
	for name, item := range sub.Dict {
		if item.Kind == KindRef {
			out[name] = item.RefNum
		}
	}
	return out
}

// pageFontDict resolves one page's raw /Font subdictionary.
func (file *File) pageFontDict(index int) (Value, bool) {
	if index < 0 || index >= len(file.resources) {
		return NullVal(), false
	}
	node, err := file.deref(file.resources[index])
	if err != nil || node.Kind != KindDict {
		return NullVal(), false
	}
	entry, ok := node.ValueEntry(keyFont)
	if !ok || entry.Kind == KindNull {
		return NullVal(), false
	}
	sub, err := file.deref(entry)
	if err != nil || sub.Kind != KindDict {
		return NullVal(), false
	}
	return sub, true
}

// mergeUsedCodes returns the sorted union of two sorted code slices.
func mergeUsedCodes(left, right []uint32) []uint32 {
	if len(left) == 0 {
		return append([]uint32(nil), right...)
	}
	if len(right) == 0 {
		return left
	}
	seen := make(map[uint32]bool, len(left)+len(right))
	for _, code := range left {
		seen[code] = true
	}
	for _, code := range right {
		seen[code] = true
	}
	out := make([]uint32, 0, len(seen))
	for code := range seen {
		out = append(out, code)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// subsetWriter accumulates the replacement and appended bodies.
type subsetWriter struct {
	file      *File
	overrides map[int][]byte
	appended  [][]byte
	first     int
}

// add appends one complete body and returns its object number.
func (writer *subsetWriter) add(body []byte) int {
	writer.appended = append(writer.appended, body)
	return writer.first + len(writer.appended) - 1
}

// font replaces one font dictionary.
func (writer *subsetWriter) font(plan *subsetFont) {
	subtype, _ := plan.dict.NameEntry(keySubtype)
	if subtype == subtypeType0 {
		writer.type0Font(plan)
		return
	}
	writer.simpleFont(plan)
}

// simpleFont replaces a simple font dictionary with a subset programme, the
// used-code widths, and a /ToUnicode map. A /FontFile3 OpenType program is
// copied whole: the descriptor keeps its reference and the dictionary is
// still rewritten for widths and Unicode.
func (writer *subsetWriter) simpleFont(plan *subsetFont) {
	desc, ok := writer.descriptor(plan.dict)
	if !ok {
		return
	}
	program, ok := writer.fontProgram(desc)
	if !ok {
		// A Type 1 /FontFile program is out of this ledger.
		return
	}
	entries := copyEntries(plan.dict)
	if program.key == keyFontFile2 {
		subset, name, ok := writer.subsetProgram(program.body, plan.font, plan.codes)
		if !ok {
			return
		}
		programNum := writer.add(programStream(program, subset))
		entries[keyBaseFont] = NameVal(name)
		entries[keyFontDescriptor] = DictVal(writer.subsetDescriptor(desc, program, name, programNum))
	}
	writeSimpleWidths(entries, plan.font, plan.codes)
	if cmapNum := writer.toUnicodeObject(plan.font, plan.codes, false); cmapNum > 0 {
		entries[keyToUnicode] = RefVal(cmapNum, 0)
	}
	writer.overrides[plan.num] = SerializeValue(DictVal(entries))
}

// type0Font replaces a Type0 font dictionary. The CIDs and the
// /CIDToGIDMap keep their meaning because the glyph indices do not change,
// and /W is trimmed to the used CIDs.
func (writer *subsetWriter) type0Font(plan *subsetFont) {
	kid, ok := writer.descendant(plan.dict)
	if !ok {
		return
	}
	desc, ok := writer.descriptor(kid)
	if !ok {
		return
	}
	program, ok := writer.fontProgram(desc)
	if !ok {
		return
	}
	name := plan.font.baseFont
	newKid := copyEntries(kid)
	if program.key == keyFontFile2 {
		subset, subnet, ok := writer.subsetProgram(program.body, plan.font, plan.codes)
		if !ok {
			return
		}
		name = subnet
		programNum := writer.add(programStream(program, subset))
		newKid[keyFontDescriptor] = DictVal(writer.subsetDescriptor(desc, program, name, programNum))
	}
	newKid[keyBaseFont] = NameVal(name)
	writeCIDWidths(newKid, plan.font, plan.codes)
	entries := copyEntries(plan.dict)
	entries[keyBaseFont] = NameVal(name)
	entries[keyDescendantFonts] = ArrayVal([]Value{DictVal(newKid)})
	if cmapNum := writer.toUnicodeObject(plan.font, plan.codes, true); cmapNum > 0 {
		entries[keyToUnicode] = RefVal(cmapNum, 0)
	}
	writer.overrides[plan.num] = SerializeValue(DictVal(entries))
}

// descriptor resolves the /FontDescriptor dictionary of one font or CID font.
func (writer *subsetWriter) descriptor(node Value) (Value, bool) {
	entry, ok := node.ValueEntry(keyFontDescriptor)
	if !ok || entry.Kind == KindNull {
		return NullVal(), false
	}
	desc, err := writer.file.deref(entry)
	if err != nil || desc.Kind != KindDict {
		return NullVal(), false
	}
	return desc, true
}

// descendant resolves the first /DescendantFonts dictionary of a Type0 font.
func (writer *subsetWriter) descendant(node Value) (Value, bool) {
	kids, ok := node.ArrayEntry(keyDescendantFonts)
	if !ok || len(kids) == 0 {
		return NullVal(), false
	}
	kid, err := writer.file.deref(kids[0])
	if err != nil || kid.Kind != KindDict {
		return NullVal(), false
	}
	subtype, _ := kid.NameEntry(keySubtype)
	if subtype != subtypeCIDFontType2 {
		return NullVal(), false
	}
	return kid, true
}

// fontProgram is one resolved outline source: its descriptor key, decoded
// bytes, and source stream dictionary.
type fontProgram struct {
	key  string
	body []byte
	dict map[string]Value
}

// fontProgram resolves /FontFile2, then an OpenType /FontFile3. A /FontFile
// Type 1 program, a CFF /FontFile3, a missing stream, and a decode error all
// return false.
func (writer *subsetWriter) fontProgram(desc Value) (fontProgram, bool) {
	for _, key := range []string{keyFontFile2, keyFontFile3} {
		stream, ok := writer.streamEntry(desc, key)
		if !ok {
			continue
		}
		if key == keyFontFile3 {
			subtype, _ := stream.NameEntry(keySubtype)
			if subtype != subtypeOpenType {
				continue
			}
		}
		body, err := decodeStream(stream)
		if err != nil {
			continue
		}
		return fontProgram{key: key, body: body, dict: stream.Dict}, true
	}
	return fontProgram{key: "", body: nil, dict: nil}, false
}

// streamEntry resolves one descriptor entry to its stream value.
func (writer *subsetWriter) streamEntry(desc Value, key string) (Value, bool) {
	entry, ok := desc.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return NullVal(), false
	}
	stream, err := writer.file.deref(entry)
	if err != nil || stream.Kind != KindStream {
		return NullVal(), false
	}
	return stream, true
}

// subsetProgram subsets one TrueType program and returns the bytes and the
// tagged /BaseFont name.
func (writer *subsetWriter) subsetProgram(
	program []byte,
	fnt *Font,
	codes []uint32,
) ([]byte, string, bool) {
	subset, err := font.Subset(program, usedGlyphIDs(fnt, codes))
	if err != nil {
		return nil, "", false
	}
	return subset, subnetName(font.SubsetTag(subset), fnt.baseFont), true
}

// usedGlyphIDs maps the used codes to glyph indices. Glyph 0 stays, and the
// result is sorted so the subset bytes are deterministic.
func usedGlyphIDs(fnt *Font, codes []uint32) []sfnt.GlyphIndex {
	seen := map[sfnt.GlyphIndex]bool{0: true}
	out := make([]sfnt.GlyphIndex, 0, len(codes)+1)
	out = append(out, 0)
	for _, code := range codes {
		gid, ok := fnt.glyphIndex(code)
		if !ok || seen[gid] {
			continue
		}
		seen[gid] = true
		out = append(out, gid)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// programStream writes the appended program stream. The source dictionary is
// kept, minus the filter entries, because the bytes are already decoded.
func programStream(program fontProgram, subset []byte) []byte {
	dict := make(map[string]Value, len(program.dict)+1)
	for key, entry := range program.dict {
		dict[key] = entry
	}
	delete(dict, keyFilter)
	delete(dict, keyParms)
	delete(dict, keyLength)
	if program.key == keyFontFile2 {
		dict[keyLength1] = IntVal(int64(len(subset)))
	}
	return SerializeValue(StreamVal(dict, subset))
}

// subsetDescriptor copies one descriptor with the new program reference and
// the tagged /FontName. The whole dictionary stays direct, so nothing shared
// with another font is replaced.
func (writer *subsetWriter) subsetDescriptor(
	desc Value,
	program fontProgram,
	name string,
	programNum int,
) map[string]Value {
	entries := copyEntries(desc)
	entries[program.key] = RefVal(programNum, 0)
	entries["FontName"] = NameVal(name)
	return entries
}

// writeSimpleWidths writes /FirstChar, /LastChar, and /Widths for the used
// codes. The codes outside the used span get the font's own fallback width.
func writeSimpleWidths(entries map[string]Value, fnt *Font, codes []uint32) {
	if len(codes) == 0 || codes[len(codes)-1] > 255 {
		return
	}
	first := codes[0]
	last := codes[len(codes)-1]
	widths := make([]Value, 0, last-first+1)
	for code := first; code <= last; code++ {
		widths = append(widths, widthValue(fnt.Width(code)))
	}
	entries[keyFirstChar] = IntVal(int64(first))
	entries[keyLastChar] = IntVal(int64(last))
	entries[keyWidths] = ArrayVal(widths)
}

// writeCIDWidths writes /DW and the /W array trimmed to the used CIDs. A run
// of consecutive CIDs with one width becomes the "cFirst cLast w" form.
func writeCIDWidths(entries map[string]Value, fnt *Font, codes []uint32) {
	if len(codes) == 0 {
		return
	}
	entries[keyDW] = widthValue(fnt.defaultWidth)
	entries[keyW] = ArrayVal(cidWidthEntries(fnt, codes))
}

// cidWidthEntries groups the used CIDs into the /W entries.
func cidWidthEntries(fnt *Font, codes []uint32) []Value {
	out := make([]Value, 0, widthEntriesPerCID*len(codes))
	for idx := 0; idx < len(codes); {
		first := idx
		width := fnt.Width(codes[idx])
		for idx+1 < len(codes) && codes[idx+1] == codes[idx]+1 && fnt.Width(codes[idx+1]) == width {
			idx++
		}
		if first == idx {
			out = append(out, IntVal(int64(codes[first])), ArrayVal([]Value{widthValue(width)}))
		} else {
			out = append(out, IntVal(int64(codes[first])), IntVal(int64(codes[idx])), widthValue(width))
		}
		idx++
	}
	return out
}

// widthValue writes a width as an integer when it is whole.
func widthValue(width float64) Value {
	if width == math.Trunc(width) && math.Abs(width) < math.MaxInt32 {
		return IntVal(int64(width))
	}
	return RealVal(width)
}

// toUnicodeObject appends the /ToUnicode CMap stream for the used codes and
// returns its object number, or 0 when no code has a mapping.
func (writer *subsetWriter) toUnicodeObject(fnt *Font, codes []uint32, twoByte bool) int {
	pairs := unicodePairs(fnt, codes, twoByte)
	if len(pairs) == 0 {
		return 0
	}
	return writer.add(SerializeValue(StreamVal(map[string]Value{}, synthToUnicode(pairs, twoByte))))
}

// unicodePairs maps each used code to its Unicode text. A code with no
// mapping falls back to the glyph name for a Type0 font and to the code point
// for a simple font, which is what the extraction layer shows.
func unicodePairs(fnt *Font, codes []uint32, twoByte bool) map[uint32]string {
	out := make(map[uint32]string, len(codes))
	var buf sfnt.Buffer
	for _, code := range codes {
		text, ok := fnt.Unicode(code)
		if !ok || text == "" {
			text = glyphNameUnicode(fnt, code, twoByte, &buf)
		}
		if text != "" {
			out[code] = text
		}
	}
	return out
}

// glyphNameUnicode is the fallback mapping: the glyph name through the Adobe
// Glyph List for a Type0 font, and the code point itself for a simple font.
func glyphNameUnicode(fnt *Font, code uint32, twoByte bool, buf *sfnt.Buffer) string {
	if twoByte {
		gid, ok := fnt.glyphIndex(code)
		if !ok || fnt.program == nil {
			return ""
		}
		name, err := fnt.program.GlyphName(buf, gid)
		if err != nil {
			return ""
		}
		text, _ := font.AGLUnicode(name)
		return text
	}
	return string(rune(code))
}

// cmapChar is one bfchar entry.
type cmapChar struct {
	code uint32
	text string
}

// cmapRange is one bfrange entry.
type cmapRange struct {
	first uint32
	last  uint32
	text  string
}

// synthToUnicode writes a /ToUnicode CMap. Consecutive codes whose text
// advances by one UTF-16 unit become a bfrange; every other code becomes a
// bfchar. Sections carry at most cmapSectionLimit entries.
func synthToUnicode(pairs map[uint32]string, twoByte bool) []byte {
	codes := make([]uint32, 0, len(pairs))
	for code := range pairs {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	chars, ranges := splitUnicode(pairs, codes)
	digits := simpleCodeDigits
	space := "<00> <FF>"
	if twoByte {
		digits = cidCodeDigits
		space = "<0000> <FFFF>"
	}
	var buf bytes.Buffer
	buf.WriteString(cmapHeaderLines)
	buf.WriteString("1 begincodespacerange\n")
	buf.WriteString(space)
	buf.WriteString("\nendcodespacerange\n")
	writeBFChar(&buf, chars, digits)
	writeBFRange(&buf, ranges, digits)
	buf.WriteString(cmapTrailer)
	return buf.Bytes()
}

// splitUnicode splits the sorted pairs into bfchar and bfrange entries.
func splitUnicode(pairs map[uint32]string, codes []uint32) ([]cmapChar, []cmapRange) {
	chars := make([]cmapChar, 0, len(codes))
	ranges := make([]cmapRange, 0)
	for idx := 0; idx < len(codes); {
		first := idx
		units := utf16.Encode([]rune(pairs[codes[idx]]))
		for idx+1 < len(codes) && codes[idx+1] == codes[idx]+1 {
			delta := codes[idx+1] - codes[first]
			if cmapIncrement(units, delta) != pairs[codes[idx+1]] {
				break
			}
			idx++
		}
		if first == idx {
			chars = append(chars, cmapChar{code: codes[first], text: pairs[codes[first]]})
		} else {
			ranges = append(ranges, cmapRange{
				first: codes[first],
				last:  codes[idx],
				text:  pairs[codes[first]],
			})
		}
		idx++
	}
	return chars, ranges
}

// writeBFChar writes the bfchar entries in sections of cmapSectionLimit.
func writeBFChar(buf *bytes.Buffer, chars []cmapChar, digits int) {
	for start := 0; start < len(chars); start += cmapSectionLimit {
		end := min(start+cmapSectionLimit, len(chars))
		fmt.Fprintf(buf, "%d beginbfchar\n", end-start)
		for _, entry := range chars[start:end] {
			fmt.Fprintf(buf, "<%0*X> <%s>\n", digits, entry.code, utf16Hex(entry.text))
		}
		buf.WriteString("endbfchar\n")
	}
}

// writeBFRange writes the bfrange entries in sections of cmapSectionLimit.
func writeBFRange(buf *bytes.Buffer, ranges []cmapRange, digits int) {
	for start := 0; start < len(ranges); start += cmapSectionLimit {
		end := min(start+cmapSectionLimit, len(ranges))
		fmt.Fprintf(buf, "%d beginbfrange\n", end-start)
		for _, entry := range ranges[start:end] {
			fmt.Fprintf(
				buf,
				"<%0*X> <%0*X> <%s>\n",
				digits, entry.first, digits, entry.last, utf16Hex(entry.text),
			)
		}
		buf.WriteString("endbfrange\n")
	}
}

// utf16Hex writes one string as UTF-16BE hex, the ToUnicode destination form.
func utf16Hex(text string) string {
	units := utf16.Encode([]rune(text))
	var buf bytes.Buffer
	for _, unit := range units {
		fmt.Fprintf(&buf, "%04X", unit)
	}
	return buf.String()
}

// subnetName is the tagged /BaseFont of a subset font.
func subnetName(tag, base string) string {
	if base == "" {
		base = "Font"
	}
	return tag + "+" + base
}

// copyEntries copies one dictionary so an override can edit its entries.
func copyEntries(val Value) map[string]Value {
	entries := make(map[string]Value, len(val.Dict)+1)
	for key, entry := range val.Dict {
		entries[key] = entry
	}
	return entries
}
