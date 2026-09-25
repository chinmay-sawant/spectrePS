package pdfout

import (
	"bytes"
	"image"
	"os"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	levelCCITTNum    = 4
	levelBadCCITTNum = 5
)

// TestLevelCCITTImage proves levels 1 through 5 treat a CCITT stream like any
// other decoded image. Level 2 re-encodes it as lossless Flate RGB, levels 3
// through 5 as DCT, and an undecodable stream copies through at every level.
func TestLevelCCITTImage(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../sampledata/fixtures/ccitt-g4.bin")
	if err != nil {
		t.Fatal(err)
	}
	dict := "/Type /XObject /Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode /DecodeParms << /K -1 >>"
	badDict := "/Type /XObject /Subtype /Image /Width 8 /Height 3 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 1 /Filter /CCITTFaxDecode /DecodeParms << /K 1 >>"
	file := levelImagePDF(t, levelStreamBody(dict, raw), levelStreamBody(badDict, raw))
	checkCCITTLevelOne(t, file)
	checkCCITTFlateLevel(t, file)
	checkCCITTDCTLevels(t, file)
}

func checkCCITTLevelOne(t *testing.T, file *pdf.File) {
	t.Helper()
	overrides, err := LevelOverrides(t.Context(), file, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Fatalf("level 1 overrides %v", overrides)
	}
}

func checkCCITTFlateLevel(t *testing.T, file *pdf.File) {
	t.Helper()
	overrides, err := LevelOverrides(t.Context(), file, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := overrides[levelCCITTNum]; !ok {
		t.Fatal("level 2 skipped the CCITT image")
	}
	if _, ok := overrides[levelBadCCITTNum]; ok {
		t.Fatal("level 2 changed an undecodable CCITT image")
	}
	reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
	pic, err := reopened.DecodeImage(levelCCITTNum)
	if err != nil {
		t.Fatal(err)
	}
	checkLevelFilter(t, reopened, levelCCITTNum, filterFlate)
	checkCCITTPixels(t, pic, ccittLevelPixels())
	checkLevelFilter(t, reopened, levelBadCCITTNum, filterCCITT)
}

func checkCCITTDCTLevels(t *testing.T, file *pdf.File) {
	t.Helper()
	for _, level := range []int{levelMedium, levelStrong, levelHard} {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if _, ok := overrides[levelCCITTNum]; !ok {
			t.Fatalf("level %d: CCITT image has no override", level)
		}
		if _, ok := overrides[levelBadCCITTNum]; ok {
			t.Fatalf("level %d: undecodable CCITT image has an override", level)
		}
		reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
		pic, err := reopened.DecodeImage(levelCCITTNum)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		bounds := pic.Bounds()
		if bounds.Dx() != 8 || bounds.Dy() != 3 {
			t.Fatalf("level %d: bounds %v", level, bounds)
		}
		checkLevelFilter(t, reopened, levelCCITTNum, filterDCT)
		checkLevelFilter(t, reopened, levelBadCCITTNum, filterCCITT)
	}
}

// ccittLevelPixels is the decoded 8 by 3 pattern widened to RGB.
func ccittLevelPixels() []byte {
	gray := []byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		255, 255, 255, 255, 255, 255, 255, 255,
		0, 255, 0, 255, 0, 255, 0, 255,
	}
	out := make([]byte, 0, len(gray)*imageChannels)
	for _, sample := range gray {
		out = append(out, sample, sample, sample)
	}
	return out
}

func checkCCITTPixels(t *testing.T, pic image.Image, want []byte) {
	t.Helper()
	if pic.Bounds().Dx() != 8 || pic.Bounds().Dy() != 3 {
		t.Fatalf("bounds %v", pic.Bounds())
	}
	for idx := range len(want) / imageChannels {
		x := idx % 8
		y := idx / 8
		red, green, blue, _ := pic.At(x, y).RGBA()
		got := []byte{byte(red >> colorByteShift), byte(green >> colorByteShift), byte(blue >> colorByteShift)}
		if !bytes.Equal(got, want[idx*imageChannels:(idx+1)*imageChannels]) {
			t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want[idx*imageChannels:(idx+1)*imageChannels])
		}
	}
}
