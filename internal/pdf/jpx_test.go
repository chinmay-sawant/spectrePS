package pdf

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"testing"
)

const (
	jpxRGBPath  = "testdata/jpx-rgb.j2k"
	jpxGrayPath = "testdata/jpx-gray.jp2"
	jpxSide     = 8

	jpxLimitSide = 60000
)

func TestImageXObjectJPX(t *testing.T) {
	t.Parallel()
	t.Run("rgb codestream", func(t *testing.T) {
		t.Parallel()
		checkJPXRGB(t)
	})
	t.Run("gray container", func(t *testing.T) {
		t.Parallel()
		checkJPXGray(t)
	})
	t.Run("bare dictionary", func(t *testing.T) {
		t.Parallel()
		checkJPXBare(t)
	})
	t.Run("filter array", func(t *testing.T) {
		t.Parallel()
		checkJPXFilterArray(t)
	})
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		checkJPXMalformed(t)
	})
	t.Run("size limit", func(t *testing.T) {
		t.Parallel()
		checkJPXLimit(t)
	})
}

// jpxFixture reads one checked-in fixture. See testdata/README.md for the
// generation command and the SHA-256 of each file.
func jpxFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkJPXRGB(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 8 /Height 8 /BitsPerComponent 8 " +
		"/ColorSpace /DeviceRGB /Filter /JPXDecode"
	file, num := oneImageDoc(t, dict, jpxFixture(t, jpxRGBPath))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkJPXBounds(t, pic)
	for posY := range jpxSide {
		for posX := range jpxSide {
			want := color.RGBA{
				R: byte(posX * 32),
				G: byte(posY * 32),
				B: byte((posX + posY) * 16),
				A: opaqueAlpha,
			}
			checkExactColor(t, pic, posX, posY, want)
		}
	}
}

func checkJPXGray(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 8 /Height 8 /BitsPerComponent 8 " +
		"/ColorSpace /DeviceGray /Filter /JPXDecode"
	file, num := oneImageDoc(t, dict, jpxFixture(t, jpxGrayPath))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkJPXBounds(t, pic)
	if _, ok := pic.(*image.Gray); !ok {
		t.Fatalf("gray decode returned %T", pic)
	}
	for posY := range jpxSide {
		for posX := range jpxSide {
			level := byte((posY*32 + posX*4) % 256)
			want := color.RGBA{R: level, G: level, B: level, A: opaqueAlpha}
			checkExactColor(t, pic, posX, posY, want)
		}
	}
}

// checkJPXBare proves the branch runs before the /BitsPerComponent and
// /ColorSpace checks: the dictionary carries neither entry.
func checkJPXBare(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 8 /Height 8 /Filter /JPXDecode"
	file, num := oneImageDoc(t, dict, jpxFixture(t, jpxGrayPath))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkJPXBounds(t, pic)
}

func checkJPXFilterArray(t *testing.T) {
	t.Helper()
	dict := "/Subtype /Image /Width 8 /Height 8 /BitsPerComponent 8 " +
		"/ColorSpace /DeviceRGB /Filter [/JPXDecode]"
	file, num := oneImageDoc(t, dict, jpxFixture(t, jpxRGBPath))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkJPXBounds(t, pic)
}

func checkJPXMalformed(t *testing.T) {
	t.Helper()
	rgb := jpxFixture(t, jpxRGBPath)
	cases := []struct {
		name string
		raw  []byte
	}{
		{name: "not a codestream", raw: []byte("not a JPEG2000 stream")},
		{name: "truncated", raw: rgb[:len(rgb)/2]},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dict := "/Subtype /Image /Width 8 /Height 8 /BitsPerComponent 8 " +
				"/ColorSpace /DeviceRGB /Filter /JPXDecode"
			file, num := oneImageDoc(t, dict, testCase.raw)
			pic, err := file.DecodeImage(num)
			wantErr(t, err, opImage, errSyntax)
			if pic != nil {
				t.Fatalf("failed decode returned %T", pic)
			}
		})
	}
}

// checkJPXLimit rewrites the SIZ size fields of the raw codestream so the
// header declares a 60000 by 60000 image. That is far past the 32 MiB cap, so
// the branch returns limitcheck before the decoder allocates.
func checkJPXLimit(t *testing.T) {
	t.Helper()
	raw := jpxFixture(t, jpxRGBPath)
	if !bytes.HasPrefix(raw, []byte{0xFF, 0x4F, 0xFF, 0x51}) {
		t.Fatal("fixture does not start with SOC and SIZ")
	}
	for _, offset := range []int{8, 12, 24, 28} {
		putJPXInt(t, raw, offset, jpxLimitSide)
	}
	dict := "/Subtype /Image /Width 8 /Height 8 /Filter /JPXDecode"
	file, num := oneImageDoc(t, dict, raw)
	pic, err := file.DecodeImage(num)
	wantErr(t, err, opImage, errLimit)
	if pic != nil {
		t.Fatalf("over-cap decode returned %T", pic)
	}
}

func putJPXInt(t *testing.T, raw []byte, offset, value int) {
	t.Helper()
	if offset < 0 || offset+4 > len(raw) {
		t.Fatalf("SIZ field at %d is outside %d bytes", offset, len(raw))
	}
	raw[offset] = byte(value >> 24)
	raw[offset+1] = byte(value >> 16)
	raw[offset+2] = byte(value >> 8)
	raw[offset+3] = byte(value)
}

func checkJPXBounds(t *testing.T, pic image.Image) {
	t.Helper()
	if pic.Bounds().Dx() != jpxSide || pic.Bounds().Dy() != jpxSide {
		t.Fatalf("bounds %v, want %dx%d", pic.Bounds(), jpxSide, jpxSide)
	}
}
