package font

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Type 1 container and cipher constants. The eexec seed, the charstring seed,
// and the default lenIV come from the Adobe Type 1 Font Format.
const (
	type1SeedEexec      = 55665
	type1SeedCharstring = 4330
	type1Cipher1        = 52845
	type1Cipher2        = 22719
	type1CipherShift    = 8
	type1DefaultLenIV   = 4
	type1PfbMarker      = 0x80
	type1PFBClear       = 1
	type1PFBBinary      = 2
	type1PFBEnd         = 3
	type1PFBHeaderLen   = 6
	type1RandomBytes    = 4
	type1DigitsPerByte  = 2
	type1FontMatrixLen  = 6
	type1ArrayHeader    = 2
	type1SubrEntryLen   = 4
	type1CharEntryLen   = 3
	type1Candidates     = 2
	type1MaxSubrs       = 65535
	type1MaxGlyphs      = 65535
	type1MaxLenIV       = 4
)

// Errors the decoder reports. LoadType1 wraps them with the failing step, so
// errors.Is keeps working.
var (
	errType1Truncated     = errors.New("font: truncated Type 1 program")
	errType1Container     = errors.New("font: unrecognized Type 1 container")
	errType1Syntax        = errors.New("font: malformed Type 1 program")
	errType1NoCharStrings = errors.New("font: Type 1 program has no CharStrings")
	errType1Limit         = errors.New("font: Type 1 program exceeds a decoder limit")
)

// Type1Point is one point in glyph space. The usual FontMatrix of
// [0.001 0 0 0.001 0 0] leaves the numbers in 1/1000 em.
type Type1Point struct {
	X float64
	Y float64
}

// Type1Op is one outline command.
type Type1Op uint8

const (
	// Type1MoveTo starts a subpath at one point.
	Type1MoveTo Type1Op = iota
	// Type1LineTo draws a line to one point.
	Type1LineTo
	// Type1CurveTo draws a cubic curve through two control points to one
	// endpoint.
	Type1CurveTo
)

// Type1Segment is one outline command with its points. A move or line carries
// one point, a curve three.
type Type1Segment struct {
	Op   Type1Op
	Args []Type1Point
}

// Type1Glyph is one interpreted charstring. Advance is the width from the
// first hsbw or sbw in 1/1000 em, and HasWidth is false when the charstring
// carried no width at all.
type Type1Glyph struct {
	Segments []Type1Segment
	Advance  float64
	HasWidth bool
}

// Type1Font is one decoded Type 1 font program. LoadType1 fills it from a PFA
// or PFB container. The zero value is not usable.
type Type1Font struct {
	// Name is the /FontName entry, empty when the program omits it.
	Name string
	// FontMatrix is the /FontMatrix entry. A program that omits it keeps
	// the identity matrix, which the decoder does not treat as an error.
	FontMatrix [6]float64
	// Encoding is the built-in /Encoding table. A code the program leaves
	// undefined holds the empty string.
	Encoding [256]string

	charStrings map[string][]byte
	subrs       [][]byte
	lenIV       int
	weight      []float64
}

// HasGlyph reports whether the charstrings dictionary names one glyph.
func (f *Type1Font) HasGlyph(name string) bool {
	if f == nil {
		return false
	}
	_, ok := f.charStrings[name]
	return ok
}

// Glyph names one glyph's charstring, runs the interpreter, and returns the
// outline in glyph space with the width in 1/1000 em. A malformed charstring,
// a seac whose component is missing, and an unknown name all return an error.
func (f *Type1Font) Glyph(name string) (Type1Glyph, error) {
	if f == nil || !f.HasGlyph(name) {
		return Type1Glyph{}, fmt.Errorf("%w: no glyph %q", errType1Syntax, name)
	}
	return f.glyphNamed(name, true)
}

