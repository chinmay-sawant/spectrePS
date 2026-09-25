package pdf

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"
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
	keySMask      = "SMask"

	colorRGB  = "DeviceRGB"
	colorGray = "DeviceGray"

	bitsEight      = 8
	rgbComponents  = 3
	rgbaComponents = 4
	opaqueAlpha    = 255
)

// ImageNameMarker is implemented by markers that record an image XObject
// name before decode. A marker without it keeps today's behavior.
type ImageNameMarker interface {
	ImageName(name string, dict Value)
}

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
// FlateDecode supports DeviceRGB, DeviceGray, DeviceCMYK, Indexed, Separation,
// DeviceN, ICCBased, CalRGB, and CalGray at 8 bits per component, and converts
// every sample to the preview RGB.
// DCTDecode decodes through image/jpeg.
// CCITTFaxDecode decodes Group 4 and Group 3 into Gray at 1 bit per component.
// JPXDecode decodes through the pure-Go JPEG2000 decoder and ignores the
// /BitsPerComponent and /ColorSpace entries, which are optional for JPX.
// Any other filter, color space, or bit depth returns undefined.
// A Flate stream whose byte count does not match width by height by components
// returns undefined too. A failed decode never returns a blank image.
func (file *File) DecodeImage(num int) (image.Image, error) {
	stream, err := file.imageStream(num)
	if err != nil {
		return nil, err
	}
	return file.decodeImageValue(stream, opImage)
}

// DecodeImageValue decodes one resolved image XObject value with the Image
// operator name on an error. The value is the stream form of DecodeImage, so a
// resolved object and its number return the same pixels when the color space
// is direct. An indirect color space needs DecodeImageValueOp and a file.
func DecodeImageValue(val Value) (image.Image, error) {
	return (*File)(nil).decodeImageValue(val, opImage)
}

// DecodeImageValueOp decodes one resolved image XObject value and reports an
// error with the caller's operator name. The content interpreter passes "Do"
// so an unsupported color space keeps the paint operator name.
func (file *File) DecodeImageValueOp(val Value, opName string) (image.Image, error) {
	return file.decodeImageValue(val, opName)
}

