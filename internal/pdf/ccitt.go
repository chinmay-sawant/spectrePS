package pdf

import (
	"bytes"
	"image"
	"sort"

	"golang.org/x/image/ccitt"
)

const (
	nameCCITT = "CCITTFaxDecode"

	keyCCITTParms      = "DecodeParms"
	keyCCITTK          = "K"
	keyCCITTColumns    = "Columns"
	keyCCITTRows       = "Rows"
	keyCCITTEndOfLine  = "EndOfLine"
	keyCCITTEndOfBlock = "EndOfBlock"
	keyCCITTBlackIs1   = "BlackIs1"
	keyCCITTByteAlign  = "EncodedByteAlign"

	bitsBilevel = 1

	// The Group 3 row decoder reads one 13-bit lookahead: the longest run
	// code is 13 bits and the longest 2-D mode code is 7. A makeup code is 64
	// or more, and an end-of-line marker is eleven zero bits then one. Byte
	// alignment may pad an end-of-line marker with up to seven more zeros.
	ccittLookahead   = 13
	ccittLookupSize  = 1 << ccittLookahead
	ccittMakeupMin   = 64
	ccittEOLZeros    = 11
	ccittAlignPadMax = 7
)

// ccittImage is the decoded /DecodeParms of one CCITT stream plus the size.
type ccittImage struct {
	k         int
	endOfLine bool
	blackIs1  bool
	byteAlign bool
	columns   int
	rows      int
}

// decodeCCITTImage decodes a /CCITTFaxDecode image stream into Gray.
// /K < 0 selects Group 4 and /K == 0 with /EndOfLine true selects Group 3,
// both through golang.org/x/image/ccitt. This package decodes the two shapes
// that library does not: a Group 3 stream with no end-of-line markers between
// rows, and a /K > 0 mixed one- and two-dimensional Group 3 stream whose rows
// begin with an end-of-line marker and a tag bit.
//
// Every shape decodes exactly /Rows rows of /Columns columns and then stops,
// so an /EndOfBlock false stream that carries no closing pattern decodes. A
// truncated row is syntaxerror and a stream the decoder rejects is
// syntaxerror too.
func decodeCCITTImage(stream Value) (image.Image, error) {
	params, err := ccittImageParams(stream)
	if err != nil {
		return nil, err
	}
	if int64(params.columns)*int64(params.rows) > maxInflated {
		return nil, NewError(opImage, errLimit)
	}
	pic := image.NewGray(image.Rect(0, 0, params.columns, params.rows))
	if params.k < 0 || (params.k == 0 && params.endOfLine) {
		options := ccitt.Options{Align: params.byteAlign, Invert: params.blackIs1}
		subFormat := ccitt.Group4
		if params.k == 0 {
			subFormat = ccitt.Group3
		}
		if err := ccitt.DecodeIntoGray(pic, bytes.NewReader(stream.Stream), ccitt.MSB, subFormat, &options); err != nil {
			return nil, NewError(opImage, errSyntax)
		}
		return pic, nil
	}
	if err := decodeCCITTG3(pic, stream.Stream, params); err != nil {
		return nil, err
	}
	return pic, nil
}

// ccittImageParams reads the image dictionary and /DecodeParms. CCITT is
// bilevel, so the bit depth is 1 and the color space is DeviceGray.
func ccittImageParams(stream Value) (ccittImage, error) {
	width, okWidth := stream.IntEntry(keyWidth)
	height, okHeight := stream.IntEntry(keyHeight)
	bits, okBits := stream.IntEntry(keyBits)
	space, okSpace := stream.NameEntry(keyColorSpace)
	if !okWidth || !okHeight || width <= 0 || height <= 0 ||
		!okBits || bits != bitsBilevel || !okSpace || space != colorGray {
		return ccittImage{}, NewError(opImage, errUndefined)
	}
	return ccittDecodeParms(stream, width, height)
}

