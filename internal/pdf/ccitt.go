package pdf

import (
	"bytes"
	"image"

	"golang.org/x/image/ccitt"
)

const (
	nameCCITT = "CCITTFaxDecode"

	keyCCITTParms     = "DecodeParms"
	keyCCITTK         = "K"
	keyCCITTColumns   = "Columns"
	keyCCITTRows      = "Rows"
	keyCCITTEndOfLine = "EndOfLine"
	keyCCITTBlackIs1  = "BlackIs1"
	keyCCITTByteAlign = "EncodedByteAlign"

	bitsBilevel = 1
)

// ccittImage is the decoded /DecodeParms of one CCITT stream plus the size.
type ccittImage struct {
	subFormat ccitt.SubFormat
	options   ccitt.Options
	columns   int
	rows      int
}

// decodeCCITTImage decodes a /CCITTFaxDecode image stream into Gray.
// /K < 0 selects Group 4 and /K == 0 with /EndOfLine true selects Group 3.
// /K > 0, a missing end-of-line marker, and any other bit depth or color
// space return undefined. A stream the decoder rejects returns syntaxerror.
func decodeCCITTImage(stream Value) (image.Image, error) {
	params, err := ccittImageParams(stream)
	if err != nil {
		return nil, err
	}
	if int64(params.columns)*int64(params.rows) > maxInflated {
		return nil, NewError(opImage, errLimit)
	}
	pic := image.NewGray(image.Rect(0, 0, params.columns, params.rows))
	reader := bytes.NewReader(stream.Stream)
	if err := ccitt.DecodeIntoGray(pic, reader, ccitt.MSB, params.subFormat, &params.options); err != nil {
		return nil, NewError(opImage, errSyntax)
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

// ccittDecodeParms reads /DecodeParms into the sub-format, the decoder
// options, and the /Columns and /Rows overrides. A missing /DecodeParms
// leaves /K at 0 and /EndOfLine false, which is undefined.
func ccittDecodeParms(stream Value, width, height int) (ccittImage, error) {
	parms, ok := stream.ValueEntry(keyCCITTParms)
	if !ok {
		parms = NullVal()
	}
	if parms.Kind != KindNull && parms.Kind != KindDict {
		return ccittImage{}, NewError(opImage, errUndefined)
	}
	subFormat, err := ccittSubFormat(parms)
	if err != nil {
		return ccittImage{}, err
	}
	options, err := ccittOptions(parms)
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
	return ccittImage{subFormat: subFormat, options: options, columns: columns, rows: rows}, nil
}

// ccittSubFormat maps /K and /EndOfLine to the ccitt package sub-format.
func ccittSubFormat(parms Value) (ccitt.SubFormat, error) {
	kValue, found, err := ccittInt(parms, keyCCITTK)
	if err != nil {
		return ccitt.Group4, err
	}
	if !found {
		kValue = 0
	}
	switch {
	case kValue < 0:
		return ccitt.Group4, nil
	case kValue > 0:
		return ccitt.Group4, NewError(opImage, errUndefined)
	default:
		endOfLine, err := ccittFlag(parms, keyCCITTEndOfLine)
		if err != nil {
			return ccitt.Group4, err
		}
		if !endOfLine {
			return ccitt.Group4, NewError(opImage, errUndefined)
		}
		return ccitt.Group3, nil
	}
}

// ccittOptions maps /BlackIs1 and /EncodedByteAlign to decoder options.
func ccittOptions(parms Value) (ccitt.Options, error) {
	blackIs1, err := ccittFlag(parms, keyCCITTBlackIs1)
	if err != nil {
		return ccitt.Options{}, err
	}
	byteAlign, err := ccittFlag(parms, keyCCITTByteAlign)
	if err != nil {
		return ccitt.Options{}, err
	}
	return ccitt.Options{Align: byteAlign, Invert: blackIs1}, nil
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
	entry, found := parms.ValueEntry(key)
	if !found || entry.Kind == KindNull {
		return false, nil
	}
	if entry.Kind != KindBool {
		return false, NewError(opImage, errUndefined)
	}
	return entry.Bool, nil
}
