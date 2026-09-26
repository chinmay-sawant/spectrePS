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
	keyImageMask  = "ImageMask"
	keyDecode     = "Decode"
	keyMask       = "Mask"
	keyMatte      = "Matte"

	colorRGB  = "DeviceRGB"
	colorGray = "DeviceGray"

	bitsEight      = 8
	rgbComponents  = 3
	rgbaComponents = 4
	opaqueAlpha    = 255

	// bitsPerByte packs one bilevel image mask row, and maskHalf is the
	// decoded value an image mask pixel needs to paint.
	bitsPerByte = 8
	maskHalf    = 0.5

	// The three RGB channel positions one preview sample walks.
	redChannel   = 0
	greenChannel = 1
	blueChannel  = 2
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
// DeviceN, ICCBased, CalRGB, and CalGray at 8 bits per component, applies a
// /Decode array before the preview conversion, and converts every sample to
// the preview RGB. An /ImageMask true stream decodes to an *image.Alpha at one
// bit per sample.
// DCTDecode decodes through image/jpeg.
// CCITTFaxDecode decodes Group 4 and Group 3 into Gray at 1 bit per component.
// JPXDecode decodes through the pure-Go JPEG2000 decoder and ignores the
// /BitsPerComponent and /ColorSpace entries, which are optional for JPX.
// A decoded image applies its /SMask, its color-key /Mask array, and its
// stencil /Mask stream. /Matte is accepted and ignored.
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

// DecodeImageMaskValue decodes one /ImageMask true value through the Image
// operator name. The result is the coverage plane, not a color image.
func DecodeImageMaskValue(val Value) (*image.Alpha, error) {
	return (*File)(nil).decodeImageMaskValue(val, opImage)
}

// DecodeImageMaskValueOp decodes one /ImageMask true value and reports an
// error with the caller's operator name.
func (file *File) DecodeImageMaskValueOp(val Value, opName string) (*image.Alpha, error) {
	return file.decodeImageMaskValue(val, opName)
}

func (file *File) decodeImageValue(val Value, opName string) (image.Image, error) {
	if !hasImageSubtype(val) {
		return nil, NewError(opName, errUndefined)
	}
	normalized, err := normalizeImageChain(val)
	if err != nil {
		return nil, err
	}
	val = normalized
	if imageMaskFlag(val) {
		return file.decodeImageMaskValue(val, opName)
	}
	filter, err := imageFilterName(val)
	if err != nil {
		return nil, err
	}
	if filter == nameJPX || filter == nameCCITT {
		return file.decodeOuterImage(val, filter, opName)
	}
	return file.decodeSampleImage(val, filter, opName)
}

// decodeOuterImage decodes a JPX or CCITT stream, whose color and precision
// the codestream carries, and applies the image masks. A /Decode array is
// undefined here because the shared sample path does not run.
func (file *File) decodeOuterImage(val Value, filter, opName string) (image.Image, error) {
	if hasImageEntry(val, keyDecode) {
		return nil, NewError(opName, errUndefined)
	}
	var (
		base image.Image
		err  error
	)
	if filter == nameJPX {
		base, err = decodeJPXImage(val)
	} else {
		base, err = decodeCCITTImage(val)
	}
	if err != nil {
		return nil, err
	}
	return file.applyImageMasks(val, base, nil, opName)
}

// decodeSampleImage decodes a DCT or Flate stream through the shared image
// parameters and applies the image masks.
func (file *File) decodeSampleImage(val Value, filter, opName string) (image.Image, error) {
	width, height, space, err := imageParams(file, val, opName)
	if err != nil {
		return nil, err
	}
	if filter != nameDCT && filter != opFlate {
		return nil, NewError(filter, errUndefined)
	}
	base, keyPlane, err := decodeBaseImage(val, filter, width, height, space, opName)
	if err != nil {
		return nil, err
	}
	return file.applyImageMasks(val, base, keyPlane, opName)
}