// ccittDecodeParms reads /DecodeParms into the Group 3 and Group 4 controls
// and the /Columns and /Rows overrides. A missing /DecodeParms selects the
// defaults: /K 0, /EndOfLine false, /EndOfBlock true, /BlackIs1 false,
// /EncodedByteAlign false, and the /Width and /Height sizes. Any entry of the
// wrong kind returns undefined.
func ccittDecodeParms(stream Value, width, height int) (ccittImage, error) {
	parms, ok := stream.ValueEntry(keyCCITTParms)
	if !ok {
		parms = NullVal()
	}
	if parms.Kind != KindNull && parms.Kind != KindDict {
		return ccittImage{}, NewError(opImage, errUndefined)
	}
	kValue, found, err := ccittInt(parms, keyCCITTK)
	if err != nil {
		return ccittImage{}, err
	}
	if !found {
		kValue = 0
	}
	endOfLine, err := ccittFlag(parms, keyCCITTEndOfLine)
	if err != nil {
		return ccittImage{}, err
	}
	// /EndOfBlock is read and validated, then dropped. Both values stop the
	// decoded rows after /Rows rows: a false stream carries no closing
	// pattern, and a true stream's pattern does not change the pixels.
	if _, err := ccittFlagDefault(parms, keyCCITTEndOfBlock, true); err != nil {
		return ccittImage{}, err
	}
	blackIs1, err := ccittFlag(parms, keyCCITTBlackIs1)
	if err != nil {
		return ccittImage{}, err
	}
	byteAlign, err := ccittFlag(parms, keyCCITTByteAlign)
	if err != nil {
		return ccittImage{}, err
	}
	columns, err := ccittDimension(parms, keyCCITTColumns, width)
	if err != nil {
		return ccittImage{}, err
	}
	rows, err := ccittDimension(parms, keyCCITTRows, height)
	if err != nil {
		return ccittImage{}, err
	}
	return ccittImage{
		k:         kValue,
		endOfLine: endOfLine,
		blackIs1:  blackIs1,
		byteAlign: byteAlign,
		columns:   columns,
		rows:      rows,
	}, nil
}

// ccittDimension returns one /Columns or /Rows override, or the fallback
// size. A present value that is not a positive whole number returns
// undefined.
func ccittDimension(parms Value, key string, fallback int) (int, error) {
	value, found, err := ccittInt(parms, key)
	if err != nil {
		return 0, err
	}
	if !found {
		return fallback, nil
	}
	if value <= 0 {
		return 0, NewError(opImage, errUndefined)
	}
	return value, nil
}

// ccittInt returns one whole-number /DecodeParms entry. A missing or null
// entry reports false. Any other kind returns undefined.
func ccittInt(parms Value, key string) (int, bool, error) {
	entry, found := parms.ValueEntry(key)
	if !found || entry.Kind == KindNull {
		return 0, false, nil
	}
	value, ok := parms.IntEntry(key)
	if !ok {
		return 0, false, NewError(opImage, errUndefined)
	}
	return value, true, nil
}

// ccittFlag returns one boolean /DecodeParms entry. A missing or null entry
// is false. Any other kind returns undefined.
func ccittFlag(parms Value, key string) (bool, error) {
	return ccittFlagDefault(parms, key, false)
}

// ccittFlagDefault returns one boolean /DecodeParms entry, or the fallback
// when the entry is missing or null. Any other kind returns undefined.
func ccittFlagDefault(parms Value, key string, fallback bool) (bool, error) {
	entry, found := parms.ValueEntry(key)
	if !found || entry.Kind == KindNull {
		return fallback, nil
	}
	if entry.Kind != KindBool {
		return false, NewError(opImage, errUndefined)
	}
	return entry.Bool, nil
}

// ccittMode is one T.4 two-dimensional mode code.
type ccittMode uint8

