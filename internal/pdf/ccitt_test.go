package pdf

import (
	"image"
	"os"
	"testing"
)

// The two CCITT fixtures are raw strip bytes, not TIFF containers. Both
// encode the same 8 by 3 bilevel image: row 0 all black, row 1 all white,
// row 2 alternating black and white. With /BlackIs1 false they decode to
// that pattern.
//
//	testdata/ccitt-g4.bin  Group 4, 14 bytes
//	                       SHA-256 b9ea02cafa44593fb4b3b56e159e00c98a533b69f5176ae5ce03b8205f9f0274
//	testdata/ccitt-g3.bin  Group 3, 13 bytes
//	                       SHA-256 1e41f54b7b17df07b8316c7c06b3902908591cab0f6f2b7e008370cd3d1b9743
//
// Regenerate with Pillow 12.3.0:
//
//	python3 internal/pdf/testdata/gen_ccitt.py internal/pdf/testdata
//
// A TIFF strip carries the fax codes with the opposite polarity to a PDF
// stream, so the generator feeds Pillow the inverse image.

const (
	ccittWidth  = 8
	ccittHeight = 3
)

func TestImageXObjectCCITT(t *testing.T) {
	t.Parallel()
	raw := ccittFixture(t, "ccitt-g4.bin")
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode"
	cases := []struct {
		name string
		dict string
		want []byte
	}{
		{
			name: "group 4",
			dict: base + " /DecodeParms << /K -1 >>",
			want: ccittPixels(),
		},
		{
			name: "black is 1",
			dict: base + " /DecodeParms << /K -1 /BlackIs1 true >>",
			want: invertedPixels(),
		},
		{
			name: "columns and rows",
			dict: "/Subtype /Image /Width 16 /Height 6 /ColorSpace /DeviceGray " +
				"/BitsPerComponent 1 /Filter /CCITTFaxDecode " +
				"/DecodeParms << /K -1 /Columns 8 /Rows 3 >>",
			want: ccittPixels(),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, testCase.dict, raw)
			pic, err := file.DecodeImage(num)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := pic.(*image.Gray); !ok {
				t.Fatalf("type %T, want *image.Gray", pic)
			}
			if pic.Bounds().Dx() != ccittWidth || pic.Bounds().Dy() != ccittHeight {
				t.Fatalf("bounds %v", pic.Bounds())
			}
			checkGrayPixels(t, pic, testCase.want)
		})
	}
}

func TestImageXObjectCCITTG3(t *testing.T) {
	t.Parallel()
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode " +
		"/DecodeParms << /K 0 /EndOfLine true"
	cases := []struct {
		name string
		dict string
		raw  []byte
	}{
		{
			name: "end of line",
			dict: base + " >>",
			raw:  ccittFixture(t, "ccitt-g3.bin"),
		},
		{
			name: "byte aligned",
			dict: base + " /EncodedByteAlign true >>",
			raw:  alignedG3(),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, testCase.dict, testCase.raw)
			pic, err := file.DecodeImage(num)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := pic.(*image.Gray); !ok {
				t.Fatalf("type %T, want *image.Gray", pic)
			}
			if pic.Bounds().Dx() != ccittWidth || pic.Bounds().Dy() != ccittHeight {
				t.Fatalf("bounds %v", pic.Bounds())
			}
			checkGrayPixels(t, pic, ccittPixels())
		})
	}
}

func TestImageXObjectCCITTRejects(t *testing.T) {
	t.Parallel()
	raw := ccittFixture(t, "ccitt-g4.bin")
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode"
	cases := []struct {
		name    string
		dict    string
		raw     []byte
		errName string
	}{
		{
			name:    "K above zero",
			dict:    base + " /DecodeParms << /K 1 >>",
			raw:     raw,
			errName: errUndefined,
		},
		{
			name:    "missing end of line",
			dict:    base + " /DecodeParms << /K 0 >>",
			raw:     raw,
			errName: errUndefined,
		},
		{
			name: "bit depth",
			dict: "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
				"/BitsPerComponent 8 /Filter /CCITTFaxDecode /DecodeParms << /K -1 >>",
			raw:     raw,
			errName: errUndefined,
		},
		{
			name: "color space",
			dict: "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceRGB " +
				"/BitsPerComponent 1 /Filter /CCITTFaxDecode /DecodeParms << /K -1 >>",
			raw:     raw,
			errName: errUndefined,
		},
		{
			name:    "missing decode parms",
			dict:    base,
			raw:     raw,
			errName: errUndefined,
		},
		{
			name:    "truncated",
			dict:    base + " /DecodeParms << /K -1 >>",
			raw:     raw[:3],
			errName: errSyntax,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, testCase.dict, testCase.raw)
			pic, err := file.DecodeImage(num)
			wantErr(t, err, opImage, testCase.errName)
			if pic != nil {
				t.Fatalf("decode returned an image with %v", err)
			}
		})
	}
}

func TestImageXObjectCCITTLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		dict string
	}{
		{
			name: "width and height",
			dict: "/Subtype /Image /Width 8193 /Height 4096 /ColorSpace /DeviceGray " +
				"/BitsPerComponent 1 /Filter /CCITTFaxDecode /DecodeParms << /K -1 >>",
		},
		{
			name: "columns and rows",
			dict: "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
				"/BitsPerComponent 1 /Filter /CCITTFaxDecode " +
				"/DecodeParms << /K -1 /Columns 8193 /Rows 4096 >>",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, testCase.dict, []byte{0})
			pic, err := file.DecodeImage(num)
			wantErr(t, err, opImage, errLimit)
			if pic != nil {
				t.Fatalf("decode returned an image with %v", err)
			}
		})
	}
}

func ccittFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// ccittPixels is the 8 by 3 pattern both fixtures decode to.
func ccittPixels() []byte {
	return []byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		255, 255, 255, 255, 255, 255, 255, 255,
		0, 255, 0, 255, 0, 255, 0, 255,
	}
}

func invertedPixels() []byte {
	out := ccittPixels()
	for i := range out {
		out[i] = 255 - out[i]
	}
	return out
}

// alignedG3 builds the fixture pattern as an /EncodedByteAlign true Group 3
// stream. The run codes are ITU-T T.4: white 0 is 00110101, white 1 is
// 000111, white 8 is 10011, black 1 is 010, and black 8 is 000101. The final
// end-of-line is left off, which the decoder accepts as a truncated stream.
func alignedG3() []byte {
	var pack bitPacker
	pack.code("000000000001")
	pack.align()
	pack.code("00110101")
	pack.code("000101")
	pack.code("000000000001")
	pack.align()
	pack.code("10011")
	pack.code("000000000001")
	pack.align()
	pack.code("00110101")
	for range 4 {
		pack.code("010")
		pack.code("000111")
	}
	return pack.bytes()
}

// bitPacker writes most significant bit first codes for a test stream.
type bitPacker struct {
	bits []byte
}

func (pack *bitPacker) code(text string) {
	for i := range len(text) {
		pack.bits = append(pack.bits, text[i]-'0')
	}
}

// align pads with zeros to the next byte boundary.
func (pack *bitPacker) align() {
	for len(pack.bits)%8 != 0 {
		pack.bits = append(pack.bits, 0)
	}
}

func (pack *bitPacker) bytes() []byte {
	out := make([]byte, (len(pack.bits)+7)/8)
	for i, bit := range pack.bits {
		if bit == 1 {
			out[i/8] |= 1 << (7 - i%8)
		}
	}
	return out
}
