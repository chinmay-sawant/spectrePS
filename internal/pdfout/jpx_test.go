package pdfout

import (
	"os"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	jpxFilterName = "JPXDecode"
	jpxLevelWidth = 8
)

// TestLevelJPXImage proves levels 2 to 5 treat a JPEG2000 stream the way they
// treat DCT: level 2 leaves the compressed stream alone, levels 3 to 5 decode
// it and write DCT, and a stream the decoder rejects copies through.
func TestLevelJPXImage(t *testing.T) {
	t.Parallel()
	checkJPXLevelDecode(t)
	checkJPXLevelCopy(t)
}

func checkJPXLevelDecode(t *testing.T) {
	t.Helper()
	checkJPXLevelRGB(t)
	checkJPXLevelGray(t)
}

func checkJPXLevelRGB(t *testing.T) {
	t.Helper()
	dict := "/Type /XObject /Subtype /Image /Width 8 /Height 8 " +
		"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /JPXDecode"
	file := levelImagePDF(t, levelStreamBody(dict, jpxLevelFixture(t, "../pdf/testdata/jpx-rgb.j2k")))

	// Level 2 re-encodes Flate and raw streams only, so the JPX stream stays.
	levelTwo, err := LevelOverrides(t.Context(), file, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := levelTwo[levelImageNum]; ok {
		t.Fatal("level 2 re-encoded the JPX image")
	}

	for _, level := range []int{3, 4, 5} {
		reopened := jpxLevelOverride(t, file, level)
		pic, err := reopened.DecodeImage(levelImageNum)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if pic.Bounds().Dx() != jpxLevelWidth || pic.Bounds().Dy() != jpxLevelWidth {
			t.Fatalf("level %d: bounds %v", level, pic.Bounds())
		}
		checkLevelFilter(t, reopened, levelImageNum, filterDCT)
		checkJPXLevelSpace(t, reopened, levelImageNum, spaceDeviceRGB)
	}
}

func checkJPXLevelGray(t *testing.T) {
	t.Helper()
	dict := "/Type /XObject /Subtype /Image /Width 8 /Height 8 " +
		"/ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /JPXDecode"
	file := levelImagePDF(t, levelStreamBody(dict, jpxLevelFixture(t, "../pdf/testdata/jpx-gray.jp2")))
	for _, level := range []int{3, 4, 5} {
		reopened := jpxLevelOverride(t, file, level)
		checkLevelFilter(t, reopened, levelImageNum, filterDCT)
		checkJPXLevelSpace(t, reopened, levelImageNum, spaceDeviceGray)
	}
}

// checkJPXLevelCopy proves a stream the decoder cannot read copies through at
// every level, the documented pass-through policy.
func checkJPXLevelCopy(t *testing.T) {
	t.Helper()
	dict := "/Type /XObject /Subtype /Image /Width 8 /Height 8 " +
		"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /JPXDecode"
	file := levelImagePDF(t,
		levelStreamBody(dict, []byte("not a JPEG2000 stream")),
		levelStreamBody(dict, jpxLevelFixture(t, "../pdf/testdata/jpx-gray.jp2")[:64]),
	)
	for level := 2; level <= 5; level++ {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		for _, num := range []int{levelImageNum, levelImageNum + 1} {
			if _, ok := overrides[num]; ok {
				t.Fatalf("level %d: undecodable JPX image %d has an override", level, num)
			}
		}
	}
}

func jpxLevelOverride(t *testing.T, file *pdf.File, level int) *pdf.File {
	t.Helper()
	overrides, err := LevelOverrides(t.Context(), file, level)
	if err != nil {
		t.Fatalf("level %d: %v", level, err)
	}
	if _, ok := overrides[levelImageNum]; !ok {
		t.Fatalf("level %d: JPX image has no override", level)
	}
	return mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
}

func checkJPXLevelSpace(t *testing.T, file *pdf.File, num int, want string) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	name, found := val.NameEntry(keyColorSpace)
	if !found || name != want {
		t.Fatalf("object %d ColorSpace = %q found %v, want %q", num, name, found, want)
	}
}

func jpxLevelFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