const (
	ccittModePass ccittMode = iota
	ccittModeHorizontal
	ccittModeV0
	ccittModeVR1
	ccittModeVR2
	ccittModeVR3
	ccittModeVL1
	ccittModeVL2
	ccittModeVL3
	ccittModeExtension
)

// ccittDecodeEntry is one row of the 13-bit run-code lookup: the code length,
// and the run length. A zero length marks an invalid prefix.
type ccittDecodeEntry struct {
	length uint8
	run    int16
}

// ccittModeEntry is one row of the 13-bit mode-code lookup.
type ccittModeEntry struct {
	length uint8
	mode   ccittMode
}

// ccittDecodeTables holds the run and mode lookup tables, built once from the
// T.4 code lists.
type ccittDecodeTables struct {
	white [ccittLookupSize]ccittDecodeEntry
	black [ccittLookupSize]ccittDecodeEntry
	mode  [ccittLookupSize]ccittModeEntry
}

var ccittDecode = buildCCITTDecodeTables()

// buildCCITTDecodeTables expands every code into all lookahead suffixes, so a
// decode is one array index and a length check.
func buildCCITTDecodeTables() ccittDecodeTables {
	var tables ccittDecodeTables
	for _, code := range ccittWhiteRunCodes {
		fillCCITTRunLookup(&tables.white, code)
	}
	for _, code := range ccittBlackRunCodes {
		fillCCITTRunLookup(&tables.black, code)
	}
	for _, code := range ccittModeCodes {
		fillCCITTModeLookup(&tables.mode, code)
	}
	return tables
}

func fillCCITTRunLookup(lookup *[ccittLookupSize]ccittDecodeEntry, code ccittRunCode) {
	entry := ccittDecodeEntry{length: code.length, run: code.run}
	fillCCITTLookup(lookup[:], uint32(code.bits), int(code.length), entry)
}

func fillCCITTModeLookup(lookup *[ccittLookupSize]ccittModeEntry, code ccittModeCode) {
	entry := ccittModeEntry{length: code.length, mode: code.mode}
	fillCCITTLookup(lookup[:], uint32(code.bits), int(code.length), entry)
}

// fillCCITTLookup writes one code entry into every lookahead slot that starts
// with the code bits.
func fillCCITTLookup[T any](lookup []T, bits uint32, length int, entry T) {
	base := int(bits) << (ccittLookahead - length)
	span := 1 << (ccittLookahead - length)
	for index := range span {
		lookup[base|index] = entry
	}
}

// ccittBits reads the most-significant-bit-first bit stream of one CCITT
// stream. The position past the end is tracked rather than clamped, so a
// truncated row reports failure instead of decoding padding.
type ccittBits struct {
	data []byte
	pos  int
}

// readBit returns the next bit and whether one was available.
func (bits *ccittBits) readBit() (int, bool) {
	if bits.pos >= len(bits.data)*byteBits {
		return 0, false
	}
	bit := int(bits.data[bits.pos/byteBits]>>(byteLowBits-bits.pos%byteBits)) & 1
	bits.pos++
	return bit, true
}

// peek returns the next ccittLookahead bits left-aligned, zero-padded past
// the end, and the number of real bits available.
func (bits *ccittBits) peek() (int, int) {
	value := 0
	avail := 0
	for range ccittLookahead {
		bit, ok := bits.readBit()
		if !ok {
			break
		}
		value = value<<1 | bit
		avail++
	}
	bits.pos -= avail
	return value << (ccittLookahead - avail), avail
}

// align skips to the next byte boundary.
func (bits *ccittBits) align() {
	if rest := bits.pos % byteBits; rest != 0 {
		bits.pos += byteBits - rest
	}
}

// findEOL consumes one end-of-line marker: eleven zero bits then one, with up
// to ccittAlignPadMax padding zeros in front.
func (bits *ccittBits) findEOL() bool {
	zeros := 0
	for {
		bit, ok := bits.readBit()
		if !ok {
			return false
		}
		if bit == 1 {
			return zeros >= ccittEOLZeros
		}
		zeros++
		if zeros > ccittEOLZeros+ccittAlignPadMax {
			return false
		}
	}
}

