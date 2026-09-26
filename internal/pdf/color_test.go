package pdf

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// previewTolerance is the float slack for a preview check.
const previewTolerance = 1e-9

// TestColorSpaceDeviceCMYK resolves DeviceCMYK and checks the frozen preview
// rule r = (1-C)(1-K).
func TestColorSpaceDeviceCMYK(t *testing.T) {
	t.Parallel()
	space := colorSpaceFor(t, nil, NameVal(colorCMYK))
	checkComponents(t, space, cmykComponents)
	checkColorPreview(t, space, []float64{1, 0, 0, 0}, 0, 1, 1)
	checkColorPreview(t, space, []float64{0, 1, 1, 0}, 1, 0, 0)
	checkColorPreview(t, space, []float64{0, 0, 0, 0}, 1, 1, 1)
	checkColorPreview(t, space, []float64{0, 0, 0, 1}, 0, 0, 0)
	checkColorPreview(t, space, []float64{0, 0, 0, 0.5}, 0.5, 0.5, 0.5)
	// The abbreviated name is the same space.
	short := colorSpaceFor(t, nil, NameVal("CMYK"))
	checkColorPreview(t, short, []float64{1, 0, 0, 0}, 0, 1, 1)
}

// TestColorSpaceIndexed resolves an Indexed space over a string table and over
// a stream table reached through the file.
func TestColorSpaceIndexed(t *testing.T) {
	t.Parallel()
	space := ArrayVal([]Value{
		NameVal(colorIndexed), NameVal(colorRGB), IntVal(1),
		StringVal(string([]byte{255, 0, 0, 0, 255, 0})),
	})
	resolved := colorSpaceFor(t, nil, space)
	checkComponents(t, resolved, 1)
	if !resolved.indexed {
		t.Fatal("indexed flag")
	}
	checkColorPreview(t, resolved, []float64{0}, 1, 0, 0)
	checkColorPreview(t, resolved, []float64{1}, 0, 1, 0)
	// An index past hival clamps to the last table entry.
	checkColorPreview(t, resolved, []float64{9}, 0, 1, 0)

	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	tableNum := doc.object(streamBody("/Filter /FlateDecode",
		flateRaw(t, []byte{0, 255})))
	file := mustOpen(t, doc.classic(""))
	streamSpace := ArrayVal([]Value{
		NameVal(colorIndexed), NameVal(colorGray), IntVal(1), RefVal(tableNum, 0),
	})
	streamResolved := colorSpaceFor(t, file, streamSpace)
	checkColorPreview(t, streamResolved, []float64{0}, 0, 0, 0)
	checkColorPreview(t, streamResolved, []float64{1}, 1, 1, 1)

	cmyk := ArrayVal([]Value{
		NameVal(colorIndexed), NameVal(colorCMYK), IntVal(0),
		StringVal(string([]byte{0, 255, 255, 0})),
	})
	checkColorPreview(t, colorSpaceFor(t, nil, cmyk), []float64{0}, 1, 0, 0)

	short := ArrayVal([]Value{
		NameVal(colorIndexed), NameVal(colorRGB), IntVal(2), StringVal(string([]byte{0, 0, 0})),
	})
	checkSpaceUndefined(t, nil, short)
	high := ArrayVal([]Value{
		NameVal(colorIndexed), NameVal(colorRGB), IntVal(256),
		StringVal(string([]byte{0, 0, 0})),
	})
	checkSpaceUndefined(t, nil, high)
}

