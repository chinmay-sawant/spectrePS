package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
)

const (
	opFlate     = "FlateDecode"
	opLZW       = "LZWDecode"
	opASCII85   = "ASCII85Decode"
	opASCIIHex  = "ASCIIHexDecode"
	opRunLength = "RunLengthDecode"
	opPredictor = "Predictor"

	// maxInflated caps one decoded stream. Predictor growth is not applied.
	maxInflated = 32 << 20
	readChunk   = 4096

	predictorNone  = 1
	predictorTIFF  = 2
	predictorPNG   = 10
	predictorPNGHi = 15

	sampleBitsOne     = 1
	sampleBitsTwo     = 2
	sampleBitsFour    = 4
	sampleBitsEight   = 8
	sampleBitsSixteen = 16

	pngTagNone    = 0
	pngTagSub     = 1
	pngTagUp      = 2
	pngTagAverage = 3
	pngTagPaeth   = 4

	keyPredictor    = "Predictor"
	keyColors       = "Colors"
	keyColumns      = "Columns"
	keyEarlyChange  = "EarlyChange"
	earlyChangeOff  = 0
	earlyChangeOn   = 1
	filterMaxColors = 64

	byteBits     = 8
	byteLowBits  = 7
	bitShift     = 3
	hexShift     = 4
	hexLetterGap = 10
	ascii85Base  = 85
	ascii85Group = 5
	ascii85Bytes = 4
	ascii85Max   = 0xFFFFFFFF
	ascii85Pad   = 'u' - '!'
	ascii85End   = '~'
	ascii85Zero  = 'z'
	ascii85First = '!'
	ascii85Last  = 'u'
	runLengthEOD = 128
	runLengthMax = 257
)

// filterParams is the /DecodeParms subset LZWDecode and FlateDecode accept.
type filterParams struct {
	predictor int
	colors    int
	bits      int
	columns   int
	early     int
}

func defaultFilterParams() filterParams {
	return filterParams{
		predictor: predictorNone,
		colors:    1,
		bits:      sampleBitsEight,
		columns:   1,
		early:     earlyChangeOn,
	}
}

// Decode applies one filter to raw.
// An empty filterName returns raw unchanged.
// FlateDecode and LZWDecode decompress and then apply the /DecodeParms
// predictor. ASCII85Decode, ASCIIHexDecode, and RunLengthDecode decode their
// byte formats.
// Any other name returns NewError(filterName, "undefined").
// A malformed stream returns syntaxerror with the filter name, and a stream
// that decodes past the 32 MiB cap returns limitcheck with the filter name.
// params is a DecodeParms dictionary, or a null value when absent.
func Decode(filterName string, params Value, raw []byte) ([]byte, error) {
	switch filterName {
	case "":
		return raw, nil
	case opFlate:
		return inflate(raw, params)
	case opLZW:
		return decodeLZW(raw, params)
	case opASCII85:
		return decodeASCII85(raw)
	case opASCIIHex:
		return decodeASCIIHex(raw)
	case opRunLength:
		return decodeRunLength(raw)
	default:
		return nil, NewError(filterName, errUndefined)
	}
}

// filterParamsFrom reads the optional parameters LZWDecode and FlateDecode
// share. A null params value selects the defaults. earlyChange is read only
// for LZWDecode. An unsupported predictor value is undefined, and a
// malformed parameter is syntaxerror, both with the Predictor op name.
func filterParamsFrom(params Value, earlyChange bool) (filterParams, error) {
	got := defaultFilterParams()
	if params.Kind == KindNull {
		return got, nil
	}
	if params.Kind != KindDict {
		return got, NewError(opPredictor, errSyntax)
	}
	if err := readPredictor(params, &got); err != nil {
		return got, err
	}
	if err := readRowParams(params, &got); err != nil {
		return got, err
	}
	if earlyChange {
		if err := readEarlyChange(params, &got); err != nil {
			return got, err
		}
	}
	return got, nil
}

