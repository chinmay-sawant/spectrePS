package pdf

import (
	"bytes"
	"image"
	"image/jpeg"
	"slices"
)

const (
	opImage   = "Image"
	nameImage = "Image"
	nameDCT   = "DCTDecode"

	keySubtype    = "Subtype"
	keyWidth      = "Width"
	keyHeight     = "Height"
	keyBits       = "BitsPerComponent"
	keyColorSpace = "ColorSpace"

	colorRGB  = "DeviceRGB"
	colorGray = "DeviceGray"

	bitsEight      = 8
	rgbComponents  = 3
	rgbaComponents = 4
	opaqueAlpha    = 255
)

// ImageObjectNums returns the in-use object numbers whose resolved object is an
// image XObject: a stream or dictionary with /Subtype /Image.
// The numbers come in ascending order.
func (file *File) ImageObjectNums() ([]int, error) {
	if file == nil {
		return nil, NewError(opImage, errType)
	}
	nums := make([]int, 0, len(file.xref))
	for num, entry := range file.xref {
		if entry.InUse {
			nums = append(nums, num)
		}
	}
	slices.Sort(nums)
	images := make([]int, 0, len(nums))
	for _, num := range nums {
		val, err := file.resolve(num)
		if err != nil {
			return nil, err
		}
		if hasImageSubtype(val) {
			images = append(images, num)
		}
	}
	return images, nil
}

// DecodeImage decodes one image XObject. num comes from ImageObjectNums.
// FlateDecode supports DeviceRGB and DeviceGray at 8 bits per component.
// DCTDecode decodes through image/jpeg.
// Any other filter, color space, or bit depth returns undefined.
// A Flate stream whose byte count does not match width by height by components
// returns undefined too. A failed decode never returns a blank image.
func (file *File) DecodeImage(num int) (image.Image, error) {
	stream, err := file.imageStream(num)
	if err != nil {
		return nil, err
	}
	width, height, space, err := imageParams(stream)
	if err != nil {
		return nil, err
	}
	filter, err := imageFilterName(stream)
	if err != nil {
		return nil, err
	}
	if filter == nameDCT {
		return decodeDCTImage(stream)
	}
	if filter != opFlate {
		return nil, NewError(filter, errUndefined)
	}
	return decodeFlateImage(stream, width, height, space)
}

func (file *File) imageStream(num int) (Value, error) {
	if file == nil {
		return NullVal(), NewError(opImage, errType)
	}
	val, err := file.resolve(num)
	if err != nil {
		return NullVal(), err
	}
	if val.Kind != KindStream || !hasImageSubtype(val) {
		return NullVal(), NewError(opImage, errUndefined)
	}
	return val, nil
}

func hasImageSubtype(val Value) bool {
	if val.Kind != KindStream && val.Kind != KindDict {
		return false
	}
	name, ok := val.NameEntry(keySubtype)
	return ok && name == nameImage
}

func imageParams(stream Value) (int, int, string, error) {
	width, okWidth := stream.IntEntry(keyWidth)
	height, okHeight := stream.IntEntry(keyHeight)
	bits, okBits := stream.IntEntry(keyBits)
	if !okWidth || !okHeight || !okBits || width <= 0 || height <= 0 || bits != bitsEight {
		return 0, 0, "", NewError(opImage, errUndefined)
	}
	space, okSpace := stream.NameEntry(keyColorSpace)
	if !okSpace || (space != colorRGB && space != colorGray) {
		return 0, 0, "", NewError(opImage, errUndefined)
	}
	return width, height, space, nil
}

// imageFilterName returns the one filter name on an image stream.
// A missing filter, a chain, or a non-name filter returns undefined.
func imageFilterName(stream Value) (string, error) {
	entry, ok := stream.ValueEntry(keyFilter)
	if ok && entry.Kind == KindName {
		return entry.Name, nil
	}
	if ok && entry.Kind == KindArray && len(entry.Array) == 1 && entry.Array[0].Kind == KindName {
		return entry.Array[0].Name, nil
	}
	return "", NewError(opImage, errUndefined)
}

func decodeDCTImage(stream Value) (image.Image, error) {
	pic, err := jpeg.Decode(bytes.NewReader(stream.Stream))
	if err != nil {
		return nil, NewError(opImage, errSyntax)
	}
	return pic, nil
}

func decodeFlateImage(stream Value, width, height int, space string) (image.Image, error) {
	raw, err := decodeStream(stream)
	if err != nil {
		return nil, err
	}
	planes := rgbComponents
	if space == colorGray {
		planes = 1
	}
	if int64(width)*int64(height)*int64(planes) != int64(len(raw)) {
		return nil, NewError(opImage, errUndefined)
	}
	if space == colorGray {
		return grayImage(raw, width, height), nil
	}
	return rgbImage(raw, width, height), nil
}

func grayImage(raw []byte, width, height int) image.Image {
	pic := image.NewGray(image.Rect(0, 0, width, height))
	copy(pic.Pix, raw)
	return pic
}

func rgbImage(raw []byte, width, height int) image.Image {
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for idx := range width * height {
		src := idx * rgbComponents
		dst := idx * rgbaComponents
		pic.Pix[dst] = raw[src]
		pic.Pix[dst+1] = raw[src+1]
		pic.Pix[dst+2] = raw[src+2]
		pic.Pix[dst+3] = opaqueAlpha
	}
	return pic
}