// TestColorSpaceICCBased resolves ICCBased through /Alternate and falls back
// to the /N channel count when no alternate is present.
func TestColorSpaceICCBased(t *testing.T) {
	t.Parallel()
	cmykProfile := StreamVal(map[string]Value{
		keyN: IntVal(4), keyAlternate: NameVal(colorCMYK),
	}, nil)
	space := ArrayVal([]Value{NameVal(colorICCBased), cmykProfile})
	resolved := colorSpaceFor(t, nil, space)
	checkComponents(t, resolved, cmykComponents)
	checkColorPreview(t, resolved, []float64{1, 0, 0, 0}, 0, 1, 1)

	rgbProfile := StreamVal(map[string]Value{keyN: IntVal(3)}, nil)
	resolved = colorSpaceFor(t, nil,
		ArrayVal([]Value{NameVal(colorICCBased), rgbProfile}))
	checkComponents(t, resolved, rgbComponents)
	checkColorPreview(t, resolved, []float64{1, 0, 0}, 1, 0, 0)

	grayProfile := StreamVal(map[string]Value{keyN: IntVal(1)}, nil)
	resolved = colorSpaceFor(t, nil,
		ArrayVal([]Value{NameVal(colorICCBased), grayProfile}))
	checkComponents(t, resolved, 1)
	checkColorPreview(t, resolved, []float64{0.25}, 0.25, 0.25, 0.25)

	mismatch := StreamVal(map[string]Value{
		keyN: IntVal(4), keyAlternate: NameVal(colorRGB),
	}, nil)
	checkSpaceUndefined(t, nil,
		ArrayVal([]Value{NameVal(colorICCBased), mismatch}))

	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	profileNum := doc.object(streamBody("/N 3 /Alternate /DeviceRGB", nil))
	file := mustOpen(t, doc.classic(""))
	indirect := ArrayVal([]Value{NameVal(colorICCBased), RefVal(profileNum, 0)})
	checkComponents(t, colorSpaceFor(t, file, indirect), rgbComponents)
}

// TestColorSpaceCalAndUnsupported covers the CIE spaces the subset treats as
// RGB and gray, and the spaces it refuses.
func TestColorSpaceCalAndUnsupported(t *testing.T) {
	t.Parallel()
	calRGB := ArrayVal([]Value{NameVal(colorCalRGB), DictVal(map[string]Value{
		"WhitePoint": ArrayVal([]Value{RealVal(0.95), IntVal(1), RealVal(1.09)}),
	})})
	resolved := colorSpaceFor(t, nil, calRGB)
	checkComponents(t, resolved, rgbComponents)
	checkColorPreview(t, resolved, []float64{0, 1, 0}, 0, 1, 0)

	calGray := ArrayVal([]Value{NameVal(colorCalGray), DictVal(nil)})
	resolved = colorSpaceFor(t, nil, calGray)
	checkComponents(t, resolved, 1)
	checkColorPreview(t, resolved, []float64{0.5}, 0.5, 0.5, 0.5)

	checkSpaceUndefined(t, nil, NameVal("Pattern"))
	checkSpaceUndefined(t, nil, ArrayVal([]Value{NameVal("Lab"), DictVal(nil)}))
	checkSpaceUndefined(t, nil, ArrayVal(nil))
	checkSpaceUndefined(t, nil, IntVal(3))
	checkSpaceUndefined(t, nil, RefVal(5, 0))
}

// TestTintTransformSampled evaluates a type 2 exponential interpolation tint
// transform. Type 0 sampled and type 3 stitching functions stay out.
func TestTintTransformSampled(t *testing.T) {
	t.Parallel()
	tint := DictVal(map[string]Value{
		keyFunctionType: IntVal(funcTypeExponential),
		keyDomain:       colorNumberArray(0, 1),
		keyC0:           colorNumberArray(0, 0, 0, 0),
		keyC1:           colorNumberArray(1, 1, 1, 0),
		keyN:            RealVal(1),
	})
	resolved := colorSpaceFor(t, nil,
		separationSpace(NameVal(colorCMYK), tint))
	checkComponents(t, resolved, 1)
	checkColorPreview(t, resolved, []float64{0}, 1, 1, 1)
	checkColorPreview(t, resolved, []float64{1}, 0, 0, 0)
	checkColorPreview(t, resolved, []float64{0.5}, 0.5, 0.5, 0.5)

	square := DictVal(map[string]Value{
		keyFunctionType: IntVal(funcTypeExponential),
		keyDomain:       colorNumberArray(0, 1),
		keyC0:           colorNumberArray(0),
		keyC1:           colorNumberArray(1),
		keyN:            RealVal(2),
	})
	resolved = colorSpaceFor(t, nil,
		separationSpace(NameVal(colorGray), square))
	checkColorPreview(t, resolved, []float64{0.5}, 0.25, 0.25, 0.25)

	sampled := DictVal(map[string]Value{keyFunctionType: IntVal(0)})
	checkSpaceUndefined(t, nil, separationSpace(NameVal(colorGray), sampled))
	stitched := DictVal(map[string]Value{keyFunctionType: IntVal(3)})
	checkSpaceUndefined(t, nil, separationSpace(NameVal(colorGray), stitched))
}

