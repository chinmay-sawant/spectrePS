package cli

import (
	"bytes"

	"github.com/chinmay-sawant/spectrePS/spectreps"
	"golang.org/x/image/tiff"
)

// tiffEncoding is the -tiffcompress flag value. golang.org/x/image/tiff
// encodes no compression and Deflate only. LZW and CCITT are decode-only.
type tiffEncoding uint8

const (
	tiffDeflate tiffEncoding = iota
	tiffNone
)

// tiffEncodingFromFlag maps the flag text to an encoding. "deflate" names
// tiff.Deflate and "none" names tiff.Uncompressed.
func tiffEncodingFromFlag(name string) (tiffEncoding, bool) {
	switch name {
	case "none":
		return tiffNone, true
	case "deflate":
		return tiffDeflate, true
	default:
		return tiffDeflate, false
	}
}

// encodeTIFF writes the page as a baseline TIFF. The pixels come from the
// same RGBA conversion the PNG and JPEG writers use. The encoder stores
// 8-bit RGBA with associated alpha, which is 24-bit RGB for opaque pages.
func encodeTIFF(img spectreps.PageImage, encoding tiffEncoding) ([]byte, error) {
	compression := tiff.Deflate
	if encoding == tiffNone {
		compression = tiff.Uncompressed
	}
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, rgbaFromPage(img), &tiff.Options{
		Compression: compression,
		Predictor:   false,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
