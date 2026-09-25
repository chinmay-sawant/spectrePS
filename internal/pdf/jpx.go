package pdf

import (
	"bytes"
	"image"

	jpeg2000 "github.com/mrjoshuak/go-jpeg2000"
)

const nameJPX = "JPXDecode"

// decodeJPXImage decodes a /JPXDecode image stream.
// The /ColorSpace and /BitsPerComponent entries are optional and ignored for
// JPX: the codestream carries the color and the precision. A header that
// declares more decoded sample bytes than one Flate stream may hold returns
// limitcheck before the decoder allocates. A failed decode returns
// syntaxerror, never a blank image.
func decodeJPXImage(stream Value) (image.Image, error) {
	meta, err := jpeg2000.DecodeMetadata(bytes.NewReader(stream.Stream))
	if err != nil {
		return nil, NewError(opImage, errSyntax)
	}
	if jpxOverCap(meta) {
		return nil, NewError(opImage, errLimit)
	}
	pic, err := jpeg2000.Decode(bytes.NewReader(stream.Stream))
	if err != nil {
		return nil, NewError(opImage, errSyntax)
	}
	return pic, nil
}

// jpxOverCap reports whether one 8-bit sample per component passes the
// maxInflated cap. Nonsense dimensions are left to the decoder to reject.
func jpxOverCap(meta *jpeg2000.Metadata) bool {
	width := int64(meta.Width)
	height := int64(meta.Height)
	components := int64(meta.NumComponents)
	if width <= 0 || height <= 0 || components <= 0 {
		return false
	}
	if width > maxInflated || height > maxInflated || components > maxInflated {
		return true
	}
	return width*height > maxInflated/components
}