// TestTintTransformCalculator evaluates type 4 PostScript calculator tint
// transforms: arithmetic, comparison, ifelse, and the stack operators.
func TestTintTransformCalculator(t *testing.T) {
	t.Parallel()
	// dup 1 exch sub 0 0 maps tint t to C=t, M=1-t, Y=0, K=0.
	space := separationSpace(NameVal(colorCMYK), calculatorValue(1, "dup 1 exch sub 0 0"))
	resolved := colorSpaceFor(t, nil, space)
	checkComponents(t, resolved, 1)
	checkColorPreview(t, resolved, []float64{0}, 1, 0, 1)
	checkColorPreview(t, resolved, []float64{1}, 0, 1, 1)
	checkColorPreview(t, resolved, []float64{0.5}, 0.5, 0.5, 1)

	// dup 0.5 lt { 0 } { 1 } ifelse exch pop 0 0 0 picks C from one half of
	// the domain and leaves four outputs.
	branch := separationSpace(NameVal(colorCMYK),
		calculatorValue(1, "dup 0.5 lt { 0 } { 1 } ifelse exch pop 0 0 0"))
	resolved = colorSpaceFor(t, nil, branch)
	checkColorPreview(t, resolved, []float64{0.25}, 1, 1, 1)
	checkColorPreview(t, resolved, []float64{0.75}, 0, 1, 1)

	checkSpaceUndefined(t, nil,
		separationSpace(NameVal(colorCMYK), calculatorValue(1, "bogus")))
	checkSpaceUndefined(t, nil,
		separationSpace(NameVal(colorCMYK),
			DictVal(map[string]Value{keyFunctionType: IntVal(3)})))
}

// TestSeparationColor loads a Separation with its alternate space and a tint
// transform stream reached through an indirect reference.
func TestSeparationColor(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	tintNum := doc.object(streamBody(
		"/FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [1 1 1 0] /N 1", nil))
	file := mustOpen(t, doc.classic(""))

	space := separationSpace(NameVal(colorCMYK), RefVal(tintNum, 0))
	resolved := colorSpaceFor(t, file, space)
	checkComponents(t, resolved, 1)
	checkColorPreview(t, resolved, []float64{0}, 1, 1, 1)
	checkColorPreview(t, resolved, []float64{1}, 0, 0, 0)
	checkColorPreview(t, resolved, []float64{0.5}, 0.5, 0.5, 0.5)

	// The tint output count must match the alternate component count.
	checkSpaceUndefined(t, file,
		separationSpace(NameVal(colorGray), RefVal(tintNum, 0)))
	// An unsupported alternate space refuses the separation.
	checkSpaceUndefined(t, file,
		separationSpace(NameVal("Lab"), RefVal(tintNum, 0)))
	// A missing tint transform refuses.
	checkSpaceUndefined(t, file,
		ArrayVal([]Value{NameVal(colorSeparation), NameVal("Spot"), NameVal(colorCMYK)}))
}

// TestDeviceNColor loads a DeviceN space whose type 4 tint transform leaves
// the four inputs as the four outputs.
func TestDeviceNColor(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	tintNum := doc.object(streamBody(
		"/FunctionType 4 /Domain [0 1 0 1 0 1 0 1] /Range [0 1 0 1 0 1 0 1]",
		nil))
	file := mustOpen(t, doc.classic(""))

	names := []Value{NameVal("Cyan"), NameVal("Magenta"), NameVal("Yellow"), NameVal("Black")}
	space := deviceNSpace(names, NameVal(colorCMYK), RefVal(tintNum, 0))
	resolved := colorSpaceFor(t, file, space)
	checkComponents(t, resolved, cmykComponents)
	checkColorPreview(t, resolved, []float64{0, 0, 0, 0}, 1, 1, 1)
	checkColorPreview(t, resolved, []float64{0, 1, 0, 0}, 1, 0, 1)
	checkColorPreview(t, resolved, []float64{0, 0, 0, 1}, 0, 0, 0)

	checkSpaceUndefined(t, file, deviceNSpace(nil, NameVal(colorCMYK), RefVal(tintNum, 0)))
	checkSpaceUndefined(t, file, deviceNSpace(
		[]Value{IntVal(1)}, NameVal(colorCMYK), RefVal(tintNum, 0)))
	checkSpaceUndefined(t, file, deviceNSpace(names, NameVal(colorCMYK),
		DictVal(map[string]Value{keyFunctionType: IntVal(0)})))
}

