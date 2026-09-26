package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"slices"
	"testing"
)

const (
	imageSide   = 8
	jpegSlack   = 3
	jpegQuality = 95

	flatRed   = 200
	flatGreen = 100
	flatBlue  = 50
)

type imageFixture struct {
	src     []byte
	rgbNum  int
	grayNum int
	dctNum  int
}

func newImageFixture(t *testing.T) imageFixture {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	rgbNum := doc.object(imageStream(t, 2, 2, "/DeviceRGB", "/FlateDecode", flateRaw(t, rgbPixels())))
	grayNum := doc.object(imageStream(t, 2, 2, "/DeviceGray", "/FlateDecode", flateRaw(t, grayPixels())))
	dctNum := doc.object(imageStream(t, imageSide, imageSide, "/DeviceRGB", "/DCTDecode", jpegBytes(t)))
	return imageFixture{src: doc.classic(""), rgbNum: rgbNum, grayNum: grayNum, dctNum: dctNum}
}

func imageStream(t *testing.T, width, height int, space, filter string, raw []byte) string {
	t.Helper()
	dict := "/Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter %s"
	return streamBody(fmt.Sprintf(dict, width, height, space, filter), raw)
}

func oneImageDoc(t *testing.T, dict string, raw []byte) (*File, int) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	num := doc.object(streamBody(dict, raw))
	return mustOpen(t, doc.classic("")), num
}

func TestImageXObjectNums(t *testing.T) {
	t.Parallel()
	fixture := newImageFixture(t)
	file := mustOpen(t, fixture.src)
	nums, err := file.ImageObjectNums()
	if err != nil {
		t.Fatal(err)
	}
	want := []int{fixture.rgbNum, fixture.grayNum, fixture.dctNum}
	if !slices.Equal(nums, want) {
		t.Fatalf("nums %v want %v", nums, want)
	}
}

func TestImageXObjectFlateRGB(t *testing.T) {
	t.Parallel()
	fixture := newImageFixture(t)
	file := mustOpen(t, fixture.src)
	pic, err := file.DecodeImage(fixture.rgbNum)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Dx() != 2 || pic.Bounds().Dy() != 2 {
		t.Fatalf("bounds %v", pic.Bounds())
	}
	checkRGBPixels(t, pic, rgbPixels())
}

func TestImageXObjectFlateGray(t *testing.T) {
	t.Parallel()
	fixture := newImageFixture(t)
	file := mustOpen(t, fixture.src)
	pic, err := file.DecodeImage(fixture.grayNum)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Dx() != 2 || pic.Bounds().Dy() != 2 {
		t.Fatalf("bounds %v", pic.Bounds())
	}
	checkGrayPixels(t, pic, grayPixels())
}

func TestImageXObjectDCT(t *testing.T) {
	t.Parallel()
	fixture := newImageFixture(t)
	file := mustOpen(t, fixture.src)
	pic, err := file.DecodeImage(fixture.dctNum)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Dx() != imageSide || pic.Bounds().Dy() != imageSide {
		t.Fatalf("bounds %v", pic.Bounds())
	}
	checkNearColor(t, pic, color.RGBA{R: flatRed, G: flatGreen, B: flatBlue, A: opaqueAlpha})
}

func TestImageXObjectUnknownFilter(t *testing.T) {
	t.Parallel()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /LZWDecode"
	file, num := oneImageDoc(t, dict, rgbPixels())
	_, err := file.DecodeImage(num)
	wantErr(t, err, "LZWDecode", errUndefined)
}

// TestImageXObjectFilterChain decodes a two-name chain: the leading
// ASCIIHexDecode unwraps a DCT body, the final stage decodes the JPEG.
func TestImageXObjectFilterChain(t *testing.T) {
	t.Parallel()
	dict := "/Subtype /Image /Width 8 /Height 8 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter [/ASCIIHexDecode /DCTDecode]"
	file, num := oneImageDoc(t, dict, asciiHexBytes(jpegBytes(t)))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Dx() != imageSide || pic.Bounds().Dy() != imageSide {
		t.Fatalf("bounds %v", pic.Bounds())
	}
	checkNearColor(t, pic, color.RGBA{R: flatRed, G: flatGreen, B: flatBlue, A: opaqueAlpha})
}