// decodeRun reads one run code from the white or black table.
func (bits *ccittBits) decodeRun(black bool) (int, bool) {
	table := &ccittDecode.white
	if black {
		table = &ccittDecode.black
	}
	value, avail := bits.peek()
	entry := table[value]
	if entry.length == 0 || int(entry.length) > avail {
		return 0, false
	}
	bits.pos += int(entry.length)
	return int(entry.run), true
}

// decodeMode reads one 2-D mode code.
func (bits *ccittBits) decodeMode() (ccittMode, bool) {
	value, avail := bits.peek()
	entry := ccittDecode.mode[value]
	if entry.length == 0 || int(entry.length) > avail {
		return ccittModePass, false
	}
	bits.pos += int(entry.length)
	return entry.mode, true
}

// runTotal reads one run of the named color, chaining makeup codes until a
// terminating code arrives.
func (bits *ccittBits) runTotal(black bool) (int, bool) {
	total := 0
	for {
		run, ok := bits.decodeRun(black)
		if !ok {
			return 0, false
		}
		total += run
		if run < ccittMakeupMin {
			return total, true
		}
	}
}

// decodeCCITTG3 decodes the Group 3 shapes golang.org/x/image/ccitt leaves
// out: /EndOfLine false streams with no markers between rows, and /K > 0
// streams whose rows begin with an end-of-line marker and a one-bit tag.
func decodeCCITTG3(pic *image.Gray, data []byte, params ccittImage) error {
	width := params.columns
	bits := ccittBits{data: data}
	ref := make([]bool, width)
	cur := make([]bool, width)
	for row := range params.rows {
		var err error
		if params.k > 0 {
			if params.byteAlign {
				bits.align()
			}
			if !bits.findEOL() {
				return NewError(opImage, errSyntax)
			}
			tag, ok := bits.readBit()
			if !ok {
				return NewError(opImage, errSyntax)
			}
			if tag == 0 {
				err = ccittTwoDimensionalRow(&bits, cur, ref)
			} else {
				err = ccittOneDimensionalRow(&bits, cur)
			}
		} else {
			err = ccittOneDimensionalRow(&bits, cur)
		}
		if err != nil {
			return err
		}
		ccittStoreRow(pic, row, cur, params.blackIs1)
		ref, cur = cur, ref
	}
	return nil
}

// ccittOneDimensionalRow decodes one run-length-coded row. Runs alternate
// from white, and the row ends when exactly the row width is filled.
func ccittOneDimensionalRow(bits *ccittBits, row []bool) error {
	position := 0
	black := false
	for position < len(row) {
		run, ok := bits.runTotal(black)
		if !ok || position+run > len(row) {
			return NewError(opImage, errSyntax)
		}
		ccittFillRow(row, position, position+run, black)
		position += run
		black = !black
	}
	return nil
}