func readPredictor(params Value, got *filterParams) error {
	predictor, err := intParam(params, keyPredictor, predictorNone)
	if err != nil {
		return err
	}
	if predictor < predictorNone {
		return NewError(opPredictor, errSyntax)
	}
	if !supportedPredictor(predictor) {
		return NewError(opPredictor, errUndefined)
	}
	got.predictor = predictor
	return nil
}

func supportedPredictor(predictor int) bool {
	if predictor == predictorNone || predictor == predictorTIFF {
		return true
	}
	return predictor >= predictorPNG && predictor <= predictorPNGHi
}

func readRowParams(params Value, got *filterParams) error {
	colors, err := intParam(params, keyColors, 1)
	if err != nil {
		return err
	}
	bits, err := intParam(params, keyBits, sampleBitsEight)
	if err != nil {
		return err
	}
	columns, err := intParam(params, keyColumns, 1)
	if err != nil {
		return err
	}
	if colors < 1 || colors > filterMaxColors || columns < 1 || columns > maxInflated ||
		!validSampleBits(bits) {
		return NewError(opPredictor, errSyntax)
	}
	got.colors = colors
	got.bits = bits
	got.columns = columns
	return nil
}

func readEarlyChange(params Value, got *filterParams) error {
	early, err := intParam(params, keyEarlyChange, earlyChangeOn)
	if err != nil {
		return err
	}
	if early != earlyChangeOff && early != earlyChangeOn {
		return NewError(opPredictor, errSyntax)
	}
	got.early = early
	return nil
}

// intParam returns one whole-number parameter entry, or the fallback when the
// entry is missing or null. A present entry of another kind is syntaxerror.
func intParam(params Value, key string, fallback int) (int, error) {
	entry, found := params.ValueEntry(key)
	if !found || entry.Kind == KindNull {
		return fallback, nil
	}
	value, ok := params.IntEntry(key)
	if !ok {
		return fallback, NewError(opPredictor, errSyntax)
	}
	return value, nil
}

// validSampleBits reports the sample widths the predictor functions support.
func validSampleBits(bits int) bool {
	switch bits {
	case sampleBitsOne, sampleBitsTwo, sampleBitsFour, sampleBitsEight, sampleBitsSixteen:
		return true
	default:
		return false
	}
}

func inflate(raw []byte, params Value) ([]byte, error) {
	parsed, err := filterParamsFrom(params, false)
	if err != nil {
		return nil, err
	}
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, NewError(opFlate, errSyntax)
	}
	decoded, readErr := readLimited(reader, maxInflated, opFlate)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, NewError(opFlate, errSyntax)
	}
	return applyPredictor(decoded, parsed)
}

// applyPredictor reverses the /Predictor transform on one decoded stream.
func applyPredictor(raw []byte, params filterParams) ([]byte, error) {
	switch {
	case params.predictor == predictorNone:
		return raw, nil
	case params.predictor == predictorTIFF:
		return applyTIFFPredictor(raw, params)
	default:
		return applyPNGPredictor(raw, params)
	}
}

// rowLayout returns the decoded row width in bytes and the PNG bytes per
// pixel bpp. A row that cannot fit in raw is syntaxerror.
func rowLayout(raw []byte, params filterParams, tag bool) (int, int, error) {
	rowBits := int64(params.colors) * int64(params.bits) * int64(params.columns)
	rowBytes := (rowBits + byteBits - 1) / byteBits
	if rowBytes < 1 || rowBytes > int64(len(raw)) {
		return 0, 0, NewError(opPredictor, errSyntax)
	}
	row := int(rowBytes)
	if tag && row >= len(raw) {
		return 0, 0, NewError(opPredictor, errSyntax)
	}
	bpp := (params.colors*params.bits + byteBits - 1) / byteBits
	if bpp < 1 {
		bpp = 1
	}
	return row, bpp, nil
}