// decodeBaseImage decodes one Flate or DCT sample stream. A color-key /Mask
// array over a Flate stream also builds the key coverage plane here, because
// only this function holds the raw samples and the color space.
func decodeBaseImage(
	stream Value, filter string, width, height int, space colorSpace, opName string,
) (image.Image, []byte, error) {
	if filter == nameDCT {
		base, err := decodeDCTImage(stream, space, opName)
		if err != nil {
			return nil, nil, err
		}
		return base, nil, nil
	}
	return decodeFlateImageKeyed(stream, width, height, space, opName)
}

// imageMaskFlag reports whether an image dictionary carries /ImageMask true.
func imageMaskFlag(val Value) bool {
	entry, ok := val.ValueEntry(keyImageMask)
	return ok && entry.Kind == KindBool && entry.Bool
}

// hasImageEntry reports whether an image dictionary carries a non-null entry.
func hasImageEntry(val Value, key string) bool {
	entry, ok := val.ValueEntry(key)
	return ok && entry.Kind != KindNull
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

// normalizeImageChain rewrites a multi-filter image value so the codec path
// sees one filter. The leading stages decode through the shared filter chain
// with their parallel /DecodeParms entries, and the returned value carries the
// final filter name, its /DecodeParms, and the decoded bytes. A single filter,
// a one-name array, and no filter pass through unchanged. A non-name chain
// item is undefined in Image.
//
//nolint:cyclop // one branch per chain item and per final filter.
func normalizeImageChain(val Value) (Value, error) {
	entry, ok := val.ValueEntry(keyFilter)
	if !ok || entry.Kind != KindArray || len(entry.Array) < 2 {
		return val, nil
	}
	for _, item := range entry.Array {
		if item.Kind != KindName {
			return NullVal(), NewError(opImage, errUndefined)
		}
	}
	parms := NullVal()
	if found, okParms := val.ValueEntry(keyParms); okParms {
		parms = found
	}
	current := val.Stream
	last := len(entry.Array) - 1
	for idx := range last {
		decoded, err := Decode(entry.Array[idx].Name, paramAt(parms, idx, true), current)
		if err != nil {
			return NullVal(), err
		}
		current = decoded
	}
	out := val
	out.Stream = current
	dict := make(map[string]Value, len(val.Dict)+1)
	for key, item := range val.Dict {
		dict[key] = item
	}
	dict[keyFilter] = NameVal(entry.Array[last].Name)
	if finalParms := paramAt(parms, last, true); finalParms.Kind != KindNull {
		dict[keyParms] = finalParms
	} else {
		delete(dict, keyParms)
	}
	out.Dict = dict
	return out, nil
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
	decode, err := imageDecode(stream, space.components, space.indexed, opName)
	if err != nil {
		return nil, err
	}
	pic, err := jpeg.Decode(bytes.NewReader(stream.Stream))
	if err != nil {
		return nil, NewError(opImage, errSyntax)
	}
	return previewDecoded(pic, space, decode, opName)
}

// previewDecoded converts one decoded image to the preview RGB the color
// space names. A CMYK space needs a CMYK source, an Indexed or gray space a
// gray source, and any other space takes the decoded RGB channels. A source
// that does not match the declared sample count is undefined in opName. A
// /Decode array remaps each sample before the conversion.
func previewDecoded(pic image.Image, space colorSpace, decode []float64, opName string) (image.Image, error) {
	if space.components == cmykComponents {
		cmyk, ok := pic.(*image.CMYK)
		if !ok {
			return nil, NewError(opName, errUndefined)
		}
		return previewCMYK(cmyk, space, decode), nil
	}
	if space.components == 1 {
		gray, ok := pic.(*image.Gray)
		if !ok {
			return nil, NewError(opName, errUndefined)
		}
		return previewGray(gray, space, decode), nil
	}
	return previewRGBImage(pic, space, decode), nil
}

// previewCMYK converts a CMYK source through the preview rule.
func previewCMYK(pic *image.CMYK, space colorSpace, decode []float64) image.Image {
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	values := make([]float64, cmykComponents)
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			at := row*pic.Stride + col*cmykComponents
			for part := range cmykComponents {
				values[part] = decodedSample(space, pic.Pix[at+part], decode, part)
			}
			red, green, blue := space.rgb(values)
			setPreviewPixel(out, row, col, red, green, blue)
		}
	}
	return out
}

