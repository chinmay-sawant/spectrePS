package pdf

import "math"

// Color space names, array heads, and function keys the resolver reads.
const (
	colorCMYK       = "DeviceCMYK"
	colorCalRGB     = "CalRGB"
	colorCalGray    = "CalGray"
	colorIndexed    = "Indexed"
	colorICCBased   = "ICCBased"
	colorSeparation = "Separation"
	colorDeviceN    = "DeviceN"

	keyAlternate = "Alternate"
	keyN         = "N"

	// maxColorDepth caps nested color space resolution. A cycle through
	// indirect references is undefined past this depth.
	maxColorDepth = 8
	// maxIndexedHival is the largest Indexed table index at 8 bits per sample.
	maxIndexedHival = 255
	// maxICCComponents is the largest ICCBased channel count the preview maps.
	maxICCComponents = 4
	// cmykComponents counts the DeviceCMYK samples one pixel stores.
	cmykComponents = 4
	// colorSampleScale scales one 8-bit sample to 0 through 1.
	colorSampleScale = 255
	// indexedItems, iccItems, and the rest count one color space array.
	indexedItems    = 4
	iccItems        = 2
	separationItems = 4
	deviceNItems    = 4
)

// colorSpace is one resolved PDF color space: the samples one pixel stores and
// the preview RGB conversion the RGB pixmap paints.
type colorSpace struct {
	// components is the number of samples one pixel stores.
	components int
	// indexed marks an Indexed space, whose single value is a table index and
	// not a sample scaled to 0 through 1.
	indexed bool
	// preview converts one value per component to the preview red, green, and
	// blue, each 0 through 1. Device, ICCBased, Separation, DeviceN, and CIE
	// values run 0 through 1; an Indexed value is the index.
	preview func(values []float64) (red, green, blue float64)
}

// deviceSpace returns one non-indexed space with its preview.
func deviceSpace(components int, preview func([]float64) (float64, float64, float64)) colorSpace {
	return colorSpace{components: components, indexed: false, preview: preview}
}

// rgb converts one value per component to the preview. A short input or a
// zero colorSpace previews black.
func (cs colorSpace) rgb(values []float64) (float64, float64, float64) {
	if cs.preview == nil || len(values) < cs.components {
		return 0, 0, 0
	}
	return cs.preview(values)
}

// sampleValue converts one raw 8-bit stream sample to the value preview takes.
// An Indexed sample is the index; every other sample scales by 1/255.
func (cs colorSpace) sampleValue(raw byte) float64 {
	if cs.indexed {
		return float64(raw)
	}
	return float64(raw) / colorSampleScale
}

// resolveColorSpace resolves one color space value to its sample count and
// preview RGB conversion. DeviceRGB, DeviceGray, DeviceCMYK, Indexed,
// ICCBased, CalRGB, CalGray, Separation, and DeviceN resolve:
//
//   - DeviceCMYK previews with r = (1-C)(1-K) and the matching green and blue
//     terms, the inverse of the K-first rule the writer freezes.
//   - ICCBased uses /Alternate when it is present. Without one, a profile with
//     /N 1, 3, or 4 previews as gray, RGB, or CMYK. The profile bytes are never
//     read, so a real profile transform is out of this subset.
//   - CalRGB previews as RGB and CalGray as gray. The white point, gamma, and
//     matrix entries are ignored.
//   - Separation and DeviceN evaluate their tint transform, then preview the
//     alternate space.
//
// Any other space, a malformed array, and a nested cycle are undefined in
// opName. file may be nil, and then an indirect color space value is
// undefined.
func (file *File) resolveColorSpace(val Value, opName string) (colorSpace, error) {
	return file.colorSpaceAt(val, opName, 0)
}

