package pdfout

import (
	"context"
	"image"
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	// MinCompressionLevel and MaxCompressionLevel bound LevelOverrides.
	MinCompressionLevel = 1
	// MaxCompressionLevel is the highest level LevelOverrides accepts.
	MaxCompressionLevel = 5

	levelFlateImages = 2
	levelMedium      = 3
	levelStrong      = 4
	levelHard        = 5

	sideCapMedium = 1754
	sideCapStrong = 1123
	sideCapHard   = 842
	qualityMedium = 80
	qualityStrong = 60
	qualityHard   = 40

	keyFilter     = "Filter"
	keyParms      = "DecodeParms"
	keyWidth      = "Width"
	keyHeight     = "Height"
	keyColorSpace = "ColorSpace"
	keySubtype    = "Subtype"
	keySMask      = "SMask"

	filterFlate     = "FlateDecode"
	filterDCT       = "DCTDecode"
	subtypeImage    = "Image"
	spaceDeviceRGB  = "DeviceRGB"
	spaceDeviceGray = "DeviceGray"

	opLevel  = "CompressionLevel"
	errType  = "typecheck"
	errRange = "rangecheck"
)

// levelImage is the image column for one compression level: the longest side
// cap in pixels and the DCT quality.
type levelImage struct {
	sideCap int
	quality int
}

// imagePolicy returns the image policy for a level. Levels 1 and 2 leave the
// image size alone, so they have none.
func imagePolicy(level int) (levelImage, bool) {
	switch level {
	case levelMedium:
		return levelImage{sideCap: sideCapMedium, quality: qualityMedium}, true
	case levelStrong:
		return levelImage{sideCap: sideCapStrong, quality: qualityStrong}, true
	case levelHard:
		return levelImage{sideCap: sideCapHard, quality: qualityHard}, true
	default:
		return levelImage{sideCap: 0, quality: 0}, false
	}
}