// previewGray converts a gray source: a DeviceGray or CalGray sample scales by
// 1/255, and an Indexed sample is the table index.
func previewGray(pic *image.Gray, space colorSpace, decode []float64) image.Image {
	values := make([]float64, 1)
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			gray := pic.GrayAt(bounds.Min.X+col, bounds.Min.Y+row).Y
			values[0] = decodedSample(space, gray, decode, 0)
			red, green, blue := space.rgb(values)
			setPreviewPixel(out, row, col, red, green, blue)
		}
	}
	return out
}

// previewRGBImage converts any source through its RGBA channels.
func previewRGBImage(pic image.Image, space colorSpace, decode []float64) image.Image {
	bounds := pic.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	values := make([]float64, rgbComponents)
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			red, green, blue, _ := pic.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			values[redChannel] = decodedSample(space, byte(red>>byteShift), decode, redChannel)
			values[greenChannel] = decodedSample(space, byte(green>>byteShift), decode, greenChannel)
			values[blueChannel] = decodedSample(space, byte(blue>>byteShift), decode, blueChannel)
			previewRed, previewGreen, previewBlue := space.rgb(values)
			setPreviewPixel(out, row, col, previewRed, previewGreen, previewBlue)
		}
	}
	return out
}

// decodeFlateImage converts one decoded Flate sample stream to the preview
// and drops the optional color-key plane. DecodeLZWImageValue uses it.
func decodeFlateImage(
	stream Value, width, height int, space colorSpace, opName string,
) (image.Image, error) {
	pic, _, err := decodeFlateImageKeyed(stream, width, height, space, opName)
	return pic, err
}

// decodeFlateImageKeyed converts one decoded Flate sample stream to the
// preview and, when the dictionary carries a color-key /Mask array, to the
// coverage plane too. The byte count must match width by height by components
// exactly.
func decodeFlateImageKeyed(
	stream Value, width, height int, space colorSpace, opName string,
) (image.Image, []byte, error) {
	raw, err := decodeStream(stream)
	if err != nil {
		return nil, nil, err
	}
	planes := space.components
	if planes <= 0 || int64(width)*int64(height)*int64(planes) != int64(len(raw)) {
		return nil, nil, NewError(opName, errUndefined)
	}
	decode, err := imageDecode(stream, planes, space.indexed, opName)
	if err != nil {
		return nil, nil, err
	}
	ranges, err := imageMaskRanges(stream, planes, opName)
	if err != nil {
		return nil, nil, err
	}
	pic, keyPlane := flatePreview(raw, width, height, planes, space, decode, ranges)
	return pic, keyPlane, nil
}

// flatePreview converts one decoded sample stream to the preview and, when
// ranges is not nil, to the color-key coverage plane too.
func flatePreview(
	raw []byte, width, height, planes int, space colorSpace, decode, ranges []float64,
) (image.Image, []byte) {
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	var keyPlane []byte
	if ranges != nil {
		keyPlane = make([]byte, width*height)
	}
	var small [rgbaComponents]float64
	values := small[:planes]
	if planes > len(small) {
		values = make([]float64, planes)
	}
	for index := range width * height {
		at := index * planes
		for part := range values {
			values[part] = decodedSample(space, raw[at+part], decode, part)
		}
		if keyPlane != nil {
			keyPlane[index] = colorKeyCoverage(values, ranges)
		}
		red, green, blue := space.rgb(values)
		setPreviewPixel(pic, index/width, index%width, red, green, blue)
	}
	return pic, keyPlane
}