// LoadType1 decodes one PFA or PFB font program. lengths carries the PDF
// stream's /Length1, /Length2, and /Length3, or zeros when the caller has
// none; the decoder prefers them and falls back to the eexec and cleartomark
// markers. The eexec and charstring ciphers use the fixed seeds of the Type 1
// format.
func LoadType1(data []byte, lengths [3]int) (*Type1Font, error) {
	if len(data) == 0 {
		return nil, errType1Truncated
	}
	if data[0] == type1PfbMarker {
		clearText, encrypted, err := splitPFB(data)
		if err != nil {
			return nil, err
		}
		return parseType1Sections(clearText, encrypted)
	}
	candidates := pfaCandidates(data, lengths)
	var firstErr error
	for _, candidate := range candidates {
		program, err := parseType1Sections(candidate.clearText, candidate.encrypted)
		if err == nil {
			return program, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		firstErr = errType1Truncated
	}
	return nil, firstErr
}

// type1Section is one cleartext and encrypted pair from a container.
type type1Section struct {
	clearText []byte
	encrypted []byte
}

// splitPFB reads the segment headers of a PFB file. Segment 1 is cleartext,
// segment 2 the eexec ciphertext, and segment 3 the trailer that is dropped.
func splitPFB(data []byte) ([]byte, []byte, error) {
	var clearText, encrypted []byte
	pos := 0
	for pos+type1PFBHeaderLen <= len(data) {
		body, kind, next, err := t1PFBSegment(data, pos)
		if err != nil {
			return nil, nil, err
		}
		pos = next
		switch kind {
		case type1PFBClear:
			clearText = append(clearText, body...)
		case type1PFBBinary:
			encrypted = append(encrypted, body...)
		case type1PFBEnd:
			return t1PFBResult(clearText, encrypted)
		}
	}
	return t1PFBResult(clearText, encrypted)
}

// t1PFBSegment reads one 0x80 header and returns its body and next offset.
func t1PFBSegment(data []byte, pos int) ([]byte, byte, int, error) {
	if data[pos] != type1PfbMarker {
		return nil, 0, 0, fmt.Errorf("%w: PFB segment marker", errType1Container)
	}
	kind := data[pos+1]
	size := int(binary.LittleEndian.Uint32(data[pos+type1ArrayHeader : pos+type1PFBHeaderLen]))
	next := pos + type1PFBHeaderLen
	if size < 0 || next+size > len(data) {
		return nil, 0, 0, fmt.Errorf("%w: PFB segment length", errType1Truncated)
	}
	return data[next : next+size], kind, next + size, nil
}

// t1PFBResult rejects a PFB that is missing either half.
func t1PFBResult(clearText, encrypted []byte) ([]byte, []byte, error) {
	if len(clearText) == 0 || len(encrypted) == 0 {
		return nil, nil, fmt.Errorf("%w: PFB without clear or encrypted data", errType1Truncated)
	}
	return clearText, encrypted, nil
}

// pfaCandidates returns the cleartext and encrypted slices to try, in order.
// A stream with /Length1-3 is tried first, then the eexec and cleartomark
// markers. The candidates differ when a producer wrote the lengths for the
// raw ciphertext but stored hex, so the caller tries each.
func pfaCandidates(data []byte, lengths [3]int) []type1Section {
	out := make([]type1Section, 0, type1Candidates)
	if lengths[0] > 0 && lengths[1] > 0 && lengths[0]+lengths[1] <= len(data) {
		out = append(out, type1Section{
			clearText: data[:lengths[0]],
			encrypted: data[lengths[0] : lengths[0]+lengths[1]],
		})
	}
	if start, end, ok := pfaMarkers(data); ok {
		out = append(out, type1Section{clearText: data[:start], encrypted: data[end:]})
	}
	return out
}

// pfaMarkers locates the eexec keyword and the cleartomark that closes the
// encrypted section. The returned start is the index of the eexec keyword and
// end the first ciphertext byte after its delimiter.
func pfaMarkers(data []byte) (int, int, bool) {
	start := indexType1Keyword(data, "eexec")
	if start < 0 {
		return 0, 0, false
	}
	end := start + len("eexec")
	if end < len(data) && isType1Space(data[end]) {
		end++
	}
	limit := len(data)
	if mark := indexAfter(data, "cleartomark", end); mark >= 0 {
		limit = mark
	}
	return start, end, limit > end
}

// indexType1Keyword returns the index of a bare keyword, or -1.
func indexType1Keyword(data []byte, keyword string) int {
	for start := 0; start+len(keyword) <= len(data); start++ {
		if string(data[start:start+len(keyword)]) != keyword {
			continue
		}
		if t1KeywordBoundary(data, start, len(keyword)) {
			return start
		}
	}
	return -1
}

// t1KeywordBoundary reports whether the keyword at start is delimited.
func t1KeywordBoundary(data []byte, start, size int) bool {
	before := byte(' ')
	if start > 0 {
		before = data[start-1]
	}
	after := byte(' ')
	if start+size < len(data) {
		after = data[start+size]
	}
	return !isType1Regular(before) && !isType1Regular(after)
}

// indexAfter finds one keyword at or after start.
func indexAfter(data []byte, keyword string, start int) int {
	for pos := start; pos+len(keyword) <= len(data); pos++ {
		if string(data[pos:pos+len(keyword)]) == keyword {
			return pos
		}
	}
	return -1
}

// parseType1Sections decrypts and parses one cleartext and ciphertext pair.
func parseType1Sections(clearText, encrypted []byte) (*Type1Font, error) {
	ciphertext, err := type1CipherBytes(encrypted)
	if err != nil {
		return nil, err
	}
	plain := type1Decrypt(ciphertext, type1SeedEexec)
	if len(plain) <= type1RandomBytes {
		return nil, fmt.Errorf("%w: eexec body too short", errType1Truncated)
	}
	private := cutType1Trailer(plain[type1RandomBytes:])
	clearTokens, err := type1Tokenize(clearText)
	if err != nil {
		return nil, err
	}
	privateTokens, err := type1Tokenize(private)
	if err != nil {
		return nil, err
	}
	program := &Type1Font{
		Name:        "",
		FontMatrix:  [6]float64{},
		Encoding:    [256]string{},
		charStrings: nil,
		subrs:       nil,
		lenIV:       type1DefaultLenIV,
		weight:      nil,
	}
	parseType1Dictionary(clearTokens, program)
	parseType1Dictionary(privateTokens, program)
	if err := parseType1Private(privateTokens, program); err != nil {
		return nil, err
	}
	return program, nil
}

// cutType1Trailer drops the decrypted padding after the closing marker. The
// bytes behind the marker decrypt to noise, and the noise can contain tokens
// like RD that would derail the tokenizer.
func cutType1Trailer(data []byte) []byte {
	for _, marker := range []string{"currentfile closefile", "cleartomark"} {
		if at := indexAfter(data, marker, 0); at >= 0 {
			return data[:at]
		}
	}
	return data
}

// type1CipherBytes decodes an ASCII hex eexec section or returns the binary
// bytes unchanged.
func type1CipherBytes(encrypted []byte) ([]byte, error) {
	if decoded, ok := type1HexText(encrypted); ok {
		return decoded, nil
	}
	if len(encrypted) == 0 {
		return nil, fmt.Errorf("%w: empty eexec section", errType1Truncated)
	}
	return encrypted, nil
}

// type1HexText decodes an ASCII hex string when every non-space byte is a hex
// digit and the digit count is even. Binary ciphertext almost never passes.
func type1HexText(data []byte) ([]byte, bool) {
	digits, ok := t1HexDigits(data)
	if !ok {
		return nil, false
	}
	decoded := make([]byte, len(digits)/type1DigitsPerByte)
	if _, err := hex.Decode(decoded, digits); err != nil {
		return nil, false
	}
	return decoded, true
}

// t1HexDigits collects the hex digits of one string, or reports false when a
// non-hex byte appears or the count is odd.
func t1HexDigits(data []byte) ([]byte, bool) {
	digits := make([]byte, 0, len(data))
	for _, value := range data {
		if isType1Space(value) {
			continue
		}
		if !isType1HexDigit(value) {
			return nil, false
		}
		digits = append(digits, value)
	}
	if len(digits) == 0 || len(digits)%type1DigitsPerByte != 0 {
		return nil, false
	}
	return digits, true
}

// isType1HexDigit reports whether the byte is an ASCII hex digit.
func isType1HexDigit(value byte) bool {
	return (value >= '0' && value <= '9') || (value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

// type1Decrypt runs the Type 1 stream cipher with one seed. The state update
// uses the ciphertext byte, as the format defines.
func type1Decrypt(data []byte, seed uint16) []byte {
	state := seed
	out := make([]byte, len(data))
	for index, value := range data {
		out[index] = value ^ byte(state>>type1CipherShift)
		state = (state+uint16(value))*type1Cipher1 + type1Cipher2
	}
	return out
}

// type1DecryptCharstring decrypts one charstring and drops its lenIV random
// bytes. A charstring shorter than lenIV returns an error.
func type1DecryptCharstring(data []byte, lenIV int) ([]byte, error) {
	plain := type1Decrypt(data, type1SeedCharstring)
	if len(plain) < lenIV {
		return nil, fmt.Errorf("%w: charstring shorter than lenIV", errType1Truncated)
	}
	return plain[lenIV:], nil
}

// parseType1Dictionary recovers /FontName, /FontMatrix, and /Encoding. It runs
// on the cleartext first and the private section second, and a value is only
// filled when it is still empty.
func parseType1Dictionary(tokens []t1Token, program *Type1Font) {
	for index := 0; index < len(tokens); index++ {
		if tokens[index].kind != t1LiteralName {
			continue
		}
		if last, ok := t1DictionaryValue(tokens, index, program); ok {
			index = last
		}
	}
}

// t1DictionaryValue reads one literal-name entry. The returned index is the
// last token the entry used.
func t1DictionaryValue(tokens []t1Token, index int, program *Type1Font) (int, bool) {
	switch tokens[index].name {
	case "FontName":
		return t1DictionaryName(tokens, index, program)
	case "FontMatrix":
		return t1DictionaryMatrix(tokens, index, program)
	case "Encoding":
		return t1DictionaryEncoding(tokens, index, program)
	}
	return index, false
}

// t1DictionaryName reads a "/FontName /X" entry.
func t1DictionaryName(tokens []t1Token, index int, program *Type1Font) (int, bool) {
	if program.Name != "" || index+1 >= len(tokens) || tokens[index+1].kind != t1LiteralName {
		return index, false
	}
	program.Name = tokens[index+1].name
	return index + 1, true
}

// t1DictionaryMatrix reads a "/FontMatrix [ ... ]" entry.
func t1DictionaryMatrix(tokens []t1Token, index int, program *Type1Font) (int, bool) {
	if program.FontMatrix != ([6]float64{}) {
		return index, false
	}
	values, last, ok := t1NumberArray(tokens, index+1, type1FontMatrixLen)
	if !ok {
		return index, false
	}
	copy(program.FontMatrix[:], values)
	return last, true
}

// t1DictionaryEncoding reads an "/Encoding ..." entry.
func t1DictionaryEncoding(tokens []t1Token, index int, program *Type1Font) (int, bool) {
	encoding, last, ok := t1Encoding(tokens, index+1)
	if !ok {
		return index, false
	}
	program.Encoding = encoding
	if last < index {
		last = index
	}
	return last, true
}

// parseType1Private recovers lenIV, /Subrs, /CharStrings, and /WeightVector
// from the decrypted private dictionary.
func parseType1Private(tokens []t1Token, program *Type1Font) error {
	parseType1Values(tokens, program)
	if program.lenIV < 0 || program.lenIV > type1MaxLenIV {
		return fmt.Errorf("%w: lenIV %d", errType1Limit, program.lenIV)
	}
	subrs, err := parseType1Subrs(tokens, program)
	if err != nil {
		return err
	}
	program.subrs = subrs
	glyphs, err := parseType1CharStrings(tokens, program)
	if err != nil {
		return err
	}
	if len(glyphs) == 0 {
		return errType1NoCharStrings
	}
	program.charStrings = glyphs
	return nil
}

// parseType1Values reads the scalar private entries.
func parseType1Values(tokens []t1Token, program *Type1Font) {
	for index := 0; index < len(tokens); index++ {
		if tokens[index].kind != t1LiteralName {
			continue
		}
		switch tokens[index].name {
		case "lenIV":
			if index+1 < len(tokens) && tokens[index+1].kind == t1Number {
				program.lenIV = int(tokens[index+1].num)
				index++
			}
		case "WeightVector":
			if values, last, ok := t1NumberArray(tokens, index+1, 0); ok {
				program.weight = values
				index = last
			}
		}
	}
}

// parseType1Subrs reads the /Subrs array. Entries appear as
// "dup <index> <length> RD <bytes> NP" or the -| spelling.
func parseType1Subrs(tokens []t1Token, program *Type1Font) ([][]byte, error) {
	start := t1FindLiteral(tokens, "Subrs")
	if start < 0 {
		return nil, nil
	}
	count := t1ArraySize(tokens, start+1)
	if count > type1MaxSubrs {
		return nil, fmt.Errorf("%w: %d Subrs", errType1Limit, count)
	}
	subrs := make([][]byte, 0, count)
	index := start + 1
	for index < len(tokens) {
		if t1SubrsEnd(tokens[index]) {
			break
		}
		if tokens[index].kind != t1Name || tokens[index].name != "dup" {
			index++
			continue
		}
		entry, next, err := t1ReadSubrEntry(tokens, index, program.lenIV)
		if err != nil {
			return nil, err
		}
		index = next
		for len(subrs) <= entry.index {
			subrs = append(subrs, nil)
		}
		subrs[entry.index] = entry.code
	}
	return subrs, nil
}

// t1SubrsEnd reports whether the token closes the /Subrs array. Real fonts
// end the entries with NP and the array with ND, readonly def, or end.
func t1SubrsEnd(token t1Token) bool {
	if token.kind != t1Name {
		return false
	}
	switch token.name {
	case "def", "readonly", "end", "ND":
		return true
	default:
		return false
	}
}

// type1SubrEntry is one decrypted Subrs element.
type type1SubrEntry struct {
	index int
	code  []byte
}

// t1ReadSubrEntry reads one "dup <index> <length> RD <bytes>" run and returns
// the entry and the index after it.
func t1ReadSubrEntry(tokens []t1Token, index, lenIV int) (type1SubrEntry, int, error) {
	if index+3 >= len(tokens) || tokens[index+1].kind != t1Number ||
		tokens[index+2].kind != t1Number || tokens[index+3].kind != t1RD {
		return type1SubrEntry{}, index + 1, fmt.Errorf("%w: Subrs entry", errType1Syntax)
	}
	entry := int(tokens[index+1].num)
	length := int(tokens[index+2].num)
	code, err := type1DecryptCharstring(tokens[index+3].data, lenIV)
	if err != nil || len(tokens[index+3].data) != length {
		return type1SubrEntry{}, index + 1, fmt.Errorf("%w: Subrs entry %d", errType1Syntax, entry)
	}
	return type1SubrEntry{index: entry, code: code}, index + type1SubrEntryLen, nil
}

// parseType1CharStrings reads the /CharStrings dictionary. Entries appear as
// "/<name> <length> RD <bytes> ND".
func parseType1CharStrings(tokens []t1Token, program *Type1Font) (map[string][]byte, error) {
	start := t1FindLiteral(tokens, "CharStrings")
	if start < 0 {
		return map[string][]byte{}, nil
	}
	count := t1ArraySize(tokens, start+1)
	if count > type1MaxGlyphs {
		return nil, fmt.Errorf("%w: %d CharStrings", errType1Limit, count)
	}
	glyphs := make(map[string][]byte, count)
	index := start + 1
	for index < len(tokens) {
		if tokens[index].kind == t1Name && tokens[index].name == "end" {
			break
		}
		name, code, next, ok := t1ReadCharEntry(tokens, index, program.lenIV)
		if !ok {
			index++
			continue
		}
		glyphs[name] = code
		index = next
	}
	return glyphs, nil
}

// t1ReadCharEntry reads one "/<name> <length> RD <bytes>" run.
func t1ReadCharEntry(tokens []t1Token, index, lenIV int) (string, []byte, int, bool) {
	if tokens[index].kind != t1LiteralName || index+2 >= len(tokens) ||
		tokens[index+1].kind != t1Number || tokens[index+2].kind != t1RD {
		return "", nil, index, false
	}
	code, err := type1DecryptCharstring(tokens[index+2].data, lenIV)
	if err != nil {
		return "", nil, index, false
	}
	return tokens[index].name, code, index + type1CharEntryLen, true
}

// t1FindLiteral returns the index of one literal name, or -1.
func t1FindLiteral(tokens []t1Token, name string) int {
	for index, token := range tokens {
		if token.kind == t1LiteralName && token.name == name {
			return index
		}
	}
	return -1
}

// t1ArraySize reads the count of "/Subrs 5 array" or "/CharStrings 855 dict".
// A missing or malformed count returns 0, which only relaxes a cap.
func t1ArraySize(tokens []t1Token, index int) int {
	for pos := index; pos < len(tokens) && pos < index+type1SubrEntryLen; pos++ {
		if tokens[pos].kind != t1Number {
			continue
		}
		if tokens[pos].num < 0 || tokens[pos].num > 0xFFFF {
			return 0
		}
		return int(tokens[pos].num)
	}
	return 0
}

// t1NumberArray reads "[" followed by at least minimum numbers followed by
// "]". A minimum of 0 accepts any length. It returns the values and the index
// of "]".
func t1NumberArray(tokens []t1Token, index, minimum int) ([]float64, int, bool) {
	if index >= len(tokens) || tokens[index].kind != t1ArrayOpen {
		return nil, 0, false
	}
	values := make([]float64, 0, minimum)
	pos := index + 1
	for pos < len(tokens) && tokens[pos].kind != t1ArrayClose {
		if tokens[pos].kind == t1Number {
			values = append(values, tokens[pos].num)
		}
		pos++
	}
	if pos >= len(tokens) || len(values) < minimum {
		return nil, 0, false
	}
	return values, pos, true
}

// t1Encoding reads a built-in /Encoding. It accepts the common
// "256 array ... dup <code> /<name> put ... def" form, a literal array, and
// one of the named tables. It returns false when the definition is not a form
// the reader understands, which leaves the built-in table empty.
func t1Encoding(tokens []t1Token, index int) ([256]string, int, bool) {
	if index >= len(tokens) {
		return [256]string{}, 0, false
	}
	if tokens[index].kind == t1LiteralName || tokens[index].kind == t1Name {
		return t1EncodingName(tokens[index].name)
	}
	if tokens[index].kind == t1ArrayOpen {
		return t1EncodingArray(tokens, index)
	}
	if !t1EncodingHeader(tokens, index) {
		return [256]string{}, 0, false
	}
	var encoding [256]string
	pos := index + type1ArrayHeader
	for pos < len(tokens) {
		if tokens[pos].kind == t1Name && tokens[pos].name == "def" {
			return encoding, pos, true
		}
		pos = t1EncodingStep(tokens, pos, &encoding)
	}
	return encoding, 0, true
}

// t1EncodingHeader reports whether the tokens start a "256 array" header.
func t1EncodingHeader(tokens []t1Token, index int) bool {
	if tokens[index].kind != t1Number || index+1 >= len(tokens) {
		return false
	}
	return tokens[index+1].kind == t1Name && tokens[index+1].name == "array"
}

// t1EncodingStep consumes one token of the array form and records a put when
// it sees one.
func t1EncodingStep(tokens []t1Token, index int, encoding *[256]string) int {
	token := tokens[index]
	if token.kind == t1BraceOpen {
		return t1SkipBraces(tokens, index)
	}
	if d, ok := t1ReadEncodingDup(tokens, index); ok {
		t1SetEncoding(encoding, d.code, d.name)
		return index + d.size
	}
	if token.kind == t1Name && token.name == "put" {
		t1EncodingPut(tokens, index, encoding)
	}
	return index + 1
}

// t1EncodingDup is one "dup <code> /<name> put" run.
type t1EncodingDup struct {
	code int
	name string
	size int
}

// t1ReadEncodingDup recognizes a dup-put run at index.
func t1ReadEncodingDup(tokens []t1Token, index int) (t1EncodingDup, bool) {
	if index+3 >= len(tokens) || tokens[index].kind != t1Name || tokens[index].name != "dup" {
		return t1EncodingDup{code: 0, name: "", size: 0}, false
	}
	if tokens[index+1].kind != t1Number || tokens[index+2].kind != t1LiteralName {
		return t1EncodingDup{code: 0, name: "", size: 0}, false
	}
	if tokens[index+3].kind != t1Name || tokens[index+3].name != "put" {
		return t1EncodingDup{code: 0, name: "", size: 0}, false
	}
	return t1EncodingDup{
		code: int(tokens[index+1].num),
		name: tokens[index+2].name,
		size: type1SubrEntryLen,
	}, true
}

// t1EncodingPut records the "<code> /<name> put" form when it appears.
func t1EncodingPut(tokens []t1Token, index int, encoding *[256]string) {
	if index < type1ArrayHeader {
		return
	}
	if tokens[index-1].kind != t1LiteralName || tokens[index-2].kind != t1Number {
		return
	}
	t1SetEncoding(encoding, int(tokens[index-2].num), tokens[index-1].name)
}

// t1SkipBraces skips a balanced procedure body and returns the index after it.
func t1SkipBraces(tokens []t1Token, index int) int {
	depth := 0
	for pos := index; pos < len(tokens); pos++ {
		switch {
		case tokens[pos].kind == t1BraceOpen:
			depth++
		case tokens[pos].kind == t1BraceClose:
			depth--
			if depth == 0 {
				return pos + 1
			}
		}
	}
	return len(tokens)
}

// t1EncodingArray reads a literal "[ /A /B ... ]" encoding.
func t1EncodingArray(tokens []t1Token, index int) ([256]string, int, bool) {
	var encoding [256]string
	code := 0
	pos := index + 1
	for pos < len(tokens) && tokens[pos].kind != t1ArrayClose {
		if tokens[pos].kind == t1LiteralName {
			t1SetEncoding(&encoding, code, tokens[pos].name)
			code++
		}
		pos++
	}
	return encoding, pos, true
}

// t1EncodingName maps a named table to the tables this package carries.
func t1EncodingName(name string) ([256]string, int, bool) {
	if name == "StandardEncoding" {
		return EncodingStandard.GlyphNames(), 0, true
	}
	return [256]string{}, 0, false
}

// t1SetEncoding records one code-to-name pair. The ".notdef" name leaves the
// code undefined, matching how the rest of this package spells an empty slot.
func t1SetEncoding(encoding *[256]string, code int, name string) {
	if code < 0 || code >= len(encoding) || name == ".notdef" {
		return
	}
	encoding[code] = name
}

// isType1Space reports whether the byte is a PostScript whitespace character.
func isType1Space(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r' ||
		value == '\f' || value == 0
}

// type1Delimiters are the PostScript delimiter characters.
const type1Delimiters = "()<>[]{}/%"

// isType1Regular reports whether the byte can appear in a PostScript name.
func isType1Regular(value byte) bool {
	return !isType1Space(value) && strings.IndexByte(type1Delimiters, value) < 0
}

// t1ParseNumber decodes a number token.
func t1ParseNumber(text string) (float64, bool) {
	if value, err := strconv.Atoi(text); err == nil {
		return float64(value), true
	}
	if value, err := strconv.ParseFloat(text, 64); err == nil {
		return value, true
	}
	return 0, false
}