func (file *File) decodeImageValue(val Value, opName string) (image.Image, error) {
	if !hasImageSubtype(val) {
		return nil, NewError(opName, errUndefined)
	}
	filter, err := imageFilterName(val)
	if err != nil {
		return nil, err
	}
	if filter == nameJPX {
		return decodeJPXImage(val)
	}
	if filter == nameCCITT {
		return decodeCCITTImage(val)
	}
	width, height, space, err := imageParams(file, val, opName)
	if err != nil {
		return nil, err
	}
	if filter == nameDCT {
		return decodeDCTImage(val, space, opName)
	}
	if filter != opFlate {
		return nil, NewError(filter, errUndefined)
	}
	return decodeFlateImage(val, width, height, space, opName)
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

// imageParams reads /Width, /Height, /BitsPerComponent, and /ColorSpace. The
// color space resolves to its sample count and preview RGB conversion. A
// missing parameter or an unsupported space is undefined in opName.
func imageParams(file *File, stream Value, opName string) (int, int, colorSpace, error) {
	width, okWidth := stream.IntEntry(keyWidth)
	height, okHeight := stream.IntEntry(keyHeight)
	bits, okBits := stream.IntEntry(keyBits)
	if !okWidth || !okHeight || !okBits || width <= 0 || height <= 0 || bits != bitsEight {
		return 0, 0, colorSpace{}, NewError(opName, errUndefined)
	}
	entry, okEntry := stream.ValueEntry(keyColorSpace)
	if !okEntry || entry.Kind == KindNull {
		return 0, 0, colorSpace{}, NewError(opName, errUndefined)
	}
	space, err := file.resolveColorSpace(entry, opName)
	if err != nil {
		return 0, 0, colorSpace{}, err
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

func decodeDCTImage(stream Value, space colorSpace, opName string) (image.Image, error) {
	pic, err := jpeg.Decode(bytes.NewReader(stream.Stream))
	if err != nil {
		return nil, NewError(opImage, errSyntax)
	}
	return previewDecoded(pic, space, opName)
}

// previewDecoded converts one decoded image to the preview RGB the color
// space names. A CMYK space needs a CMYK source, an Indexed or gray space a
// gray source, and any other space takes the decoded RGB channels. A source
// that does not match the declared sample count is undefined in opName.
func previewDecoded(pic image.Image, space colorSpace, opName string) (image.Image, error) {
	if space.components == cmykComponents {
		cmyk, ok := pic.(*image.CMYK)
		if !ok {
			return nil, NewError(opName, errUndefined)
		}
		return previewCMYK(cmyk, space), nil
	}
	if space.components == 1 {
		gray, ok := pic.(*image.Gray)
		if !ok {
			return nil, NewError(opName, errUndefined)
		}
		return previewGray(gray, space), nil
	}
	return previewRGBImage(pic, space), nil
}

// previewCMYK converts a CMYK source through the preview rule.
func previewCMYK(pic *image.CMYK, space colorSpace) image.Image {
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	values := make([]float64, cmykComponents)
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			at := row*pic.Stride + col*cmykComponents
			for part := range cmykComponents {
				values[part] = float64(pic.Pix[at+part]) / colorSampleScale
			}
			red, green, blue := space.rgb(values)
			setPreviewPixel(out, row, col, red, green, blue)
		}
	}
	return out
}

// previewGray converts a gray source: a DeviceGray or CalGray sample scales by
// 1/255, and an Indexed sample is the table index.
func previewGray(pic *image.Gray, space colorSpace) image.Image {
	values := make([]float64, 1)
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			values[0] = space.sampleValue(pic.GrayAt(bounds.Min.X+col, bounds.Min.Y+row).Y)
			red, green, blue := space.rgb(values)
			setPreviewPixel(out, row, col, red, green, blue)
		}
	}
	return out
}

// previewRGBImage converts any source through its RGBA channels.
func previewRGBImage(pic image.Image, space colorSpace) image.Image {
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	values := make([]float64, rgbComponents)
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			red, green, blue, _ := pic.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			values[0] = float64(byte(red>>byteShift)) / colorSampleScale
			values[1] = float64(byte(green>>byteShift)) / colorSampleScale
			values[2] = float64(byte(blue>>byteShift)) / colorSampleScale
			previewRed, previewGreen, previewBlue := space.rgb(values)
			setPreviewPixel(out, row, col, previewRed, previewGreen, previewBlue)
		}
	}
	return out
}

// decodeFlateImage converts one decoded Flate sample stream to the preview.
// The byte count must match width by height by components exactly.
func decodeFlateImage(stream Value, width, height int, space colorSpace, opName string) (image.Image, error) {
	raw, err := decodeStream(stream)
	if err != nil {
		return nil, err
	}
	planes := space.components
	if planes <= 0 || int64(width)*int64(height)*int64(planes) != int64(len(raw)) {
		return nil, NewError(opName, errUndefined)
	}
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	var small [rgbaComponents]float64
	values := small[:planes]
	if planes > len(small) {
		values = make([]float64, planes)
	}
	for index := range width * height {
		at := index * planes
		for part := range planes {
			values[part] = space.sampleValue(raw[at+part])
		}
		red, green, blue := space.rgb(values)
		setPreviewPixel(pic, index/width, index%width, red, green, blue)
	}
	return pic, nil
}

// setPreviewPixel stores one preview RGB value, rounded to a byte.
func setPreviewPixel(pic *image.RGBA, row, col int, red, green, blue float64) {
	at := row*pic.Stride + col*rgbaComponents
	pic.Pix[at] = previewByte(red)
	pic.Pix[at+1] = previewByte(green)
	pic.Pix[at+2] = previewByte(blue)
	pic.Pix[at+3] = opaqueAlpha
}

// previewByte rounds one preview channel to a byte.
func previewByte(value float64) byte {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return byte(math.Round(value * colorSampleScale))
}