func (file *File) colorSpaceAt(val Value, opName string, depth int) (colorSpace, error) {
	if depth > maxColorDepth {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	node, err := file.colorNode(val)
	if err != nil {
		return colorSpace{}, err
	}
	switch node.Kind {
	case KindName:
		return deviceColorSpace(node.Name, opName)
	case KindArray:
		return file.arrayColorSpace(node, opName, depth)
	case KindNull, KindBool, KindInt, KindReal, KindString, KindDict, KindStream, KindRef:
		return colorSpace{}, NewError(opName, errUndefined)
	}
	return colorSpace{}, NewError(opName, errUndefined)
}

// colorNode dereferences one color space value. A nil file leaves a reference
// unresolved, and the caller rejects it as undefined.
func (file *File) colorNode(val Value) (Value, error) {
	if val.Kind != KindRef || file == nil {
		return val, nil
	}
	return file.deref(val)
}

// deviceColorSpace resolves one device space name. The one-letter names from
// the ISO 32000-2 abbreviated forms map to the same spaces.
func deviceColorSpace(name, opName string) (colorSpace, error) {
	switch name {
	case colorRGB, "RGB":
		return deviceSpace(rgbComponents, rgbPreview), nil
	case colorGray, "G":
		return deviceSpace(1, grayPreview), nil
	case colorCMYK, "CMYK":
		return deviceSpace(cmykComponents, cmykPreview), nil
	default:
		return colorSpace{}, NewError(opName, errUndefined)
	}
}

func (file *File) arrayColorSpace(node Value, opName string, depth int) (colorSpace, error) {
	items := node.Array
	if len(items) == 0 || items[0].Kind != KindName {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	switch items[0].Name {
	case colorIndexed:
		return file.indexedColorSpace(items, opName, depth)
	case colorICCBased:
		return file.iccColorSpace(items, opName, depth)
	case colorSeparation:
		return file.separationColorSpace(items, opName, depth)
	case colorDeviceN:
		return file.deviceNColorSpace(items, opName, depth)
	case colorCalRGB:
		return deviceSpace(rgbComponents, rgbPreview), nil
	case colorCalGray:
		return deviceSpace(1, grayPreview), nil
	default:
		return colorSpace{}, NewError(opName, errUndefined)
	}
}

// indexedColorSpace resolves [/Indexed base hival lookup]. lookup is a string
// or a stream, and its table converts each index to the base space.
func (file *File) indexedColorSpace(items []Value, opName string, depth int) (colorSpace, error) {
	if len(items) != indexedItems {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	base, err := file.colorSpaceAt(items[1], opName, depth+1)
	if err != nil {
		return colorSpace{}, err
	}
	hival, lookup, err := file.indexedLookup(items, base, opName)
	if err != nil {
		return colorSpace{}, err
	}
	return indexedPreview(lookup, base, hival), nil
}

// indexedLookup validates hival and the lookup length, then returns both.
func (file *File) indexedLookup(items []Value, base colorSpace, opName string) (int, []byte, error) {
	hival, ok := wholeInt(items[2])
	if !ok || hival < 0 || hival > maxIndexedHival {
		return 0, nil, NewError(opName, errUndefined)
	}
	lookup, err := file.lookupBytes(items[3], opName)
	if err != nil {
		return 0, nil, err
	}
	if len(lookup) < (hival+1)*base.components {
		return 0, nil, NewError(opName, errUndefined)
	}
	return hival, lookup, nil
}

// indexedPreview converts every lookup entry to the preview at resolve time,
// so a painted pixel reads one small table.
func indexedPreview(lookup []byte, base colorSpace, hival int) colorSpace {
	table := make([][3]float64, hival+1)
	values := make([]float64, base.components)
	for index := 0; index <= hival; index++ {
		for part := range base.components {
			values[part] = float64(lookup[index*base.components+part]) / colorSampleScale
		}
		red, green, blue := base.rgb(values)
		table[index] = [3]float64{red, green, blue}
	}
	return colorSpace{
		components: 1,
		indexed:    true,
		preview: func(values []float64) (float64, float64, float64) {
			index := int(values[0])
			if math.IsNaN(values[0]) || index < 0 {
				index = 0
			}
			if index > hival {
				index = hival
			}
			entry := table[index]
			return entry[0], entry[1], entry[2]
		},
	}
}

// lookupBytes returns the Indexed lookup table: a string, or a stream decoded
// through the shared filter path.
func (file *File) lookupBytes(val Value, opName string) ([]byte, error) {
	node, err := file.colorNode(val)
	if err != nil {
		return nil, err
	}
	switch node.Kind {
	case KindString:
		return []byte(node.String), nil
	case KindStream:
		return decodeStream(node)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindArray, KindDict, KindRef:
		return nil, NewError(opName, errUndefined)
	}
	return nil, NewError(opName, errUndefined)
}

// iccColorSpace resolves [/ICCBased profile]. The profile's /Alternate wins;
// without one the /N channel count picks the gray, RGB, or CMYK preview.
func (file *File) iccColorSpace(items []Value, opName string, depth int) (colorSpace, error) {
	if len(items) != iccItems {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	profile, err := file.colorNode(items[1])
	if err != nil {
		return colorSpace{}, err
	}
	channels, ok := iccChannels(profile)
	if !ok {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	return file.iccPreview(profile, channels, opName, depth)
}

// iccChannels reads /N from one ICCBased profile stream.
func iccChannels(profile Value) (int, bool) {
	if profile.Kind != KindStream {
		return 0, false
	}
	channels, ok := profile.IntEntry(keyN)
	if !ok || channels < 1 || channels > maxICCComponents {
		return 0, false
	}
	return channels, true
}

func (file *File) iccPreview(
	profile Value,
	channels int,
	opName string,
	depth int,
) (colorSpace, error) {
	if entry, present := profile.ValueEntry(keyAlternate); present && entry.Kind != KindNull {
		alternate, err := file.colorSpaceAt(entry, opName, depth+1)
		if err != nil {
			return colorSpace{}, err
		}
		if alternate.components != channels {
			return colorSpace{}, NewError(opName, errUndefined)
		}
		return alternate, nil
	}
	return iccDevicePreview(channels, opName)
}

// iccDevicePreview picks the device preview by channel count when the profile
// carries no /Alternate.
func iccDevicePreview(channels int, opName string) (colorSpace, error) {
	switch channels {
	case 1:
		return deviceSpace(1, grayPreview), nil
	case rgbComponents:
		return deviceSpace(rgbComponents, rgbPreview), nil
	case cmykComponents:
		return deviceSpace(cmykComponents, cmykPreview), nil
	default:
		return colorSpace{}, NewError(opName, errUndefined)
	}
}

// separationColorSpace resolves [/Separation name alternate tint].
func (file *File) separationColorSpace(items []Value, opName string, depth int) (colorSpace, error) {
	if len(items) != separationItems || items[1].Kind != KindName {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	alternate, err := file.colorSpaceAt(items[2], opName, depth+1)
	if err != nil {
		return colorSpace{}, err
	}
	tint, err := file.loadTintFunction(items[3], 1, alternate.components, opName)
	if err != nil {
		return colorSpace{}, err
	}
	return tintPreview(1, alternate, tint), nil
}

// deviceNColorSpace resolves [/DeviceN names alternate tint].
func (file *File) deviceNColorSpace(items []Value, opName string, depth int) (colorSpace, error) {
	if len(items) != deviceNItems || items[1].Kind != KindArray || len(items[1].Array) == 0 {
		return colorSpace{}, NewError(opName, errUndefined)
	}
	inputs := len(items[1].Array)
	for _, name := range items[1].Array {
		if name.Kind != KindName {
			return colorSpace{}, NewError(opName, errUndefined)
		}
	}
	alternate, err := file.colorSpaceAt(items[2], opName, depth+1)
	if err != nil {
		return colorSpace{}, err
	}
	tint, err := file.loadTintFunction(items[3], inputs, alternate.components, opName)
	if err != nil {
		return colorSpace{}, err
	}
	return tintPreview(inputs, alternate, tint), nil
}

// tintPreview builds the preview closure for a Separation or DeviceN space.
// The tint transform maps the components to the alternate space and the
// alternate preview maps those to RGB. Scratch buffers are reused per call.
func tintPreview(inputs int, alternate colorSpace, tint tintFunction) colorSpace {
	in := make([]float64, inputs)
	out := make([]float64, alternate.components)
	return colorSpace{
		components: inputs,
		indexed:    false,
		preview: func(values []float64) (float64, float64, float64) {
			for index := range in {
				in[index] = clampColorValue(values[index])
			}
			if !tint(out, in) {
				return 0, 0, 0
			}
			return alternate.rgb(out)
		},
	}
}

func rgbPreview(values []float64) (float64, float64, float64) {
	return clampColorValue(values[0]), clampColorValue(values[1]), clampColorValue(values[2])
}

func grayPreview(values []float64) (float64, float64, float64) {
	gray := clampColorValue(values[0])
	return gray, gray, gray
}

// cmykPreview applies the frozen rule r = (1-C)(1-K), and the same for green
// and blue. It is the inverse of the K-first RGB to CMYK rule the writer uses.
func cmykPreview(values []float64) (float64, float64, float64) {
	cyan := clampColorValue(values[0])
	magenta := clampColorValue(values[1])
	yellow := clampColorValue(values[2])
	black := clampColorValue(values[3])
	return (1 - cyan) * (1 - black),
		(1 - magenta) * (1 - black),
		(1 - yellow) * (1 - black)
}

// clampColorValue limits one color value to 0 through 1.
func clampColorValue(value float64) float64 {
	return clampNumber(value, 0, 1)
}

// clampNumber limits a number to a closed range. A reversed range swaps ends.
func clampNumber(value, low, high float64) float64 {
	if low > high {
		low, high = high, low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// wholeInt returns one whole number value as an int.
func wholeInt(val Value) (int, bool) {
	switch val.Kind {
	case KindInt:
		return int(val.Int), true
	case KindReal:
		whole := int64(val.Real)
		if float64(whole) == val.Real {
			return int(whole), true
		}
		return 0, false
	case KindNull, KindBool, KindName, KindString, KindArray, KindDict, KindStream, KindRef:
		return 0, false
	}
	return 0, false
}

// numberOf returns one numeric value. Integers and reals both count.
func numberOf(val Value) (float64, bool) {
	switch val.Kind {
	case KindInt:
		return float64(val.Int), true
	case KindReal:
		return val.Real, true
	case KindNull, KindBool, KindName, KindString, KindArray, KindDict, KindStream, KindRef:
		return 0, false
	}
	return 0, false
}

// numberList returns every number of an array value in order.
func numberList(entry Value) ([]float64, bool) {
	if entry.Kind != KindArray {
		return nil, false
	}
	out := make([]float64, len(entry.Array))
	for index, item := range entry.Array {
		number, ok := numberOf(item)
		if !ok {
			return nil, false
		}
		out[index] = number
	}
	return out, true
}

// numberEntry reads one optional number array. valid is false when the entry
// is present and malformed.
func numberEntry(node Value, key string) ([]float64, bool, bool) {
	entry, ok := node.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return nil, false, true
	}
	nums, valid := numberList(entry)
	return nums, true, valid
}

// numberValue reads one optional number entry.
func numberValue(node Value, key string) (float64, bool) {
	entry, ok := node.ValueEntry(key)
	if !ok {
		return 0, false
	}
	return numberOf(entry)
}