// applyTIFFPredictor reverses Predictor 2. Each sample is the difference
// between itself and the sample Colors positions to its left, modulo the
// sample width.
func applyTIFFPredictor(raw []byte, params filterParams) ([]byte, error) {
	rowBytes, _, err := rowLayout(raw, params, false)
	if err != nil {
		return nil, err
	}
	if len(raw)%rowBytes != 0 {
		return nil, NewError(opPredictor, errSyntax)
	}
	out := make([]byte, len(raw))
	mask := int64(1)<<params.bits - 1
	history := make([]int64, params.colors)
	samples := params.colors * params.columns
	for row := 0; row < len(raw); row += rowBytes {
		for i := range history {
			history[i] = 0
		}
		for i := range samples {
			bitPos := i * params.bits
			value := readSample(raw[row:row+rowBytes], bitPos, params.bits)
			if i >= params.colors {
				value = (value + history[i%params.colors]) & mask
			}
			history[i%params.colors] = value
			writeSample(out[row:row+rowBytes], bitPos, params.bits, value)
		}
	}
	return out, nil
}

// applyPNGPredictor reverses the PNG predictors 10 through 15. Each encoded
// row is one algorithm tag byte followed by the predicted row bytes. The tag
// selects the algorithm for that row, whatever /Predictor says.
func applyPNGPredictor(raw []byte, params filterParams) ([]byte, error) {
	rowBytes, bpp, err := rowLayout(raw, params, true)
	if err != nil {
		return nil, err
	}
	stride := rowBytes + 1
	if len(raw)%stride != 0 {
		return nil, NewError(opPredictor, errSyntax)
	}
	rows := len(raw) / stride
	out := make([]byte, rows*rowBytes)
	prev := make([]byte, rowBytes)
	for row := range rows {
		tag := int(raw[row*stride])
		src := raw[row*stride+1 : (row+1)*stride]
		dst := out[row*rowBytes : (row+1)*rowBytes]
		if err := pngRow(src, dst, prev, tag, bpp); err != nil {
			return nil, err
		}
		prev = dst
	}
	return out, nil
}

// pngRow writes one unfiltered row. prev is the unfiltered row above.
func pngRow(src, dst, prev []byte, tag, bpp int) error {
	switch tag {
	case pngTagNone:
		copy(dst, src)
	case pngTagSub:
		pngSub(src, dst, bpp)
	case pngTagUp:
		pngUp(src, dst, prev)
	case pngTagAverage:
		pngAverage(src, dst, prev, bpp)
	case pngTagPaeth:
		pngPaeth(src, dst, prev, bpp)
	default:
		return NewError(opPredictor, errSyntax)
	}
	return nil
}

func pngSub(src, dst []byte, bpp int) {
	for i := range dst {
		dst[i] = src[i] + leftByte(dst, i, bpp)
	}
}

func pngUp(src, dst, prev []byte) {
	for i := range dst {
		dst[i] = src[i] + prev[i]
	}
}

func pngAverage(src, dst, prev []byte, bpp int) {
	for i := range dst {
		dst[i] = src[i] + byte((int(leftByte(dst, i, bpp))+int(prev[i]))>>1)
	}
}

func pngPaeth(src, dst, prev []byte, bpp int) {
	for i := range dst {
		dst[i] = src[i] + paeth(leftByte(dst, i, bpp), prev[i], leftByte(prev, i, bpp))
	}
}

func leftByte(row []byte, index, bpp int) byte {
	if index < bpp {
		return 0
	}
	return row[index-bpp]
}

