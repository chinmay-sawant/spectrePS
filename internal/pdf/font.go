package pdf

import (
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/font"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// errInvalidFont is the PostScript error name for a font with no outline
// source. documentation/fonts.md sets the standard-14 policy to this name.
const (
	errInvalidFont = "invalidfont"

	opFont = "font"

	keyWidths          = "Widths"
	keyFirstChar       = "FirstChar"
	keyBaseFont        = "BaseFont"
	keyMissingWidth    = "MissingWidth"
	keyFontDescriptor  = "FontDescriptor"
	keyEncoding        = "Encoding"
	keyBaseEncoding    = "BaseEncoding"
	keyDifferences     = "Differences"
	keyToUnicode       = "ToUnicode"
	keyFontFile        = "FontFile"
	keyFontFile2       = "FontFile2"
	keyFontFile3       = "FontFile3"
	keyDescendantFonts = "DescendantFonts"
	keyCIDToGIDMap     = "CIDToGIDMap"
	keyDW              = "DW"
	keyFlags           = "Flags"
	keyFont            = "Font"

	subtypeType0        = "Type0"
	subtypeType1        = "Type1"
	subtypeMMType1      = "MMType1"
	subtypeCIDFontType2 = "CIDFontType2"
	subtypeOpenType     = "OpenType"
	subtypeIdentity     = "Identity"
	encodingIdentityH   = "Identity-H"
	symbolicFontFlag    = 4
	defaultCIDWidth     = 1000
	emScale             = 1000
	cidMask             = 0xFFFF
	cidPairLen          = 2
	cidRangeLen         = 3

	keyLength1 = "Length1"
	keyLength2 = "Length2"
	keyLength3 = "Length3"
)

// Font is one loaded /Font resource. A simple font reads /Widths, /Encoding,
// and /FontDescriptor. A Type0 font reads /DescendantFonts, /CIDToGIDMap, and
// the descendant's /W and /DW.
//
// A font without an outline source still reports widths and Unicode. It
// paints nothing and the text machine reports invalidfont, the policy in
// documentation/fonts.md.
type Font struct {
	subtype  string
	baseFont string
	identity bool // Type0 with /Encoding /Identity-H

	metrics      *font.Metrics
	widths       map[byte]float64
	hasWidths    bool
	missingWidth float64
	encoding     [256]string

	cidToGID     map[uint32]uint16
	cidWidths    map[uint32]float64
	defaultWidth float64

	toUnicode map[uint32]string
	program   *sfnt.Font
	gidNames  map[string]sfnt.GlyphIndex

	// type1 is the decoded /FontFile program of a simple Type 1 font.
	// builtinEncoding is the program's built-in encoding, used for a
	// symbolic font before the PDF encoding.
	type1           *font.Type1Font
	builtinEncoding [256]string
}

// loadFont reads one resolved font dictionary. A dictionary that is not a
// font is typecheck. An unsupported descendant loads without an outline
// source, so painting reports invalidfont instead of failing the page.
func (file *File) loadFont(val Value) (*Font, error) {
	if val.Kind != KindDict {
		return nil, NewError(opFont, errType)
	}
	out := &Font{
		subtype:         "",
		baseFont:        "",
		identity:        false,
		metrics:         nil,
		widths:          nil,
		hasWidths:       false,
		missingWidth:    0,
		encoding:        [256]string{},
		cidToGID:        nil,
		cidWidths:       nil,
		defaultWidth:    defaultCIDWidth,
		toUnicode:       nil,
		program:         nil,
		gidNames:        nil,
		type1:           nil,
		builtinEncoding: [256]string{},
	}
	out.subtype, _ = val.NameEntry(keySubtype)
	out.baseFont, _ = val.NameEntry(keyBaseFont)

	desc, _ := val.ValueEntry(keyFontDescriptor)
	desc, err := file.deref(desc)
	if err != nil {
		return nil, err
	}
	if out.subtype == subtypeType0 {
		desc, err = file.loadType0(out, val)
		if err != nil {
			return nil, err
		}
		file.loadProgram(out, desc)
	} else {
		// The Type 1 built-in encoding feeds loadEncoding, so the
		// program loads before the simple font entries.
		file.loadProgram(out, desc)
		if err := file.loadSimple(out, val, desc); err != nil {
			return nil, err
		}
	}
	file.loadToUnicode(out, val)
	return out, nil
}

func (file *File) loadSimple(out *Font, val, desc Value) error {
	out.loadWidths(val, desc)
	if err := file.loadEncoding(out, val, desc); err != nil {
		return err
	}
	if metrics, ok := font.Standard14(out.baseFont); ok {
		out.metrics = metrics
	}
	return nil
}

// loadWidths reads /Widths, /FirstChar, and /MissingWidth.
func (f *Font) loadWidths(val, desc Value) {
	if width, ok := desc.IntEntry(keyMissingWidth); ok {
		f.missingWidth = float64(width)
	}
	items, ok := val.ArrayEntry(keyWidths)
	if !ok {
		return
	}
	f.hasWidths = true
	f.widths = map[byte]float64{}
	first, _ := val.IntEntry(keyFirstChar)
	for i, item := range items {
		code := first + i
		if code < 0 || code > 255 {
			continue
		}
		if width, ok := realOf(item); ok {
			f.widths[byte(code)] = width
		}
	}
}

// loadEncoding builds the code-to-glyph-name table from /Encoding and
// /Differences. A symbolic Type 1 font with no /Encoding starts from the
// program's built-in encoding.
func (file *File) loadEncoding(out *Font, val, desc Value) error {
	entry, ok := val.ValueEntry(keyEncoding)
	if !ok || entry.Kind == KindNull {
		out.encoding = out.defaultEncoding(desc)
		if out.symbolic(desc) && out.type1 != nil {
			out.encoding = out.builtinEncoding
		}
		return nil
	}
	entry, err := file.deref(entry)
	if err != nil {
		return err
	}
	if entry.Kind == KindName {
		out.encoding = namedEncoding(entry.Name)
		return nil
	}
	if entry.Kind != KindDict {
		return NewError(opFont, errType)
	}
	if base, ok := entry.NameEntry(keyBaseEncoding); ok {
		out.encoding = namedEncoding(base)
	}
	diffs, ok := entry.ArrayEntry(keyDifferences)
	if !ok {
		return nil
	}
	return applyDifferences(&out.encoding, diffs)
}

// symbolic reports whether the font dictionary marks the font symbolic. A
// symbolic font may use its built-in encoding, and the standard 14 Symbol and
// ZapfDingbats fonts are symbolic by name.
func (f *Font) symbolic(desc Value) bool {
	if f.baseFont == "Symbol" || f.baseFont == "ZapfDingbats" {
		return true
	}
	flags, ok := desc.IntEntry(keyFlags)
	return ok && flags&symbolicFontFlag != 0
}

// defaultEncoding is StandardEncoding for a nonsymbolic font and the empty
// built-in table for a symbolic one. The built-in tables of Symbol and
// ZapfDingbats are out of this ledger.
func (f *Font) defaultEncoding(desc Value) [256]string {
	if f.symbolic(desc) {
		return [256]string{}
	}
	return font.EncodingStandard.GlyphNames()
}

func namedEncoding(name string) [256]string {
	switch name {
	case "StandardEncoding":
		return font.EncodingStandard.GlyphNames()
	case "WinAnsiEncoding":
		return font.EncodingWinAnsi.GlyphNames()
	case "MacRomanEncoding":
		return font.EncodingMacRoman.GlyphNames()
	default:
		return [256]string{}
	}
}

func applyDifferences(table *[256]string, diffs []Value) error {
	code := 0
	for _, item := range diffs {
		if number, ok := intOf(item); ok {
			code = number
			continue
		}
		if item.Kind != KindName {
			return NewError(opFont, errType)
		}
		if code >= 0 && code < 256 {
			if item.Name == ".notdef" {
				table[code] = ""
			} else {
				table[code] = item.Name
			}
		}
		code++
	}
	return nil
}

// loadType0 reads the descendant font and returns the descriptor that holds
// the program. Only CIDFontType2 loads, and only Identity-H has a code space.
func (file *File) loadType0(out *Font, val Value) (Value, error) {
	encName, _ := val.NameEntry(keyEncoding)
	out.identity = encName == encodingIdentityH
	kids, ok := val.ArrayEntry(keyDescendantFonts)
	if !ok || len(kids) == 0 {
		return NullVal(), NewError(opFont, errInvalidFont)
	}
	kid, err := file.deref(kids[0])
	if err != nil {
		return NullVal(), err
	}
	if kid.Kind != KindDict {
		return NullVal(), NewError(opFont, errType)
	}
	if subtype, _ := kid.NameEntry(keySubtype); subtype != subtypeCIDFontType2 {
		return NullVal(), nil
	}
	out.loadCIDWidths(kid)
	if err := file.loadCIDToGID(out, kid); err != nil {
		return NullVal(), err
	}
	desc, _ := kid.ValueEntry(keyFontDescriptor)
	return file.deref(desc)
}

// loadCIDWidths reads /W and /DW. /W entries are "c [w...]" or "cFirst cLast w".
func (f *Font) loadCIDWidths(kid Value) {
	if width, ok := kid.IntEntry(keyDW); ok {
		f.defaultWidth = float64(width)
	}
	items, ok := kid.ArrayEntry(keyW)
	if !ok {
		return
	}
	f.cidWidths = map[uint32]float64{}
	for idx := 0; idx < len(items); {
		used := f.addCIDWidthEntry(items[idx:])
		if used == 0 {
			return
		}
		idx += used
	}
}

// addCIDWidthEntry reads one /W entry and returns the number of array items
// it used. A malformed entry returns 0.
func (f *Font) addCIDWidthEntry(items []Value) int {
	first, ok := intOf(items[0])
	if !ok || first < 0 || first > cidMask {
		return 0
	}
	if len(items) >= cidPairLen && items[1].Kind == KindArray {
		f.addCIDWidthSubset(items[1].Array, first)
		return cidPairLen
	}
	return f.addCIDWidthRange(items, first)
}

// addCIDWidthRange reads the "cFirst cLast w" form.
func (f *Font) addCIDWidthRange(items []Value, first int) int {
	if len(items) < cidRangeLen {
		return 0
	}
	last, okLast := intOf(items[1])
	width, okWidth := realOf(items[2])
	if !okLast || !okWidth || last < first || last-first > cidMask {
		return 0
	}
	for cid := first; cid <= last; cid++ {
		f.cidWidths[cidKey(cid)] = width
	}
	return cidRangeLen
}

// addCIDWidthSubset reads the "c [w...]" form.
func (f *Font) addCIDWidthSubset(items []Value, first int) {
	for index, item := range items {
		if width, ok := realOf(item); ok {
			f.cidWidths[cidKey(first+index)] = width
		}
	}
}

// cidKey converts a validated CID to the map key type.
func cidKey(cid int) uint32 {
	return uint32(cid & cidMask) //nolint:gosec // the mask bounds the value to 16 bits
}

// loadCIDToGID reads /CIDToGIDMap. A missing map, or the name /Identity, maps
// each CID to the glyph of the same index.
func (file *File) loadCIDToGID(out *Font, kid Value) error {
	entry, ok := kid.ValueEntry(keyCIDToGIDMap)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	if entry.Kind == KindName {
		return nil
	}
	stream, err := file.deref(entry)
	if err != nil {
		return err
	}
	if stream.Kind != KindStream {
		return nil
	}
	body, err := decodeStream(stream)
	if err != nil {
		return err
	}
	out.cidToGID = map[uint32]uint16{}
	for i := 0; i+1 < len(body); i += cidPairLen {
		out.cidToGID[cidKey(i/cidPairLen)] = uint16(body[i])<<byteShift | uint16(body[i+1])
	}
	return nil
}

// loadProgram reads /FontFile for a simple Type 1 font, then /FontFile2 and
// an OpenType /FontFile3. A /MMType1 font keeps its PDF widths and paints
// with no outline source. A missing or broken program still loads the font.
func (file *File) loadProgram(out *Font, desc Value) {
	if desc.Kind != KindDict || out.subtype == subtypeMMType1 {
		return
	}
	if out.subtype == subtypeType1 {
		if program := file.parseType1Program(desc); program != nil {
			out.type1 = program
			if out.symbolic(desc) {
				out.builtinEncoding = program.Encoding
			}
			return
		}
	}
	for _, key := range []string{keyFontFile2, keyFontFile3} {
		program := file.parseProgram(desc, key)
		if program == nil {
			continue
		}
		out.program = program
		out.gidNames = glyphNames(program)
		return
	}
}

// parseType1Program decodes the /FontFile stream with its /Length1, /Length2,
// and /Length3 entries. A missing stream, a stream that is not a stream, and
// a decode error all return nil.
func (file *File) parseType1Program(desc Value) *font.Type1Font {
	entry, ok := desc.ValueEntry(keyFontFile)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	stream, err := file.deref(entry)
	if err != nil || stream.Kind != KindStream {
		return nil
	}
	body, err := decodeStream(stream)
	if err != nil {
		return nil
	}
	var lengths [3]int
	for index, key := range []string{keyLength1, keyLength2, keyLength3} {
		if value, ok := stream.IntEntry(key); ok {
			lengths[index] = value
		}
	}
	program, err := font.LoadType1(body, lengths)
	if err != nil {
		return nil
	}
	return program
}

// parseProgram decodes one font file stream and parses it through sfnt. A
// missing stream, a deferred subtype, and a parse error all return nil.
func (file *File) parseProgram(desc Value, key string) *sfnt.Font {
	entry, ok := desc.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	stream, err := file.deref(entry)
	if err != nil || stream.Kind != KindStream {
		return nil
	}
	if key == keyFontFile3 {
		subtype, _ := stream.NameEntry(keySubtype)
		if subtype != subtypeOpenType {
			return nil
		}
	}
	body, err := decodeStream(stream)
	if err != nil {
		return nil
	}
	program, err := sfnt.Parse(body)
	if err != nil {
		return nil
	}
	return program
}

// loadToUnicode reads the /ToUnicode CMap. A malformed map is ignored and the
// encoding remains the fallback.
func (file *File) loadToUnicode(out *Font, val Value) {
	entry, ok := val.ValueEntry(keyToUnicode)
	if !ok || entry.Kind == KindNull {
		return
	}
	stream, err := file.deref(entry)
	if err != nil || stream.Kind != KindStream {
		return
	}
	body, err := decodeStream(stream)
	if err != nil {
		return
	}
	out.toUnicode = parseToUnicode(body)
}

// glyphNames reverses the program's glyph names, so a PDF encoding name can
// pick a glyph. Fonts without names leave the map empty and the cmap is the
// fallback.
func glyphNames(program *sfnt.Font) map[string]sfnt.GlyphIndex {
	if program == nil {
		return nil
	}
	var buf sfnt.Buffer
	names := map[string]sfnt.GlyphIndex{}
	for gid := sfnt.GlyphIndex(0); int(gid) < program.NumGlyphs(); gid++ {
		name, err := program.GlyphName(&buf, gid)
		if err != nil || name == "" || name == ".notdef" {
			continue
		}
		if _, exists := names[name]; !exists {
			names[name] = gid
		}
	}
	return names
}

// Width returns the advance of a character code in 1/1000 em. A simple font
// with /Widths reads the array, then /MissingWidth. A simple font without
// /Widths falls back to the standard 14 metrics by encoding name and then by
// code. A Type0 font reads /W and /DW.
func (f *Font) Width(code uint32) float64 {
	if f == nil {
		return 0
	}
	if f.twoByteCodes() {
		return f.cidWidth(code)
	}
	return f.simpleWidth(code)
}

func (f *Font) cidWidth(code uint32) float64 {
	if width, ok := f.cidWidths[code]; ok {
		return width
	}
	return f.defaultWidth
}

func (f *Font) simpleWidth(code uint32) float64 {
	if f.hasWidths {
		if width, ok := f.widths[byte(code)]; ok {
			return width
		}
		return f.missingWidth
	}
	if f.metrics != nil {
		if width, ok := f.metricWidth(code); ok {
			return width
		}
	}
	if width, ok := f.programAdvance(code); ok {
		return width
	}
	return f.missingWidth
}

// metricWidth looks up a standard 14 advance by encoding name, then by code.
func (f *Font) metricWidth(code uint32) (float64, bool) {
	if name := f.encoding[byte(code)]; name != "" {
		if width, ok := f.metrics.WidthByName(name); ok {
			return float64(width), true
		}
	}
	if width, ok := f.metrics.WidthByCode(byte(code)); ok {
		return float64(width), true
	}
	return 0, false
}

// Unicode returns the Unicode string for a character code. /ToUnicode wins.
// A simple font without one maps the resolved glyph name, through the PDF
// encoding and the built-in encoding of a symbolic font, with the Adobe Glyph
// List. A name the list does not carry leaves the code-point fallback to the
// extraction layer.
func (f *Font) Unicode(code uint32) (string, bool) {
	if f == nil {
		return "", false
	}
	if text, ok := f.toUnicode[code]; ok {
		return text, true
	}
	if f.twoByteCodes() {
		return "", false
	}
	name := f.encoding[byte(code)]
	if name == "" {
		name = f.builtinEncoding[byte(code)]
	}
	if name == "" {
		return "", false
	}
	return font.AGLUnicode(name)
}

// twoByteCodes reports whether character codes are two bytes.
func (f *Font) twoByteCodes() bool {
	return f != nil && f.subtype == subtypeType0
}

// paintSource reports whether the font can produce an outline for a code.
// Identity-H and a parsed program or Type 1 font are required; bare CFF and
// any other Type0 encoding are out of this ledger.
func (f *Font) paintSource() bool {
	if f == nil || (f.program == nil && f.type1 == nil) {
		return false
	}
	return !f.twoByteCodes() || f.identity
}

// glyphIndex maps a character code to a glyph index in the embedded sfnt
// program.
func (f *Font) glyphIndex(code uint32) (sfnt.GlyphIndex, bool) {
	if !f.paintSource() || f.program == nil {
		return 0, false
	}
	if f.twoByteCodes() {
		if f.cidToGID != nil {
			gid, ok := f.cidToGID[code]
			return sfnt.GlyphIndex(gid), ok
		}
		if code > math.MaxUint16 || int(code) >= f.program.NumGlyphs() {
			return 0, false
		}
		return sfnt.GlyphIndex(code), true
	}
	if name := f.encoding[byte(code)]; name != "" {
		if gid, ok := f.gidNames[name]; ok {
			return gid, true
		}
	}
	var buf sfnt.Buffer
	gid, err := f.program.GlyphIndex(&buf, rune(code))
	if err != nil {
		return 0, false
	}
	return gid, true
}

// outlinePPEM is the reference size glyphs load at. Coordinates come back in
// 26.6 pixels at this ppem, so the text machine divides by it to reach em
// units. The ppem stays small so the 26.6 multiplication inside sfnt cannot
// overflow.
const outlinePPEM = 64

// outline loads the glyph segments at outlinePPEM. The segments have Y
// growing down. A Type 1 font interprets its charstring and converts the
// result to the same shape.
func (f *Font) outline(code uint32) (sfnt.Segments, bool) {
	if f.type1 != nil && !f.twoByteCodes() {
		return f.type1Outline(code)
	}
	gid, ok := f.glyphIndex(code)
	if !ok {
		return nil, false
	}
	var buf sfnt.Buffer
	segments, err := f.program.LoadGlyph(&buf, gid, fixed.I(outlinePPEM), nil)
	if err != nil {
		return nil, false
	}
	return segments, true
}

// programAdvance returns the glyph advance from the embedded program in
// 1/1000 em. It is the fallback for a font without /Widths.
func (f *Font) programAdvance(code uint32) (float64, bool) {
	if f.type1 != nil && !f.twoByteCodes() {
		return f.type1Advance(code)
	}
	if f.program == nil {
		return 0, false
	}
	gid, ok := f.glyphIndex(code)
	if !ok {
		return 0, false
	}
	var buf sfnt.Buffer
	advance, err := f.program.GlyphAdvance(&buf, gid, fixed.I(outlinePPEM), xfont.HintingNone)
	if err != nil {
		return 0, false
	}
	return float64(advance) / fixedShift / outlinePPEM * emScale, true
}

// realOf returns a number from an int or real value.
func realOf(val Value) (float64, bool) {
	if val.Kind == KindInt {
		return float64(val.Int), true
	}
	if val.Kind == KindReal {
		return val.Real, true
	}
	return 0, false
}