// asciiHexBytes encodes raw as an ASCIIHexDecode body with the EOD marker.
func asciiHexBytes(raw []byte) []byte {
	const hexDigits = "0123456789ABCDEF"
	out := make([]byte, 0, len(raw)*2+1)
	for _, value := range raw {
		out = append(out, hexDigits[value>>hexShift], hexDigits[value&0x0F])
	}
	return append(out, '>')
}

func TestImageXObjectUnsupported(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		dict string
		raw  []byte
		op   string
	}{
		{
			name: "bit depth",
			dict: "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
				"/BitsPerComponent 4",
			raw: []byte("x"),
			op:  opImage,
		},
		{
			name: "color space",
			dict: "/Subtype /Image /Width 2 /Height 2 /ColorSpace /Pattern " +
				"/BitsPerComponent 8 /Filter /FlateDecode",
			raw: []byte("x"),
			op:  opImage,
		},
		{
			name: "shape",
			dict: "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
				"/BitsPerComponent 8 /Filter /FlateDecode",
			raw: flateRaw(t, []byte{0, 0, 0}),
			op:  opImage,
		},
		{
			name: "predictor",
			dict: "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
				"/BitsPerComponent 8 /Filter /FlateDecode " +
				"/DecodeParms << /Predictor 3 >>",
			raw: flateRaw(t, rgbPixels()),
			op:  opPredictor,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, tc.dict, tc.raw)
			_, err := file.DecodeImage(num)
			wantErr(t, err, tc.op, errUndefined)
		})
	}
}

func rgbPixels() []byte {
	return []byte{
		255, 0, 0, 0, 255, 0,
		0, 0, 255, 255, 255, 255,
	}
}

func grayPixels() []byte {
	return []byte{0, 85, 170, 255}
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	pic := image.NewRGBA(image.Rect(0, 0, imageSide, imageSide))
	flat := color.RGBA{R: flatRed, G: flatGreen, B: flatBlue, A: opaqueAlpha}
	for y := range imageSide {
		for x := range imageSide {
			pic.SetRGBA(x, y, flat)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, pic, &jpeg.Options{Quality: jpegQuality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func checkRGBPixels(t *testing.T, pic image.Image, want []byte) {
	t.Helper()
	width := pic.Bounds().Dx()
	for posY := range pic.Bounds().Dy() {
		for posX := range width {
			at := (posY*width + posX) * rgbComponents
			checkExactColor(t, pic, posX, posY, color.RGBA{
				R: want[at], G: want[at+1], B: want[at+2], A: opaqueAlpha,
			})
		}
	}
}

func checkGrayPixels(t *testing.T, pic image.Image, want []byte) {
	t.Helper()
	width := pic.Bounds().Dx()
	for posY := range pic.Bounds().Dy() {
		for posX := range width {
			gray := want[posY*width+posX]
			checkExactColor(t, pic, posX, posY, color.RGBA{R: gray, G: gray, B: gray, A: opaqueAlpha})
		}
	}
}

func checkExactColor(t *testing.T, pic image.Image, posX, posY int, want color.RGBA) {
	t.Helper()
	got, ok := color.RGBAModel.Convert(pic.At(posX, posY)).(color.RGBA)
	if !ok {
		t.Fatalf("pixel %d,%d has type %T", posX, posY, pic.At(posX, posY))
	}
	if got != want {
		t.Fatalf("pixel %d,%d got %v want %v", posX, posY, got, want)
	}
}

func checkNearColor(t *testing.T, pic image.Image, want color.RGBA) {
	t.Helper()
	bounds := pic.Bounds()
	for posY := range bounds.Dy() {
		for posX := range bounds.Dx() {
			got, ok := color.RGBAModel.Convert(pic.At(posX, posY)).(color.RGBA)
			if !ok {
				t.Fatalf("pixel %d,%d has type %T", posX, posY, pic.At(posX, posY))
			}
			if !nearByte(got.R, want.R) || !nearByte(got.G, want.G) ||
				!nearByte(got.B, want.B) || !nearByte(got.A, want.A) {
				t.Fatalf("jpeg pixel %d,%d got %v want %v", posX, posY, got, want)
			}
		}
	}
}

func nearByte(got, want uint8) bool {
	if got > want {
		return got-want <= jpegSlack
	}
	return want-got <= jpegSlack
}
