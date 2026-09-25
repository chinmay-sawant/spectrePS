package pdf

import "image"

const (
	lzwClear    = 256
	lzwEOD      = 257
	lzwFirst    = 258
	lzwMaxCode  = 4095
	lzwMaxWidth = 12
	lzwStart    = 9
)

// lzwEntry is one decoded table entry. A literal has prefix -1.
type lzwEntry struct {
	prefix int
	suffix byte
	length int
}

// decodeLZW reverses LZWDecode with the /DecodeParms predictor and
// /EarlyChange. Malformed data is syntaxerror and a stream that expands past
// the 32 MiB cap is limitcheck.
func decodeLZW(raw []byte, params Value) ([]byte, error) {
	parsed, err := filterParamsFrom(params, true)
	if err != nil {
		return nil, err
	}
	expanded, err := lzwExpand(raw, parsed.early)
	if err != nil {
		return nil, err
	}
	return applyPredictor(expanded, parsed)
}

// lzwExpand walks the 9 to 12 bit MSB-first code stream. Codes may straddle
// byte boundaries. early is the /EarlyChange value: 1 widens one code early,
// the vendor default, and 0 widens as late as possible.
func lzwExpand(raw []byte, early int) ([]byte, error) {
	table := lzwNewTable()
	next := lzwFirst
	width := lzwStart
	prev := -1
	reader := lzwBits{data: raw, pos: 0}
	out := make([]byte, 0, readChunk)
	var scratch []byte
	for {
		code, ok := reader.read(width)
		if !ok {
			return nil, NewError(opLZW, errSyntax)
		}
		if code == lzwClear {
			next = lzwFirst
			width = lzwStart
			prev = -1
			continue
		}
		if code == lzwEOD {
			return out, nil
		}
		expanded, newScratch, first, err := lzwOutput(code, table, next, prev, scratch, out)
		if err != nil {
			return nil, err
		}
		out = expanded
		scratch = newScratch
		if prev >= 0 && next <= lzwMaxCode {
			table[next] = lzwEntry{prefix: prev, suffix: first, length: table[prev].length + 1}
			next++
			if lzwWiden(next, width, early) {
				width++
			}
		}
		prev = code
	}
}

// lzwNewTable returns the literal table every clear code restores.
func lzwNewTable() []lzwEntry {
	table := make([]lzwEntry, lzwMaxCode+1)
	for i := range 256 {
		table[i] = lzwEntry{prefix: -1, suffix: byte(i), length: 1}
	}
	return table
}

// lzwOutput expands one code and appends it to out. It returns the new
// scratch and output slices plus the first byte of the expansion.
// code past the next table entry is syntaxerror.
func lzwOutput(
	code int,
	table []lzwEntry,
	next, prev int,
	scratch, out []byte,
) ([]byte, []byte, byte, error) {
	if code > next || (code == next && prev < 0) {
		return out, scratch, 0, NewError(opLZW, errSyntax)
	}
	var sequence []byte
	if code < next {
		sequence, scratch = lzwSequence(table, code, scratch)
	} else {
		// code == next is the KwKwK case: previous expansion plus its own first byte.
		sequence, scratch = lzwSequence(table, prev, scratch)
		sequence = append(sequence, sequence[0])
	}
	if len(sequence) > maxInflated-len(out) {
		return out, scratch, 0, NewError(opLZW, errLimit)
	}
	return append(out, sequence...), scratch, sequence[0], nil
}

// lzwSequence expands one table entry into scratch and returns the sequence
// and the scratch slice, both in order.
func lzwSequence(table []lzwEntry, code int, scratch []byte) ([]byte, []byte) {
	scratch = scratch[:0]
	for at := code; at >= 0; at = table[at].prefix {
		scratch = append(scratch, table[at].suffix)
	}
	for i, j := 0, len(scratch)-1; i < j; i, j = i+1, j-1 {
		scratch[i], scratch[j] = scratch[j], scratch[i]
	}
	return scratch, scratch
}

// lzwWiden reports whether the next code reaches the width limit. Early
// change 1 widens at 2^width - 1 and 0 at 2^width.
func lzwWiden(next, width, early int) bool {
	if width >= lzwMaxWidth {
		return false
	}
	return next >= 1<<width-early
}

// lzwBits reads MSB-first codes from a byte slice.
type lzwBits struct {
	data []byte
	pos  int
}

// read returns the next width-bit code. The bool is false at end of data.
func (bits *lzwBits) read(width int) (int, bool) {
	if width < 1 || bits.pos > len(bits.data)*byteBits-width {
		return 0, false
	}
	value := 0
	for range width {
		octet := bits.data[bits.pos>>bitShift]
		bit := (octet >> (byteLowBits - bits.pos&byteLowBits)) & 1
		value = value<<1 | int(bit)
		bits.pos++
	}
	return value, true
}

// DecodeLZWImageValue decodes one image XObject whose filter is LZWDecode.
// It runs the same sample and shape path as a Flate image, so the level 2
// writer can re-encode an LZW image the way it re-encodes CCITT.
// Any other filter, a color space Spectre cannot paint, and a sample count
// that does not match the dictionary return undefined.
func DecodeLZWImageValue(val Value) (image.Image, error) {
	if !hasImageSubtype(val) {
		return nil, NewError(opImage, errUndefined)
	}
	filter, err := imageFilterName(val)
	if err != nil || filter != opLZW {
		return nil, NewError(opImage, errUndefined)
	}
	width, height, space, err := imageParams(val)
	if err != nil {
		return nil, err
	}
	return decodeFlateImage(val, width, height, space)
}
