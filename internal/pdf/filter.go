package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
)

const (
	opFlate      = "FlateDecode"
	opPredictor  = "Predictor"
	errUndefined = "undefined"
	errSyntax    = "syntaxerror"
	errLimit     = "limitcheck"

	// maxInflated caps one decoded Flate stream. Predictor growth is not applied.
	maxInflated = 32 << 20
	readChunk   = 4096
)

// Decode applies one filter to raw.
// An empty filterName returns raw unchanged.
// FlateDecode uses compress/zlib, the zlib wrapper around compress/flate.
// Any other name returns NewError(filterName, "undefined").
// params is a DecodeParms dictionary, or a null value when absent.
// Predictor above 1 returns NewError("Predictor", "undefined").
// A missing predictor, predictor 1, or a null params value is plain Flate.
func Decode(filterName string, params Value, raw []byte) ([]byte, error) {
	if filterName == "" {
		return raw, nil
	}
	if filterName != opFlate {
		return nil, NewError(filterName, errUndefined)
	}
	if err := rejectHighPredictor(params); err != nil {
		return nil, err
	}
	return inflate(raw)
}

func rejectHighPredictor(params Value) error {
	if params.Kind == KindNull {
		return nil
	}
	if params.Kind != KindDict {
		return NewError(opPredictor, errSyntax)
	}
	entry, found := params.ValueEntry(opPredictor)
	if !found || entry.Kind == KindNull {
		return nil
	}
	number, ok := params.IntEntry(opPredictor)
	if !ok || number < 1 {
		return NewError(opPredictor, errSyntax)
	}
	if number > 1 {
		return NewError(opPredictor, errUndefined)
	}
	return nil
}

func inflate(raw []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, NewError(opFlate, errSyntax)
	}
	decoded, readErr := readLimited(reader, maxInflated)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, NewError(opFlate, errSyntax)
	}
	return decoded, nil
}

func readLimited(reader io.Reader, limit int) ([]byte, error) {
	buf := make([]byte, 0, readChunk)
	tmp := make([]byte, readChunk)
	for {
		count, err := reader.Read(tmp)
		if count > 0 {
			if count > limit || len(buf) > limit-count {
				return nil, NewError(opFlate, errLimit)
			}
			buf = append(buf, tmp[:count]...)
		}
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil || count == 0 {
			return nil, NewError(opFlate, errSyntax)
		}
	}
}
