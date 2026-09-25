package spectreps_test

import (
	"errors"
	"os"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestYFlip(t *testing.T) {
	const side = 200
	pages := runPS(t, "0 0 moveto currentpoint 0 eq exch 0 eq and { 100 0 lineto stroke } if", spectreps.RunOptions{
		PageWidthPt:   side,
		PageHeightPt:  side,
		ResolutionDPI: 72,
	})
	if len(pages) != 1 || pages[0].Width != side || pages[0].Height != side {
		t.Fatalf("page %+v", pages[0].Width)
	}
	img := pages[0]
	if rowPainted(img, 0) {
		t.Fatal("row 0 is painted")
	}
	if !rowPainted(img, img.Height-1) {
		t.Fatal("bottom row is blank")
	}
}

func TestPaint(t *testing.T) {
	opt := spectreps.RunOptions{PageWidthPt: 40, PageHeightPt: 40, ResolutionDPI: 72}
	stroke := runPS(t, "0 0 moveto 20 0 lineto stroke", opt)
	if len(stroke) != 1 || !rowPainted(stroke[0], stroke[0].Height-1) {
		t.Fatal("stroke did not paint")
	}
	checkFill(t, opt)
	hole := "0 0 moveto 30 0 lineto 30 30 lineto 0 30 lineto closepath " +
		"8 8 moveto 22 8 lineto 22 22 lineto 8 22 lineto closepath "
	checkEvenOdd(t, hole, opt)
	//nolint:dupword // two showpage operators produce two pages
	two := runPS(t, "showpage showpage", opt)
	if len(two) != 2 || rowPainted(two[0], 0) || rowPainted(two[1], 0) {
		t.Fatalf("showpage pages = %d", len(two))
	}
	blank := runPS(t, "", opt)
	if len(blank) != 1 || rowPainted(blank[0], blank[0].Height-1) {
		t.Fatal("empty program was not one blank page")
	}
	painted := runPS(t, "0 0 moveto 10 0 lineto stroke", opt)
	if len(painted) != 1 || !rowPainted(painted[0], painted[0].Height-1) {
		t.Fatal("paint without showpage did not return one image")
	}
}

func checkFill(t *testing.T, opt spectreps.RunOptions) {
	t.Helper()
	filled := runPS(t, "0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill", opt)
	if !pixelBlack(filled[0], 10, 10) {
		t.Fatal("fill left the interior white")
	}
}

func checkEvenOdd(t *testing.T, hole string, opt spectreps.RunOptions) {
	t.Helper()
	evenOdd := runPS(t, hole+"eofill", opt)
	nonzero := runPS(t, hole+"fill", opt)
	if pixelBlack(evenOdd[0], 15, 15) {
		t.Fatal("eofill painted the hole")
	}
	if !pixelBlack(nonzero[0], 15, 15) {
		t.Fatal("fill left the hole white")
	}
	if !pixelBlack(evenOdd[0], 2, 2) {
		t.Fatal("eofill left the border white")
	}
}

func TestPixelCap(t *testing.T) {
	in := newInst(t)
	_, err := in.RunPostScript(t.Context(), []byte("showpage"), spectreps.RunOptions{
		PageWidthPt:   20001,
		PageHeightPt:  10,
		ResolutionDPI: 72,
	})
	assertLimit(t, err)
	_, err = in.RunPostScript(t.Context(), []byte("showpage"), spectreps.RunOptions{
		PageWidthPt:   10000,
		PageHeightPt:  5000,
		ResolutionDPI: 72,
	})
	assertLimit(t, err)
}

func TestRunPostScript(t *testing.T) {
	pages := runPS(t, "1 2 add", spectreps.RunOptions{})
	if len(pages) != 1 || pages[0].Width != 612 || pages[0].Height != 792 {
		t.Fatalf("default page %dx%d count %d", pages[0].Width, pages[0].Height, len(pages))
	}
	in := newInst(t)
	_, err := in.RunPostScript(t.Context(), []byte("/Nope findfont"), spectreps.RunOptions{})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != "invalidfont" || job.Op != "findfont" {
		t.Fatalf("findfont error = %v", err)
	}
}

func TestFixturePPM(t *testing.T) {
	pages := runPS(t, "0 0 moveto 100 0 lineto stroke", spectreps.RunOptions{
		PageWidthPt:   200,
		PageHeightPt:  200,
		ResolutionDPI: 72,
	})
	got := encodePPM(pages[0])
	want, err := os.ReadFile("../sampledata/fixtures/line-bottom.ppm")
	if err != nil {
		t.Fatal(err)
	}
	if spectreps.CompareFiles(got, want).Equal {
		return
	}
	res := spectreps.CompareFiles(got, want)
	t.Fatalf("fixture mismatch %+v", res)
}

func runPS(t *testing.T, src string, opt spectreps.RunOptions) []spectreps.PageImage {
	t.Helper()
	pages, err := newInst(t).RunPostScript(t.Context(), []byte(src), opt)
	if err != nil {
		t.Fatalf("RunPostScript(%q) %v", src, err)
	}
	return pages
}

func newInst(t *testing.T) *spectreps.Instance {
	t.Helper()
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

func assertLimit(t *testing.T, err error) {
	t.Helper()
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != "limitcheck" {
		t.Fatalf("error = %v, want limitcheck", err)
	}
}

func rowPainted(img spectreps.PageImage, row int) bool {
	base := row * img.Stride
	for x := range img.Width * 3 {
		if img.Pixels[base+x] != 255 {
			return true
		}
	}
	return false
}

func pixelBlack(img spectreps.PageImage, x, yFromBottom int) bool {
	row := img.Height - 1 - yFromBottom
	i := row*img.Stride + x*3
	return img.Pixels[i] == 0 && img.Pixels[i+1] == 0 && img.Pixels[i+2] == 0
}

func encodePPM(img spectreps.PageImage) []byte {
	header := []byte("P6\n" + itoa(img.Width) + " " + itoa(img.Height) + "\n255\n")
	body := make([]byte, 0, img.Width*img.Height*3)
	for row := range img.Height {
		base := row * img.Stride
		body = append(body, img.Pixels[base:base+img.Width*3]...)
	}
	return append(header, body...)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
