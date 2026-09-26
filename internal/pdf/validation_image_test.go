package pdf

import (
	"bytes"
	"image"
	"os"
	"testing"
)

// TestValidationImageErrorShapes locks the named errors one image decode
// returns: a malformed DCT body, an image filter chain with an unknown second
// name, a Flate image past the 32 MiB cap, a /DecodeParms value that is not a
// dictionary, and /Predictor 0. No case returns a blank image.
func TestValidationImageErrorShapes(t *testing.T) {
	t.Parallel()
	validationDCTError(t)
	validationImageChainError(t)
	validationFlateCap(t)
	validationImageParmsKind(t)
	validationPredictorZero(t)
}

func validationDCTError(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 8 /Height 8 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /DCTDecode"
	file, num := oneImageDoc(t, dict, []byte("this body is not a jpeg"))
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opImage, errSyntax)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

// validationImageChainError locks the shape an unknown stage in an image
// filter chain reports: undefined with that stage's filter name, the same
// contract the generic stream chain uses.
func validationImageChainError(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter [/FlateDecode /BogusDecode]"
	file, num := oneImageDoc(t, dict, flateRaw(t, rgbPixels()))
	pic, err := file.DecodeImage(num)
	wantErr(t, err, "BogusDecode", errUndefined)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

// validationFlateCap decodes a zlib body that expands past the 32 MiB cap
// without ever asking the image path for the pixels.
func validationFlateCap(t *testing.T) {
	t.Helper()
	plain := make([]byte, maxInflated+1)
	stored := flateRaw(t, plain)
	dict := "/Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 8 /Filter /FlateDecode"
	file, num := oneImageDoc(t, dict, stored)
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opFlate, errLimit)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

// validationImageParmsKind locks the shape a Flate /DecodeParms of the wrong
// kind reports: the predictor reader refuses it before any inflate.
func validationImageParmsKind(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode /DecodeParms 3"
	file, num := oneImageDoc(t, dict, flateRaw(t, rgbPixels()))
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opPredictor, errSyntax)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

func validationPredictorZero(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode /DecodeParms << /Predictor 0 >>"
	file, num := oneImageDoc(t, dict, flateRaw(t, rgbPixels()))
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opPredictor, errSyntax)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

// TestValidationFilterChain locks the generic stream chain: a one-name array
// with a /DecodeParms array decodes, the first unknown stage reports undefined
// with its own name, and an array item that is not a name is a PDF type error.
func TestValidationFilterChain(t *testing.T) {
	t.Parallel()
	plain := []byte("q 10 20 m 30 40 l S\n")
	validationChainOneName(t, plain)
	validationChainUnknownStage(t, plain)
	validationChainNonName(t, plain)
}

func validationChainOneName(t *testing.T, plain []byte) {
	t.Helper()
	val := chainStream([]string{opFlate}, ArrayVal([]Value{NullVal()}), flateRaw(t, plain))
	got, err := decodeStream(val)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("one-name chain got %q", got)
	}
}

func validationChainUnknownStage(t *testing.T, plain []byte) {
	t.Helper()
	parms := ArrayVal([]Value{NullVal(), NullVal()})
	val := chainStream([]string{opFlate, "BogusDecode"}, parms, flateRaw(t, plain))
	_, err := decodeStream(val)
	wantJob(t, err, "BogusDecode", errUndefined)
}

func validationChainNonName(t *testing.T, plain []byte) {
	t.Helper()
	dict := map[string]Value{"Filter": ArrayVal([]Value{NameVal(opFlate), IntVal(7)})}
	val := StreamVal(dict, flateRaw(t, plain))
	_, err := decodeStream(val)
	wantJob(t, err, opPDF, errType)
}

// TestValidationCCITTParams locks the /DecodeParms validation and the decode
// of every /K shape. A /DecodeParms value that is not a dictionary, a flag of
// the wrong kind, a non-positive /Columns or /Rows, and a real or name /K are
// undefined in Image. A Group 4 stream decodes with /EndOfBlock true and with
// /EndOfBlock false, and the Group 3 shapes decode with and without
// end-of-line markers.
func TestValidationCCITTParams(t *testing.T) {
	t.Parallel()
	validationCCITTParamErrors(t)
	validationCCITTEndOfBlock(t)
	validationCCITTGroup3NoEOL(t)
	validationCCITTTruncated(t)
	validationCCITTCorpusEndOfBlock(t)
}

func validationCCITTParamErrors(t *testing.T) {
	t.Helper()
	raw := ccittFixture(t, "ccitt-g4.bin")
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode"
	cases := []struct {
		name string
		dict string
	}{
		{"decode parms not a dict", base + " /DecodeParms 5"},
		{"end of line not bool", base + " /DecodeParms << /K -1 /EndOfLine 1 >>"},
		{"black is 1 not bool", base + " /DecodeParms << /K -1 /BlackIs1 /true >>"},
		{"byte align not bool", base + " /DecodeParms << /K -1 /EncodedByteAlign 1 >>"},
		{"end of block not bool", base + " /DecodeParms << /K -1 /EndOfBlock 1 >>"},
		{"columns zero", base + " /DecodeParms << /K -1 /Columns 0 >>"},
		{"columns negative", base + " /DecodeParms << /K -1 /Columns -8 >>"},
		{"rows zero", base + " /DecodeParms << /K -1 /Rows 0 >>"},
		{"k real", base + " /DecodeParms << /K 1.5 >>"},
		{"k name", base + " /DecodeParms << /K /Four >>"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			file, num := oneImageDoc(t, testCase.dict, raw)
			pic, err := file.DecodeImage(num)
			wantErr(t, err, opImage, errUndefined)
			if pic != nil {
				t.Fatalf("decode returned an image with %v", err)
			}
		})
	}
}