// ccittTwoDimensionalRow decodes one row relative to the reference row above
// it: pass, horizontal, and vertical modes from the T.4 2-D set. A vertical
// mode moves the next changing element to b1 plus its delta. An extension
// code is unsupported and reports syntaxerror.
func ccittTwoDimensionalRow(bits *ccittBits, row, ref []bool) error {
	changes := ccittChanges(ref)
	width := len(row)
	position := 0
	search := -1
	black := false
	for position < width {
		mode, ok := bits.decodeMode()
		if !ok {
			return NewError(opImage, errSyntax)
		}
		switch mode {
		case ccittModePass:
			_, second := ccittB1B2(changes, ref, search, black)
			ccittFillRow(row, position, second, black)
			position = second
		case ccittModeHorizontal:
			first, second := false, true
			if black {
				first, second = second, first
			}
			run1, ok := bits.runTotal(first)
			if !ok {
				return NewError(opImage, errSyntax)
			}
			run2, ok := bits.runTotal(second)
			if !ok || position+run1+run2 > width {
				return NewError(opImage, errSyntax)
			}
			ccittFillRow(row, position, position+run1, black)
			ccittFillRow(row, position+run1, position+run1+run2, !black)
			position += run1 + run2
		case ccittModeExtension:
			return NewError(opImage, errSyntax)
		case ccittModeV0, ccittModeVR1, ccittModeVR2, ccittModeVR3,
			ccittModeVL1, ccittModeVL2, ccittModeVL3:
			first, _ := ccittB1B2(changes, ref, search, black)
			next := first + ccittVerticalDelta(mode)
			if next < position || next > width {
				return NewError(opImage, errSyntax)
			}
			ccittFillRow(row, position, next, black)
			position = next
			black = !black
		default:
			return NewError(opImage, errSyntax)
		}
		search = position
	}
	return nil
}

// ccittChanges returns the changing positions of one reference row: the
// indexes whose color differs from the pixel before them, with the row width
// appended as a sentinel for a missing change.
func ccittChanges(row []bool) []int {
	changes := make([]int, 0, len(row)/2+1)
	previous := false
	for index, black := range row {
		if black != previous {
			changes = append(changes, index)
		}
		previous = black
	}
	return append(changes, len(row))
}

// ccittB1B2 returns b1 and b2 for one 2-D mode: b1 is the first changing
// element on the reference row to the right of search whose color is opposite
// to the current run, and b2 is the changing element after b1. The width
// sentinel stands in for a change that does not exist.
func ccittB1B2(changes []int, ref []bool, search int, black bool) (int, int) {
	width := len(ref)
	index := sort.SearchInts(changes, search+1)
	for index < len(changes)-1 && ref[changes[index]] == black {
		index++
	}
	b1 := changes[index]
	if b1 >= width || index+1 >= len(changes) {
		return b1, width
	}
	return b1, changes[index+1]
}

// ccittVerticalDelta returns the pixel delta one vertical mode codes.
func ccittVerticalDelta(mode ccittMode) int {
	switch mode {
	case ccittModeV0:
		return 0
	case ccittModeVR1:
		return 1
	case ccittModeVR2:
		return 2
	case ccittModeVR3:
		return 3
	case ccittModeVL1:
		return -1
	case ccittModeVL2:
		return -2
	case ccittModeVL3:
		return -3
	}
	return 0
}

// ccittFillRow writes one run of one color.
func ccittFillRow(row []bool, from, to int, black bool) {
	for index := from; index < to; index++ {
		row[index] = black
	}
}

// ccittStoreRow writes one decoded row into the Gray image. A white pixel is
// 255 and a black pixel is 0, and /BlackIs1 inverts both, matching the
// golang.org/x/image/ccitt option the Group 4 and Group 3 paths pass.
func ccittStoreRow(pic *image.Gray, row int, pixels []bool, blackIs1 bool) {
	at := row * pic.Stride
	for column, black := range pixels {
		value := byte(0xFF)
		if black {
			value = 0
		}
		if blackIs1 {
			value = ^value
		}
		pic.Pix[at+column] = value
	}
}

// ccittRunCode is one T.4 run-length code: the bits right-aligned, the bit
// count, and the run length it codes.
type ccittRunCode struct {
	bits   uint16
	length uint8
	run    int16
}

// ccittModeCode is one T.4 2-D mode code.
type ccittModeCode struct {
	bits   uint16
	length uint8
	mode   ccittMode
}

// Code generated from the ITU-T T.4 and T.6 run-length and 2-D mode code
// tables. Do not edit by hand.
//
// Each entry is one Huffman code: the bits right-aligned, the bit count, and
// the white or black run length the code terminates, or the 2-D mode it names.
// The values were checked against golang.org/x/image/ccitt (BSD-3-Clause),
// which generates the same tables from the T.6 specification.