// decodedSample converts one raw 8-bit sample to the value the preview takes,
// after the /Decode array when one is present.
func decodedSample(space colorSpace, raw byte, decode []float64, part int) float64 {
	sample := space.sampleValue(raw)
	if decode == nil || part*domainPair+1 >= len(decode) {
		return sample
	}
	low := decode[part*domainPair]
	high := decode[part*domainPair+1]
	return low + sample*(high-low)
}

// imageDecode reads the /Decode array for a color space with components
// samples. A missing entry returns nil. An Indexed space keeps its table index
// and refuses a decode array, and a malformed array is undefined in opName.
func imageDecode(stream Value, components int, indexed bool, opName string) ([]float64, error) {
	entry, ok := stream.ValueEntry(keyDecode)
	if !ok || entry.Kind == KindNull {
		return nil, nil
	}
	nums, valid := numberList(entry)
	if !valid || len(nums) != domainPair*components || indexed {
		return nil, NewError(opName, errUndefined)
	}
	return nums, nil
}

// imageMaskRanges reads a color-key /Mask array, the 2n sample ranges that
// key a pixel out. A missing entry and a stream /Mask both return nil; the
// stream form is a stencil and is handled with the other masks.
func imageMaskRanges(stream Value, components int, opName string) ([]float64, error) {
	entry, ok := stream.ValueEntry(keyMask)
	if !ok || entry.Kind == KindNull {
		return nil, nil
	}
	if entry.Kind != KindArray {
		return nil, nil
	}
	nums, valid := numberList(entry)
	if !valid || len(nums) != domainPair*components {
		return nil, NewError(opName, errUndefined)
	}
	return nums, nil
}