// validationCCITTEndOfBlock proves the same Group 4 fixture decodes with
// /EndOfBlock true and with /EndOfBlock false.
func validationCCITTEndOfBlock(t *testing.T) {
	t.Helper()
	raw := ccittFixture(t, "ccitt-g4.bin")
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode"
	for _, endOfBlock := range []string{"true", "false"} {
		t.Run("end of block "+endOfBlock, func(t *testing.T) {
			t.Parallel()
			dict := base + " /DecodeParms << /K -1 /EndOfBlock " + endOfBlock + " >>"
			file, num := oneImageDoc(t, dict, raw)
			pic, err := file.DecodeImage(num)
			if err != nil {
				t.Fatal(err)
			}
			checkGrayPixels(t, pic, ccittPixels())
		})
	}
}

// validationPlainG3 builds the fixture pattern as a Group 3 stream with no
// end-of-line markers, which /EndOfLine false selects. The codes are the same
// T.4 white and black codes alignedG3 uses: row 0 is white 0 then black 8,
// row 1 is white 8, and row 2 is white 0 then four black 1 and white 1 pairs.
func validationPlainG3() []byte {
	var pack bitPacker
	pack.code("00110101")
	pack.code("000101")
	pack.code("10011")
	pack.code("00110101")
	for range 4 {
		pack.code("010")
		pack.code("000111")
	}
	return pack.bytes()
}

// validationCCITTGroup3NoEOL proves the no-end-of-line Group 3 row loop
// decodes the known pattern with both /EndOfBlock values, and that the fixture
// with end-of-line markers still decodes through /EndOfLine true.
func validationCCITTGroup3NoEOL(t *testing.T) {
	t.Helper()
	base := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode"
	for _, endOfBlock := range []string{"true", "false"} {
		t.Run("group 3 no end of line end of block "+endOfBlock, func(t *testing.T) {
			t.Parallel()
			dict := base + " /DecodeParms << /K 0 /EndOfBlock " + endOfBlock + " >>"
			file, num := oneImageDoc(t, dict, validationPlainG3())
			pic, err := file.DecodeImage(num)
			if err != nil {
				t.Fatal(err)
			}
			checkGrayPixels(t, pic, ccittPixels())
		})
	}
	t.Run("group 3 end of line", func(t *testing.T) {
		t.Parallel()
		dict := base + " /DecodeParms << /K 0 /EndOfLine true >>"
		file, num := oneImageDoc(t, dict, ccittFixture(t, "ccitt-g3.bin"))
		pic, err := file.DecodeImage(num)
		if err != nil {
			t.Fatal(err)
		}
		checkGrayPixels(t, pic, ccittPixels())
	})
}

// validationCCITTTruncated proves a no-end-of-line Group 3 stream cut off
// inside a row reports syntaxerror rather than a partial image.
func validationCCITTTruncated(t *testing.T) {
	t.Helper()
	raw := validationPlainG3()
	if len(raw) < 6 {
		t.Fatalf("plain group 3 stream is %d bytes", len(raw))
	}
	dict := "/Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode " +
		"/DecodeParms << /K 0 /EndOfBlock false >>"
	file, num := oneImageDoc(t, dict, raw[:5])
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opImage, errSyntax)
	if pic != nil {
		t.Fatalf("decode returned an image with %v", err)
	}
}

// validationCCITTCorpusEndOfBlock decodes the six images of the committed
// corpus case, one pair per /K value, and proves the /EndOfBlock true and false
// bytes of each pair decode to the same pixels.
func validationCCITTCorpusEndOfBlock(t *testing.T) {
	t.Helper()
	src, err := os.ReadFile("../../sampledata/validation/images/ccitt_EndOfBlock_false.pdf")
	if err != nil {
		t.Fatal(err)
	}
	file := mustOpen(t, src)
	nums, err := file.ImageObjectNums()
	if err != nil {
		t.Fatal(err)
	}
	if len(nums) != 6 {
		t.Fatalf("corpus has %d images, want 6", len(nums))
	}
	for pair := range 3 {
		left := validationDecodeGray(t, file, nums[pair*2])
		right := validationDecodeGray(t, file, nums[pair*2+1])
		if !bytes.Equal(left.Pix, right.Pix) {
			t.Fatalf("images %d and %d differ", nums[pair*2], nums[pair*2+1])
		}
		if validationCountBlack(left) == 0 {
			t.Fatalf("image %d decoded blank", nums[pair*2])
		}
	}
}

func validationDecodeGray(t *testing.T, file *File, num int) *image.Gray {
	t.Helper()
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatalf("object %d: %v", num, err)
	}
	gray, ok := pic.(*image.Gray)
	if !ok {
		t.Fatalf("object %d is %T", num, pic)
	}
	if gray.Bounds().Dx() != 81 || gray.Bounds().Dy() != 26 {
		t.Fatalf("object %d bounds %v", num, gray.Bounds())
	}
	return gray
}

func validationCountBlack(pic *image.Gray) int {
	count := 0
	for _, value := range pic.Pix {
		if value == 0 {
			count++
		}
	}
	return count
}