var ccittWhiteRunCodes = []ccittRunCode{
	{0x7, 4, 2},
	{0x8, 4, 3},
	{0xb, 4, 4},
	{0xc, 4, 5},
	{0xe, 4, 6},
	{0xf, 4, 7},
	{0x7, 5, 10},
	{0x8, 5, 11},
	{0x12, 5, 128},
	{0x13, 5, 8},
	{0x14, 5, 9},
	{0x1b, 5, 64},
	{0x3, 6, 13},
	{0x7, 6, 1},
	{0x8, 6, 12},
	{0x17, 6, 192},
	{0x18, 6, 1664},
	{0x2a, 6, 16},
	{0x2b, 6, 17},
	{0x34, 6, 14},
	{0x35, 6, 15},
	{0x3, 7, 22},
	{0x4, 7, 23},
	{0x8, 7, 20},
	{0xc, 7, 19},
	{0x13, 7, 26},
	{0x17, 7, 21},
	{0x18, 7, 28},
	{0x24, 7, 27},
	{0x27, 7, 18},
	{0x28, 7, 24},
	{0x2b, 7, 25},
	{0x37, 7, 256},
	{0x2, 8, 29},
	{0x3, 8, 30},
	{0x4, 8, 45},
	{0x5, 8, 46},
	{0xa, 8, 47},
	{0xb, 8, 48},
	{0x12, 8, 33},
	{0x13, 8, 34},
	{0x14, 8, 35},
	{0x15, 8, 36},
	{0x16, 8, 37},
	{0x17, 8, 38},
	{0x1a, 8, 31},
	{0x1b, 8, 32},
	{0x24, 8, 53},
	{0x25, 8, 54},
	{0x28, 8, 39},
	{0x29, 8, 40},
	{0x2a, 8, 41},
	{0x2b, 8, 42},
	{0x2c, 8, 43},
	{0x2d, 8, 44},
	{0x32, 8, 61},
	{0x33, 8, 62},
	{0x34, 8, 63},
	{0x35, 8, 0},
	{0x36, 8, 320},
	{0x37, 8, 384},
	{0x4a, 8, 59},
	{0x4b, 8, 60},
	{0x52, 8, 49},
	{0x53, 8, 50},
	{0x54, 8, 51},
	{0x55, 8, 52},
	{0x58, 8, 55},
	{0x59, 8, 56},
	{0x5a, 8, 57},
	{0x5b, 8, 58},
	{0x64, 8, 448},
	{0x65, 8, 512},
	{0x67, 8, 640},
	{0x68, 8, 576},
	{0x98, 9, 1472},
	{0x99, 9, 1536},
	{0x9a, 9, 1600},
	{0x9b, 9, 1728},
	{0xcc, 9, 704},
	{0xcd, 9, 768},
	{0xd2, 9, 832},
	{0xd3, 9, 896},
	{0xd4, 9, 960},
	{0xd5, 9, 1024},
	{0xd6, 9, 1088},
	{0xd7, 9, 1152},
	{0xd8, 9, 1216},
	{0xd9, 9, 1280},
	{0xda, 9, 1344},
	{0xdb, 9, 1408},
	{0x8, 11, 1792},
	{0xc, 11, 1856},
	{0xd, 11, 1920},
	{0x12, 12, 1984},
	{0x13, 12, 2048},
	{0x14, 12, 2112},
	{0x15, 12, 2176},
	{0x16, 12, 2240},
	{0x17, 12, 2304},
	{0x1c, 12, 2368},
	{0x1d, 12, 2432},
	{0x1e, 12, 2496},
	{0x1f, 12, 2560},
}