// LevelOverrides returns complete replacement bodies for the objects one
// compression level changes.
// Level 1 re-Flates every uncompressed page content stream. Level 2 also
// re-encodes Flate and raw image streams losslessly. Levels 3 through 5 also
// re-encode images as DCT, resampling a longest side above the level cap.
// An image Spectre cannot decode, and an image with an /SMask, is copied
// unchanged. Level 0 does not use this function.
// A canceled context returns ctx.Err() and a nil map.
// A nil context panics with "pdfout: nil context".
func LevelOverrides(ctx context.Context, file *pdf.File, level int) (map[int][]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if file == nil {
		return nil, pdf.NewError(opLevel, errType)
	}
	if level < MinCompressionLevel || level > MaxCompressionLevel {
		return nil, pdf.NewError(opLevel, errRange)
	}
	overrides := map[int][]byte{}
	if err := collectOverrides(file, level, overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

func collectOverrides(file *pdf.File, level int, overrides map[int][]byte) error {
	if err := flateContent(file, overrides); err != nil {
		return err
	}
	if level == levelFlateImages {
		if err := flateImages(file, overrides); err != nil {
			return err
		}
	}
	if policy, ok := imagePolicy(level); ok {
		if err := dctImages(file, policy, overrides); err != nil {
			return err
		}
	}
	return nil
}

// flateContent re-Flates every uncompressed content stream of every page.
func flateContent(file *pdf.File, overrides map[int][]byte) error {
	for page := range file.PageCount() {
		nums, err := file.PageContentNums(page)
		if err != nil {
			return err
		}
		for _, num := range nums {
			if err := flateContentObject(file, num, overrides); err != nil {
				return err
			}
		}
	}
	return nil
}

func flateContentObject(file *pdf.File, num int, overrides map[int][]byte) error {
	if _, done := overrides[num]; done {
		return nil
	}
	val, ok, err := file.ObjectValue(num)
	if err != nil {
		return err
	}
	if !ok || val.Kind != pdf.KindStream || streamFiltered(val) {
		return nil
	}
	stored, err := flateBytes(val.Stream)
	if err != nil {
		return err
	}
	overrides[num] = flateOverride(val, stored)
	return nil
}

// flateOverride writes one stream body with /Filter set to Flate and no
// /DecodeParms, keeping every other dictionary entry.
func flateOverride(val pdf.Value, stored []byte) []byte {
	dict := streamDict(val)
	dict[keyFilter] = pdf.NameVal(filterFlate)
	delete(dict, keyParms)
	return pdf.SerializeValue(pdf.StreamVal(dict, stored))
}

// flateImages re-encodes Flate image streams and Flates raw image streams
// without resampling.
func flateImages(file *pdf.File, overrides map[int][]byte) error {
	nums, err := file.ImageObjectNums()
	if err != nil {
		return err
	}
	for _, num := range nums {
		if err := flateImageObject(file, num, overrides); err != nil {
			return err
		}
	}
	return nil
}

func flateImageObject(file *pdf.File, num int, overrides map[int][]byte) error {
	if _, done := overrides[num]; done {
		return nil
	}
	val, ok, err := file.ObjectValue(num)
	if err != nil {
		return err
	}
	if !ok || val.Kind != pdf.KindStream || !imageSubtype(val) {
		return nil
	}
	filter, hasFilter := imageFilter(val)
	if !hasFilter {
		return flateRawImage(val, num, overrides)
	}
	if filter != filterFlate {
		return nil
	}
	return reflateImage(file, val, num, overrides)
}

// flateRawImage wraps uncompressed packed samples in zlib. The raw bytes are
// the sample rows, so no decode is needed.
func flateRawImage(val pdf.Value, num int, overrides map[int][]byte) error {
	stored, err := flateBytes(val.Stream)
	if err != nil {
		return err
	}
	overrides[num] = flateOverride(val, stored)
	return nil
}

// reflateImage decodes a Flate image and writes it back as lossless Flate RGB.
func reflateImage(file *pdf.File, val pdf.Value, num int, overrides map[int][]byte) error {
	pic, err := file.DecodeImage(num)
	if err != nil {
		// An image Spectre cannot decode is copied unchanged.
		return nil //nolint:nilerr // pass-through is the documented policy
	}
	stored, err := EncodeFlateRGB(pic)
	if err != nil {
		return err
	}
	dict := streamDict(val)
	dict[keyFilter] = pdf.NameVal(filterFlate)
	dict[keyColorSpace] = pdf.NameVal(spaceDeviceRGB)
	delete(dict, keyParms)
	overrides[num] = pdf.SerializeValue(pdf.StreamVal(dict, stored))
	return nil
}

// dctImages re-encodes images as DCT, resampling a longest side above the cap.
func dctImages(file *pdf.File, policy levelImage, overrides map[int][]byte) error {
	nums, err := file.ImageObjectNums()
	if err != nil {
		return err
	}
	for _, num := range nums {
		if err := dctImageObject(file, num, policy, overrides); err != nil {
			return err
		}
	}
	return nil
}

func dctImageObject(file *pdf.File, num int, policy levelImage, overrides map[int][]byte) error {
	if _, done := overrides[num]; done {
		return nil
	}
	val, ok, err := file.ObjectValue(num)
	if err != nil {
		return err
	}
	if !ok || val.Kind != pdf.KindStream || !imageSubtype(val) {
		return nil
	}
	if _, hasMask := val.ValueEntry(keySMask); hasMask {
		// Resizing the base image alone would leave its mask at the old size.
		return nil
	}
	return writeDCTImage(file, val, num, policy, overrides)
}

func writeDCTImage(
	file *pdf.File,
	val pdf.Value,
	num int,
	policy levelImage,
	overrides map[int][]byte,
) error {
	pic, err := file.DecodeImage(num)
	if err != nil {
		// An image Spectre cannot decode is copied unchanged.
		return nil //nolint:nilerr // pass-through is the documented policy
	}
	width, height := resampleSize(pic.Bounds().Dx(), pic.Bounds().Dy(), policy.sideCap)
	if width != pic.Bounds().Dx() || height != pic.Bounds().Dy() {
		pic = ScaleImage(pic, width, height)
	}
	stored, err := EncodeDCT(pic, policy.quality)
	if err != nil {
		return err
	}
	dict := streamDict(val)
	dict[keyWidth] = pdf.IntVal(int64(width))
	dict[keyHeight] = pdf.IntVal(int64(height))
	dict[keyFilter] = pdf.NameVal(filterDCT)
	dict[keyColorSpace] = pdf.NameVal(imageColorName(pic))
	delete(dict, keyParms)
	overrides[num] = pdf.SerializeValue(pdf.StreamVal(dict, stored))
	return nil
}

// resampleSize caps the longest side at sideCap and keeps the aspect ratio
// rounded to the nearest pixel. A side at or under the cap keeps its size.
func resampleSize(width, height, sideCap int) (int, int) {
	longest := max(width, height)
	if longest <= sideCap {
		return width, height
	}
	scaledWidth := int(math.Round(float64(width) * float64(sideCap) / float64(longest)))
	scaledHeight := int(math.Round(float64(height) * float64(sideCap) / float64(longest)))
	return max(scaledWidth, minScaleSize), max(scaledHeight, minScaleSize)
}

// imageColorName is the PDF name for the JPEG color model. image/jpeg writes
// gray for *image.Gray and three channels for everything else.
func imageColorName(pic image.Image) string {
	if _, ok := pic.(*image.Gray); ok {
		return spaceDeviceGray
	}
	return spaceDeviceRGB
}

// streamFiltered reports whether a stream carries a filter chain.
func streamFiltered(val pdf.Value) bool {
	entry, ok := val.ValueEntry(keyFilter)
	return ok && entry.Kind != pdf.KindNull
}

// imageFilter returns the one filter name on an image stream. The bool is
// false when the stream has no filter. A chain reports true with an empty name.
func imageFilter(val pdf.Value) (string, bool) {
	entry, ok := val.ValueEntry(keyFilter)
	if !ok || entry.Kind == pdf.KindNull {
		return "", false
	}
	if entry.Kind == pdf.KindName {
		return entry.Name, true
	}
	if entry.Kind == pdf.KindArray && len(entry.Array) == 1 && entry.Array[0].Kind == pdf.KindName {
		return entry.Array[0].Name, true
	}
	return "", true
}

func imageSubtype(val pdf.Value) bool {
	name, ok := val.NameEntry(keySubtype)
	return ok && name == subtypeImage
}

// streamDict copies a stream dictionary so an override can edit its entries.
func streamDict(val pdf.Value) map[string]pdf.Value {
	dict := make(map[string]pdf.Value, len(val.Dict)+1)
	for key, entry := range val.Dict {
		dict[key] = entry
	}
	return dict
}
