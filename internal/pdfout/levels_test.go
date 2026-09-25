package pdfout

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	levelContentText = "BT /F1 12 Tf 5 5 Td (Hi) Tj ET"
	levelFontNum     = 6
	levelImageNum    = 4
	levelSMaskNum    = 5
)

// TestLevelContentCopy proves the pass-through writer keeps text, both content
// streams, and the font at every level.
func TestLevelContentCopy(t *testing.T) {
	t.Parallel()
	file := levelTextPDF(t)
	want := levelContentText + "\n" + lineMarks
	for level := MinCompressionLevel; level <= MaxCompressionLevel; level++ {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		for _, num := range []int{4, 5} {
			if _, ok := overrides[num]; !ok {
				t.Fatalf("level %d: content object %d has no override", level, num)
			}
		}
		reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
		if reopened.PageCount() != 1 {
			t.Fatalf("level %d: pages %d", level, reopened.PageCount())
		}
		content, err := reopened.Content(0)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if string(content) != want {
			t.Fatalf("level %d content %q", level, content)
		}
		checkCopiedValue(t, reopened, levelFontNum, "BaseFont", "Helvetica")
	}
}

func TestLevelImagePolicy(t *testing.T) {
	t.Parallel()
	checkDCTCap(t)
	checkFlateImages(t)
	checkSMaskCopy(t)
}

func checkDCTCap(t *testing.T) {
	t.Helper()
	file := levelImagePDF(t, dctImageBody(t, 1800, 900, ""))
	for _, level := range []int{1, 2} {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := overrides[levelImageNum]; ok {
			t.Fatalf("level %d changed the DCT image", level)
		}
	}
	cases := []struct {
		level  int
		width  int
		height int
	}{
		{level: 3, width: 1754, height: 877},
		{level: 4, width: 1123, height: 562},
		{level: 5, width: 842, height: 421},
	}
	for _, testCase := range cases {
		overrides, err := LevelOverrides(t.Context(), file, testCase.level)
		if err != nil {
			t.Fatalf("level %d: %v", testCase.level, err)
		}
		if _, ok := overrides[levelImageNum]; !ok {
			t.Fatalf("level %d: image has no override", testCase.level)
		}
		reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
		pic, err := reopened.DecodeImage(levelImageNum)
		if err != nil {
			t.Fatalf("level %d: %v", testCase.level, err)
		}
		bounds := pic.Bounds()
		if bounds.Dx() != testCase.width || bounds.Dy() != testCase.height {
			t.Fatalf("level %d: bounds %v, want %dx%d", testCase.level, bounds, testCase.width, testCase.height)
		}
		checkLevelFilter(t, reopened, levelImageNum, filterDCT)
	}
}

func checkFlateImages(t *testing.T) {
	t.Helper()
	samples := levelSamples()
	dict := "/Type /XObject /Subtype /Image /Width 4 /Height 4 /ColorSpace /DeviceRGB /BitsPerComponent 8"
	file := levelImagePDF(t,
		levelStreamBody(dict, samples),
		levelStreamBody(dict+" /Filter /FlateDecode", flateLevelBytes(t, samples)),
	)
	levelOne, err := LevelOverrides(t.Context(), file, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(levelOne) != 0 {
		t.Fatalf("level 1 overrides %v", levelOne)
	}
	overrides, err := LevelOverrides(t.Context(), file, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, num := range []int{4, 5} {
		if _, ok := overrides[num]; !ok {
			t.Fatalf("level 2: image %d has no override", num)
		}
	}
	reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
	for _, num := range []int{4, 5} {
		pic, err := reopened.DecodeImage(num)
		if err != nil {
			t.Fatalf("image %d: %v", num, err)
		}
		checkLevelPixels(t, pic, samples)
		checkLevelFilter(t, reopened, num, filterFlate)
	}
}

func checkSMaskCopy(t *testing.T) {
	t.Helper()
	file := levelImagePDF(t,
		dctImageBody(t, 40, 30, " /SMask 5 0 R"),
		dctImageBody(t, 40, 30, ""),
	)
	overrides, err := LevelOverrides(t.Context(), file, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := overrides[levelImageNum]; ok {
		t.Fatal("level 3 resampled an image with an /SMask")
	}
	if _, ok := overrides[levelSMaskNum]; !ok {
		t.Fatal("level 3 skipped the SMask image")
	}
}

func checkLevelFilter(t *testing.T, file *pdf.File, num int, want string) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	name, found := val.NameEntry(keyFilter)
	if !found || name != want {
		t.Fatalf("object %d Filter = %q found %v, want %q", num, name, found, want)
	}
}

func checkLevelPixels(t *testing.T, pic image.Image, want []byte) {
	t.Helper()
	for idx := range len(want) / imageChannels {
		x := idx % 4
		y := idx / 4
		red, green, blue, _ := pic.At(x, y).RGBA()
		got := []byte{byte(red >> colorByteShift), byte(green >> colorByteShift), byte(blue >> colorByteShift)}
		if !bytes.Equal(got, want[idx*imageChannels:(idx+1)*imageChannels]) {
			t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want[idx*imageChannels:(idx+1)*imageChannels])
		}
	}
}

// levelTextPDF is a one-page PDF with a text stream and a path stream.
func levelTextPDF(t *testing.T) *pdf.File {
	t.Helper()
	doc := newFixtureDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents [4 0 R 5 0 R] " +
		"/Resources << /Font << /F1 6 0 R >> >> >>")
	doc.object(string(streamBody([]byte(levelContentText), false)))
	doc.object(string(streamBody([]byte(lineMarks), false)))
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	return mustOpenPDF(t, doc.classic())
}

// levelImagePDF is a one-page PDF with no content stream and one object per
// body, starting at object 4.
func levelImagePDF(t *testing.T, bodies ...string) *pdf.File {
	t.Helper()
	doc := newFixtureDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R >>")
	for _, body := range bodies {
		doc.object(body)
	}
	return mustOpenPDF(t, doc.classic())
}

func levelStreamBody(dict string, raw []byte) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n", dict, len(raw)) + string(raw) + "\nendstream"
}

func dctImageBody(t *testing.T, width, height int, extra string) string {
	t.Helper()
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			at := pic.PixOffset(x, y)
			pic.Pix[at] = byte(x % 256)
			pic.Pix[at+1] = byte(y % 256)
			pic.Pix[at+2] = byte((x + y) % 256)
			pic.Pix[at+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, pic, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	dict := fmt.Sprintf(
		"/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode%s",
		width, height, extra)
	return levelStreamBody(dict, buf.Bytes())
}

func flateLevelBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	stored, err := flateBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

// levelSamples is one 4 by 4 RGB image of deterministic bytes.
func levelSamples() []byte {
	samples := make([]byte, 0, 4*4*imageChannels)
	for y := range 4 {
		for x := range 4 {
			samples = append(samples, byte(x*10), byte(y*20), byte((x+y)*5))
		}
	}
	return samples
}