// paeth is the PNG Paeth predictor.
func paeth(left, above, aboveLeft byte) byte {
	base := int(left) + int(above) - int(aboveLeft)
	leftDist := absInt(base - int(left))
	aboveDist := absInt(base - int(above))
	aboveLeftDist := absInt(base - int(aboveLeft))
	if leftDist <= aboveDist && leftDist <= aboveLeftDist {
		return left
	}
	if aboveDist <= aboveLeftDist {
		return above
	}
	return aboveLeft
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// readSample reads one MSB-first sample of bits bits.
func readSample(row []byte, bitPos, bits int) int64 {
	value := int64(0)
	for i := range bits {
		at := bitPos + i
		bit := (row[at>>bitShift] >> (byteLowBits - at&byteLowBits)) & 1
		value = value<<1 | int64(bit)
	}
	return value
}

// writeSample writes one MSB-first sample of bits bits.
func writeSample(row []byte, bitPos, bits int, value int64) {
	for i := range bits {
		at := bitPos + i
		mask := byte(1) << (byteLowBits - at&byteLowBits)
		if value>>(bits-1-i)&1 == 1 {
			row[at>>bitShift] |= mask
		} else {
			row[at>>bitShift] &^= mask
		}
	}
}

// decodeASCIIHex reverses ASCIIHexDecode. Whitespace is ignored, and a
// greater-than sign ends the data. An odd final digit is padded with 0.
func decodeASCIIHex(raw []byte) ([]byte, error) {
	out := make([]byte, 0, len(raw)/2+1)
	high := -1
	for _, cur := range raw {
		if isPDFSpace(cur) {
			continue
		}
		if cur == '>' {
			break
		}
		value := hexDigit(cur)
		if value < 0 {
			return nil, NewError(opASCIIHex, errSyntax)
		}
		if high < 0 {
			high = value
			continue
		}
		if len(out) >= maxInflated {
			return nil, NewError(opASCIIHex, errLimit)
		}
		out = append(out, byte(high<<hexShift|value))
		high = -1
	}
	if high >= 0 {
		if len(out) >= maxInflated {
			return nil, NewError(opASCIIHex, errLimit)
		}
		out = append(out, byte(high<<hexShift))
	}
	return out, nil
}

// hexDigit returns one hexadecimal digit value, or -1.
func hexDigit(cur byte) int {
	switch {
	case cur >= '0' && cur <= '9':
		return int(cur - '0')
	case cur >= 'a' && cur <= 'f':
		return int(cur-'a') + hexLetterGap
	case cur >= 'A' && cur <= 'F':
		return int(cur-'A') + hexLetterGap
	default:
		return -1
	}
}

// ascii85State accumulates one ASCII85 group.
type ascii85State struct {
	out   []byte
	group [ascii85Group]byte
	count int
}

// decodeASCII85 reverses ASCII85Decode. Whitespace is ignored, z is four
// zero bytes, and the ~ marker ends the data. A leading <~ is accepted, and
// the final partial group writes n-1 bytes for n characters. One leftover
// character is syntaxerror.
func decodeASCII85(raw []byte) ([]byte, error) {
	state := ascii85State{
		out:   make([]byte, 0, len(raw)*ascii85Bytes/ascii85Group+ascii85Bytes),
		group: [ascii85Group]byte{},
		count: 0,
	}
	pos := ascii85Start(raw)
	for pos < len(raw) {
		cur := raw[pos]
		pos++
		if isPDFSpace(cur) {
			continue
		}
		if cur == ascii85End {
			return state.finish()
		}
		if err := state.add(cur); err != nil {
			return nil, err
		}
	}
	return state.finish()
}

// ascii85Start skips the optional <~ wrapper.
func ascii85Start(raw []byte) int {
	const wrapper = "<~"
	if len(raw) >= len(wrapper) && raw[0] == wrapper[0] && raw[1] == wrapper[1] {
		return len(wrapper)
	}
	return 0
}

// add consumes one data character.
func (state *ascii85State) add(cur byte) error {
	if cur == ascii85Zero && state.count == 0 {
		if err := state.reserve(ascii85Bytes); err != nil {
			return err
		}
		state.out = append(state.out, 0, 0, 0, 0)
		return nil
	}
	if cur < ascii85First || cur > ascii85Last {
		return NewError(opASCII85, errSyntax)
	}
	state.group[state.count] = cur - ascii85First
	state.count++
	if state.count < ascii85Group {
		return nil
	}
	return state.flush()
}

// flush writes one complete group as four bytes.
func (state *ascii85State) flush() error {
	value, err := ascii85Value(state.group)
	if err != nil {
		return err
	}
	if err := state.reserve(ascii85Bytes); err != nil {
		return err
	}
	quads := ascii85Quads(value)
	state.out = append(state.out, quads[:]...)
	state.count = 0
	return nil
}

// finish writes the final partial group. A count of 0 writes nothing, 2
// through 4 pad with the highest digit and write count-1 bytes, and 1 is
// syntaxerror.
func (state *ascii85State) finish() ([]byte, error) {
	if state.count == 0 {
		return state.out, nil
	}
	if state.count == 1 {
		return nil, NewError(opASCII85, errSyntax)
	}
	for i := state.count; i < ascii85Group; i++ {
		state.group[i] = ascii85Pad
	}
	value, err := ascii85Value(state.group)
	if err != nil {
		return nil, err
	}
	if err := state.reserve(state.count - 1); err != nil {
		return nil, err
	}
	quads := ascii85Quads(value)
	return append(state.out, quads[:state.count-1]...), nil
}

func (state *ascii85State) reserve(need int) error {
	if len(state.out) > maxInflated-need {
		return NewError(opASCII85, errLimit)
	}
	return nil
}

// ascii85Value converts five base-85 digits to a 32-bit value.
func ascii85Value(group [ascii85Group]byte) (uint32, error) {
	value := uint64(0)
	for _, digit := range group {
		value = value*ascii85Base + uint64(digit)
	}
	if value > ascii85Max {
		return 0, NewError(opASCII85, errSyntax)
	}
	return uint32(value), nil
}

// ascii85Quads splits one value into big-endian bytes.
func ascii85Quads(value uint32) [ascii85Bytes]byte {
	const (
		topByte = (ascii85Bytes - 1) * byteBits
		midByte = topByte - byteBits
		lowByte = midByte - byteBits
	)
	return [ascii85Bytes]byte{
		byte(value >> topByte),
		byte(value >> midByte),
		byte(value >> lowByte),
		byte(value),
	}
}

// decodeRunLength reverses RunLengthDecode. A length of 0 through 127 copies
// the next length+1 bytes, 129 through 255 repeats the next byte 257-length
// times, and 128 ends the data.
func decodeRunLength(raw []byte) ([]byte, error) {
	out := make([]byte, 0, len(raw))
	pos := 0
	for pos < len(raw) {
		length := int(raw[pos])
		pos++
		if length == runLengthEOD {
			return out, nil
		}
		if length < runLengthEOD {
			copied := length + 1
			if copied > len(raw)-pos {
				return nil, NewError(opRunLength, errSyntax)
			}
			if len(out) > maxInflated-copied {
				return nil, NewError(opRunLength, errLimit)
			}
			out = append(out, raw[pos:pos+copied]...)
			pos += copied
			continue
		}
		if pos >= len(raw) {
			return nil, NewError(opRunLength, errSyntax)
		}
		repeated := runLengthMax - length
		if len(out) > maxInflated-repeated {
			return nil, NewError(opRunLength, errLimit)
		}
		for range repeated {
			out = append(out, raw[pos])
		}
		pos++
	}
	return out, nil
}

func readLimited(reader io.Reader, limit int, opName string) ([]byte, error) {
	buf := make([]byte, 0, readChunk)
	tmp := make([]byte, readChunk)
	for {
		count, err := reader.Read(tmp)
		if count > 0 {
			if count > limit || len(buf) > limit-count {
				return nil, NewError(opName, errLimit)
			}
			buf = append(buf, tmp[:count]...)
		}
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil || count == 0 {
			return nil, NewError(opName, errSyntax)
		}
	}
}