var ccittBlackRunCodes = []ccittRunCode{
	{0x2, 2, 3},
	{0x3, 2, 2},
	{0x2, 3, 1},
	{0x3, 3, 4},
	{0x2, 4, 6},
	{0x3, 4, 5},
	{0x3, 5, 7},
	{0x4, 6, 9},
	{0x5, 6, 8},
	{0x4, 7, 10},
	{0x5, 7, 11},
	{0x7, 7, 12},
	{0x4, 8, 13},
	{0x7, 8, 14},
	{0x18, 9, 15},
	{0x8, 10, 18},
	{0xf, 10, 64},
	{0x17, 10, 16},
	{0x18, 10, 17},
	{0x37, 10, 0},
	{0x8, 11, 1792},
	{0xc, 11, 1856},
	{0xd, 11, 1920},
	{0x17, 11, 24},
	{0x18, 11, 25},
	{0x28, 11, 23},
	{0x37, 11, 22},
	{0x67, 11, 19},
	{0x68, 11, 20},
	{0x6c, 11, 21},
	{0x12, 12, 1984},
	{0x13, 12, 2048},
	{0x14, 12, 2112},
	{0x15, 12, 2176},
	{0x16, 12, 2240},
	{0x17, 12, 2304},
	{0x1c, 12, 2368},
	{0x1d, 12, 2432},
	{0x1e, 12, 2496},
	{0x1f, 12, 2560},
	{0x24, 12, 52},
	{0x27, 12, 55},
	{0x28, 12, 56},
	{0x2b, 12, 59},
	{0x2c, 12, 60},
	{0x33, 12, 320},
	{0x34, 12, 384},
	{0x35, 12, 448},
	{0x37, 12, 53},
	{0x38, 12, 54},
	{0x52, 12, 50},
	{0x53, 12, 51},
	{0x54, 12, 44},
	{0x55, 12, 45},
	{0x56, 12, 46},
	{0x57, 12, 47},
	{0x58, 12, 57},
	{0x59, 12, 58},
	{0x5a, 12, 61},
	{0x5b, 12, 256},
	{0x64, 12, 48},
	{0x65, 12, 49},
	{0x66, 12, 62},
	{0x67, 12, 63},
	{0x68, 12, 30},
	{0x69, 12, 31},
	{0x6a, 12, 32},
	{0x6b, 12, 33},
	{0x6c, 12, 40},
	{0x6d, 12, 41},
	{0xc8, 12, 128},
	{0xc9, 12, 192},
	{0xca, 12, 26},
	{0xcb, 12, 27},
	{0xcc, 12, 28},
	{0xcd, 12, 29},
	{0xd2, 12, 34},
	{0xd3, 12, 35},
	{0xd4, 12, 36},
	{0xd5, 12, 37},
	{0xd6, 12, 38},
	{0xd7, 12, 39},
	{0xda, 12, 42},
	{0xdb, 12, 43},
	{0x4a, 13, 640},
	{0x4b, 13, 704},
	{0x4c, 13, 768},
	{0x4d, 13, 832},
	{0x52, 13, 1280},
	{0x53, 13, 1344},
	{0x54, 13, 1408},
	{0x55, 13, 1472},
	{0x5a, 13, 1536},
	{0x5b, 13, 1600},
	{0x64, 13, 1664},
	{0x65, 13, 1728},
	{0x6c, 13, 512},
	{0x6d, 13, 576},
	{0x72, 13, 896},
	{0x73, 13, 960},
	{0x74, 13, 1024},
	{0x75, 13, 1088},
	{0x76, 13, 1152},
	{0x77, 13, 1216},
}

var ccittModeCodes = []ccittModeCode{
	{0x1, 1, ccittModeV0},
	{0x1, 3, ccittModeHorizontal},
	{0x2, 3, ccittModeVL1},
	{0x3, 3, ccittModeVR1},
	{0x1, 4, ccittModePass},
	{0x2, 6, ccittModeVL2},
	{0x3, 6, ccittModeVR2},
	{0x1, 7, ccittModeExtension},
	{0x2, 7, ccittModeVL3},
	{0x3, 7, ccittModeVR3},
}