// TestColorSpaceImageCMYK decodes a CMYK Flate image to the preview bytes.
func TestColorSpaceImageCMYK(t *testing.T) {
	t.Parallel()
	dict := "/Subtype /Image /Width 2 /Height 1 /ColorSpace /DeviceCMYK " +
		"/BitsPerComponent 8 /Filter /FlateDecode"
	raw := []byte{0, 255, 255, 0, 0, 0, 0, 255}
	file, num := oneImageDoc(t, dict, flateRaw(t, raw))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, pic, []byte{255, 0, 0, 0, 0, 0})
}

// TestColorSpaceImageCMYKDCTPreview converts a CMYK decode result through the
// preview seam. Go's image/jpeg encoder writes no CMYK stream, so a real CMYK
// JPEG is not built here; the decoder itself returns *image.CMYK.
func TestColorSpaceImageCMYKDCTPreview(t *testing.T) {
	t.Parallel()
	pic := image.NewCMYK(image.Rect(0, 0, 2, 1))
	pic.SetCMYK(0, 0, color.CMYK{C: 0, M: 255, Y: 255, K: 0})
	pic.SetCMYK(1, 0, color.CMYK{C: 0, M: 0, Y: 0, K: 255})
	space := colorSpaceFor(t, nil, NameVal(colorCMYK))
	got, err := previewDecoded(pic, space, nil, opImage)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, got, []byte{255, 0, 0, 0, 0, 0})

	rgb := image.NewRGBA(image.Rect(0, 0, 1, 1))
	_, err = previewDecoded(rgb, space, nil, opImage)
	wantErr(t, err, opImage, errUndefined)
}

// TestColorSpaceImageIndexed decodes an Indexed Flate image to the table RGB.
func TestColorSpaceImageIndexed(t *testing.T) {
	t.Parallel()
	dict := "/Subtype /Image /Width 2 /Height 1 " +
		"/ColorSpace [/Indexed /DeviceRGB 1 <FF0000 00FF00>] " +
		"/BitsPerComponent 8 /Filter /FlateDecode"
	file, num := oneImageDoc(t, dict, flateRaw(t, []byte{0, 1}))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, pic, []byte{255, 0, 0, 0, 255, 0})
}

// TestColorSpaceImageSeparation decodes a Separation Flate image through its
// tint transform and the DeviceCMYK preview.
func TestColorSpaceImageSeparation(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	dict := "/Subtype /Image /Width 1 /Height 1 " +
		"/ColorSpace [/Separation /Spot /DeviceCMYK 4 0 R] " +
		"/BitsPerComponent 8 /Filter /FlateDecode"
	num := doc.object(streamBody(dict, flateRaw(t, []byte{128})))
	doc.object(streamBody(
		"/FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [1 1 1 0] /N 1", nil))
	file := mustOpen(t, doc.classic(""))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, pic, []byte{127, 127, 127})
}

// TestColorSpaceImageOpName keeps the caller's operator name on an
// unsupported image color space.
func TestColorSpaceImageOpName(t *testing.T) {
	t.Parallel()
	dict := "/Subtype /Image /Width 1 /Height 1 /ColorSpace /Pattern " +
		"/BitsPerComponent 8 /Filter /FlateDecode"
	file, num := oneImageDoc(t, dict, []byte("x"))
	_, err := file.DecodeImage(num)
	wantErr(t, err, opImage, errUndefined)
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	_, err = file.DecodeImageValueOp(val, opDo)
	wantErr(t, err, opDo, errUndefined)
}

// TestTintTransformCalculatorStack runs the stack operators through the
// evaluator directly, because a tint transform leaves exactly one value per
// alternate component and this program leaves seven.
func TestTintTransformCalculatorStack(t *testing.T) {
	t.Parallel()
	program, err := parseCalculator([]byte("0.1 0.2 0.3 3 1 roll 3 copy 4 index pop"))
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]float64, 7)
	if !evalCalculator(program, []float64{0.5}, []float64{0, 1}, dst, flatRange(7)) {
		t.Fatal("eval failed")
	}
	want := []float64{0.5, 0.3, 0.1, 0.2, 0.3, 0.1, 0.2}
	for index, value := range want {
		if !nearPreview(dst[index], value) {
			t.Fatalf("stack[%d] = %v want %v (all %v)", index, dst[index], value, dst)
		}
	}
}