// colorKeyCoverage returns 0 when every component falls inside its key range
// and 255 otherwise. A pixel inside the range is masked out.
func colorKeyCoverage(values, ranges []float64) byte {
	for part := range values {
		low, high := ranges[part*domainPair], ranges[part*domainPair+1]
		if low > high {
			low, high = high, low
		}
		if values[part] < low || values[part] > high {
			return opaqueAlpha
		}
	}
	return 0
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

// applyImageMasks turns one decoded image into the image the device paints,
// combining the /SMask, the color-key /Mask array coverage, and the stencil
// /Mask stream. /Matte is accepted and ignored: this subset does not remove a
// matte color, which is a deviation recorded in documentation/devices.md.
// An image with no mask returns the decoded image unchanged.
func (file *File) applyImageMasks(
	val Value, base image.Image, keyPlane []byte, opName string,
) (image.Image, error) {
	width := base.Bounds().Dx()
	height := base.Bounds().Dy()
	plane := keyPlane
	sheet, err := file.imageMaskPlane(val, width, height, keyPlane != nil, opName)
	if err != nil {
		return nil, err
	}
	plane = combinePlanes(plane, sheet)
	soft, err := file.valueSoftMaskPlane(val, width, height, opName)
	if err != nil {
		return nil, err
	}
	plane = combinePlanes(plane, soft)
	if plane == nil {
		return base, nil
	}
	return withAlpha(base, plane), nil
}

// valueSoftMaskPlane reads the optional /SMask entry of one image dictionary.
func (file *File) valueSoftMaskPlane(
	val Value, width, height int, opName string,
) ([]byte, error) {
	entry, ok := val.ValueEntry(keySMask)
	if !ok || entry.Kind == KindNull {
		return nil, nil
	}
	return file.imageSoftMaskPlane(entry, width, height, opName)
}

// imageMaskPlane returns the coverage plane one image dictionary asks for. A
// color-key array that the decoder could not build is undefined, because the
// raw samples are gone.
func (file *File) imageMaskPlane(
	val Value, width, height int, keyed bool, opName string,
) ([]byte, error) {
	entry, ok := val.ValueEntry(keyMask)
	if !ok || entry.Kind == KindNull {
		return nil, nil
	}
	node, err := derefImageEntry(file, entry, opName)
	if err != nil {
		return nil, err
	}
	switch node.Kind {
	case KindArray:
		if !keyed {
			return nil, NewError(opName, errUndefined)
		}
		return nil, nil
	case KindStream:
		stencil, err := file.decodeImageMaskValue(node, opName)
		if err != nil {
			return nil, err
		}
		return imageAlphaPlane(stencil, width, height, opName)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindString, KindDict, KindRef:
		return nil, NewError(opName, errUndefined)
	}
	return nil, NewError(opName, errUndefined)
}

// imageSoftMaskPlane returns the coverage plane of one /SMask stream.
func (file *File) imageSoftMaskPlane(val Value, width, height int, opName string) ([]byte, error) {
	node, err := derefImageEntry(file, val, opName)
	if err != nil {
		return nil, err
	}
	if imageMaskFlag(node) {
		mask, err := file.decodeImageMaskValue(node, opName)
		if err != nil {
			return nil, err
		}
		return imageAlphaPlane(mask, width, height, opName)
	}
	plane, err := file.decodeGrayMaskPlane(node, opName)
	if err != nil {
		return nil, err
	}
	if len(plane) != width*height {
		return nil, NewError(opName, errUndefined)
	}
	return plane, nil
}

// decodeGrayMaskPlane decodes one 8-bit gray mask image into a coverage plane.
func (file *File) decodeGrayMaskPlane(node Value, opName string) ([]byte, error) {
	if node.Kind != KindStream {
		return nil, NewError(opName, errUndefined)
	}
	width, height, space, err := imageParams(file, node, opName)
	if err != nil {
		return nil, err
	}
	if space.components != 1 {
		return nil, NewError(opName, errUndefined)
	}
	filter, err := imageFilterName(node)
	if err != nil {
		return nil, err
	}
	if filter != opFlate {
		return nil, NewError(filter, errUndefined)
	}
	raw, err := decodeStream(node)
	if err != nil {
		return nil, err
	}
	if len(raw) != width*height {
		return nil, NewError(opName, errUndefined)
	}
	decode, err := imageDecode(node, 1, false, opName)
	if err != nil {
		return nil, err
	}
	plane := make([]byte, width*height)
	for idx, sample := range raw {
		plane[idx] = previewByte(decodedSample(space, sample, decode, 0))
	}
	return plane, nil
}

// imageAlphaPlane copies one image mask into a plane of the expected size.
func imageAlphaPlane(mask *image.Alpha, width, height int, opName string) ([]byte, error) {
	if mask.Bounds().Dx() != width || mask.Bounds().Dy() != height {
		return nil, NewError(opName, errUndefined)
	}
	plane := make([]byte, 0, width*height)
	stride := mask.Stride
	for row := range height {
		plane = append(plane, mask.Pix[row*stride:row*stride+width]...)
	}
	return plane, nil
}

// combinePlanes multiplies two coverage planes. A nil plane means opaque, so
// combining nil with anything returns the other plane.
func combinePlanes(left, right []byte) []byte {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	for idx := range left {
		left[idx] = byte(int(left[idx]) * int(right[idx]) / opaqueAlpha)
	}
	return left
}

// withAlpha builds one RGBA image with the base color and the coverage plane.
func withAlpha(base image.Image, plane []byte) *image.RGBA {
	bounds := base.Bounds()
	width := bounds.Dx()
	out := image.NewRGBA(image.Rect(0, 0, width, bounds.Dy()))
	for row := range bounds.Dy() {
		for col := range width {
			red, green, blue, _ := base.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			at := row*out.Stride + col*rgbaComponents
			out.Pix[at] = byte(red >> byteShift)
			out.Pix[at+1] = byte(green >> byteShift)
			out.Pix[at+2] = byte(blue >> byteShift)
			out.Pix[at+3] = plane[row*width+col]
		}
	}
	return out
}

// derefImageEntry resolves one image dictionary entry through the file when
// it is indirect. A reference without a file is undefined in opName.
func derefImageEntry(file *File, entry Value, opName string) (Value, error) {
	if entry.Kind != KindRef {
		return entry, nil
	}
	if file == nil {
		return NullVal(), NewError(opName, errUndefined)
	}
	return file.deref(entry)
}

// decodeImageMaskValue decodes one /ImageMask true stream at one bit per
// sample. /Decode [1 0] inverts the mask. A marker of 1 paints; the caller
// tints the plane with the current fill color.
func (file *File) decodeImageMaskValue(val Value, opName string) (*image.Alpha, error) {
	normalized, err := normalizeImageChain(val)
	if err != nil {
		return nil, err
	}
	val = normalized
	if !imageMaskFlag(val) {
		return nil, NewError(opName, errUndefined)
	}
	params, err := imageMaskParamsOf(val, opName)
	if err != nil {
		return nil, err
	}
	raw, err := decodeStream(val)
	if err != nil {
		return nil, err
	}
	if len(raw) != params.rowBytes*params.height {
		return nil, NewError(opName, errUndefined)
	}
	return maskedAlpha(raw, params), nil
}

// imageMaskParams is the validated geometry and decode range of one image
// mask.
type imageMaskParams struct {
	width    int
	height   int
	rowBytes int
	low      float64
	high     float64
}

// imageMaskParamsOf reads and validates /Width, /Height, /BitsPerComponent,
// the filter, and /Decode for one image mask.
func imageMaskParamsOf(val Value, opName string) (imageMaskParams, error) {
	width, okWidth := val.IntEntry(keyWidth)
	height, okHeight := val.IntEntry(keyHeight)
	if !okWidth || !okHeight || width <= 0 || height <= 0 {
		return imageMaskParams{}, NewError(opName, errUndefined)
	}
	if bits, ok := val.IntEntry(keyBits); ok && bits != bitsBilevel {
		return imageMaskParams{}, NewError(opName, errUndefined)
	}
	filter, err := imageFilterName(val)
	if err != nil {
		return imageMaskParams{}, err
	}
	if filter != opFlate {
		return imageMaskParams{}, NewError(filter, errUndefined)
	}
	low, high, err := imageMaskDecode(val, opName)
	if err != nil {
		return imageMaskParams{}, err
	}
	return imageMaskParams{
		width:    width,
		height:   height,
		rowBytes: (width + bitsPerByte - 1) / bitsPerByte,
		low:      low,
		high:     high,
	}, nil
}

// maskedAlpha turns one packed bilevel mask into an alpha plane. A decoded
// value at or above one half paints, so /Decode [1 0] inverts the mask.
func maskedAlpha(raw []byte, params imageMaskParams) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, params.width, params.height))
	for row := range params.height {
		for col := range params.width {
			bit := float64(maskBit(raw, params.rowBytes, row, col))
			if params.low+bit*(params.high-params.low) >= maskHalf {
				mask.Pix[row*mask.Stride+col] = opaqueAlpha
			}
		}
	}
	return mask
}

// imageMaskDecode reads the /Decode array of an image mask, defaulting to
// [0 1]. The entry order is kept, so /Decode [1 0] inverts the mask.
func imageMaskDecode(val Value, opName string) (float64, float64, error) {
	entry, ok := val.ValueEntry(keyDecode)
	if !ok || entry.Kind == KindNull {
		return 0, 1, nil
	}
	nums, valid := numberList(entry)
	if !valid || len(nums) != domainPair {
		return 0, 0, NewError(opName, errUndefined)
	}
	return nums[0], nums[1], nil
}

// maskBit returns bit col of one packed bilevel row, most significant bit
// first.
func maskBit(raw []byte, rowBytes, row, col int) byte {
	at := row*rowBytes + col/bitsPerByte
	return raw[at] >> (bitsPerByte - 1 - col%bitsPerByte) & 1
}