// TestTintTransformCalculatorMath runs one operator per case through the
// evaluator. Each program leaves one number and takes no input.
func TestTintTransformCalculatorMath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code string
		want float64
	}{
		{code: "3 4 add", want: 7},
		{code: "3 4 sub", want: -1},
		{code: "3 4 mul", want: 12},
		{code: "3 4 div", want: 0.75},
		{code: "7 2 idiv", want: 3},
		{code: "7 2 mod", want: 1},
		{code: "-3 abs", want: 3},
		{code: "2.3 ceiling", want: 3},
		{code: "2.7 floor", want: 2},
		{code: "2.5 round", want: 3},
		{code: "-2.7 truncate", want: -2},
		{code: "9 sqrt", want: 3},
		{code: "0 sin", want: 0},
		{code: "0 cos", want: 1},
		{code: "1 0 atan", want: 90},
		{code: "2 3 exp", want: 8},
		{code: "100 log", want: 2},
		{code: "1 ln", want: 0},
		{code: "5 2 bitshift", want: 20},
		{code: "1 1 eq { 7 } { 8 } ifelse", want: 7},
		{code: "true false and { 0 } { 1 } ifelse", want: 1},
		{code: "false not { 0 } { 1 } ifelse", want: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.code, func(t *testing.T) {
			t.Parallel()
			program, err := parseCalculator([]byte(testCase.code))
			if err != nil {
				t.Fatal(err)
			}
			dst := make([]float64, 1)
			if !evalCalculator(program, nil, nil, dst, []float64{-1000, 1000}) {
				t.Fatal("eval failed")
			}
			if !nearPreview(dst[0], testCase.want) {
				t.Fatalf("result %v want %v", dst[0], testCase.want)
			}
		})
	}
}

// flatRange returns count output ranges of 0 through 1.
func flatRange(count int) []float64 {
	limits := make([]float64, domainPair*count)
	for index := range count {
		limits[domainPair*index] = 0
		limits[domainPair*index+1] = 1
	}
	return limits
}

// colorSpaceFor resolves one space value or fails the test.
func colorSpaceFor(t *testing.T, file *File, space Value) colorSpace {
	t.Helper()
	resolved, err := file.resolveColorSpace(space, opImage)
	if err != nil {
		t.Fatalf("resolve color space: %v", err)
	}
	return resolved
}

// checkSpaceUndefined requires an undefined error in the Image operator.
func checkSpaceUndefined(t *testing.T, file *File, space Value) {
	t.Helper()
	_, err := file.resolveColorSpace(space, opImage)
	wantErr(t, err, opImage, errUndefined)
}

func checkComponents(t *testing.T, space colorSpace, want int) {
	t.Helper()
	if space.components != want {
		t.Fatalf("components %d want %d", space.components, want)
	}
}

func checkColorPreview(
	t *testing.T,
	space colorSpace,
	values []float64,
	wantRed, wantGreen, wantBlue float64,
) {
	t.Helper()
	red, green, blue := space.rgb(values)
	if !nearPreview(red, wantRed) || !nearPreview(green, wantGreen) || !nearPreview(blue, wantBlue) {
		t.Fatalf("preview %v = %v,%v,%v want %v,%v,%v",
			values, red, green, blue, wantRed, wantGreen, wantBlue)
	}
}

func nearPreview(got, want float64) bool {
	return math.Abs(got-want) <= previewTolerance
}

func colorNumberArray(values ...float64) Value {
	items := make([]Value, len(values))
	for index, value := range values {
		items[index] = RealVal(value)
	}
	return ArrayVal(items)
}

func separationSpace(alternate, tint Value) Value {
	return ArrayVal([]Value{
		NameVal(colorSeparation), NameVal("Spot"), alternate, tint,
	})
}

func deviceNSpace(names []Value, alternate, tint Value) Value {
	return ArrayVal([]Value{
		NameVal(colorDeviceN), ArrayVal(names), alternate, tint,
	})
}

// calculatorValue builds one direct type 4 stream. Range fixes the output
// count at four, so the caller's code must leave four numbers on the stack.
func calculatorValue(inputs int, code string) Value {
	domain := make([]float64, 0, inputs*2)
	for range inputs {
		domain = append(domain, 0, 1)
	}
	return StreamVal(map[string]Value{
		keyFunctionType: IntVal(funcTypeCalculator),
		keyDomain:       colorNumberArray(domain...),
		keyRange:        colorNumberArray(0, 1, 0, 1, 0, 1, 0, 1),
	}, []byte(code))
}
