package spectreps_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationDoPolicy locks the v0.0.4 Do policy: a missing name, a
// non-image subtype, and an unusable /SMask refuse with undefined in Do and
// paint nothing, while a color-key /Mask array, a stencil stream /Mask, and a
// /Decode array decode and paint. The v0.0.3 behavior that ignored /Mask and
// /Decode is gone.
func TestValidationDoPolicy(t *testing.T) {
	in := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 2, PageHeightPt: 2, ResolutionDPI: 72}

	t.Run("missing name", func(t *testing.T) {
		payload := doPolicyPDF(t, "/Nope Do", "<< >>")
		refuseDo(t, in, payload, opt)
	})
	t.Run("non-image subtype", func(t *testing.T) {
		payload := doPolicyPDF(t, "/X0 Do", "<< /XObject << /X0 5 0 R >> >>",
			[]byte("<< /Type /XObject /Subtype /PS >>"))
		refuseDo(t, in, payload, opt)
	})
	t.Run("unusable SMask", func(t *testing.T) {
		payload := doPolicyPDF(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			doPolicyImage(t, "/Subtype /Image /Width 2 /Height 2 "+
				"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode "+
				"/SMask 6 0 R", doPolicyRGBPixels()),
			[]byte("<< /S /Alpha >>"))
		refuseDo(t, in, payload, opt)
	})
	t.Run("color key Mask", func(t *testing.T) {
		payload := doPolicyPDF(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			doPolicyImage(t, "/Subtype /Image /Width 2 /Height 2 "+
				"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode "+
				"/Mask [0.5 1 0 0.25 0 0.25]", doPolicyRGBPixels()))
		img := paintDo(t, in, payload, opt)
		wantDoPixel(t, img, 0, 0, 255, 255, 255)
		wantDoPixel(t, img, 1, 0, 0, 255, 0)
		wantDoPixel(t, img, 0, 1, 0, 0, 255)
		wantDoPixel(t, img, 1, 1, 255, 255, 255)
	})
	t.Run("stream Mask", func(t *testing.T) {
		payload := doPolicyPDF(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			doPolicyImage(t, "/Subtype /Image /Width 2 /Height 2 "+
				"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode "+
				"/Mask 6 0 R", doPolicyRGBPixels()),
			doPolicyImage(t, "/Subtype /Image /Width 2 /Height 2 "+
				"/ImageMask true /Filter /FlateDecode", []byte{0x80, 0x40}))
		img := paintDo(t, in, payload, opt)
		wantDoPixel(t, img, 0, 0, 255, 0, 0)
		wantDoPixel(t, img, 1, 0, 255, 255, 255)
		wantDoPixel(t, img, 0, 1, 255, 255, 255)
		wantDoPixel(t, img, 1, 1, 255, 255, 255)
	})
	t.Run("Decode array", func(t *testing.T) {
		payload := doPolicyPDF(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			doPolicyImage(t, "/Subtype /Image /Width 2 /Height 2 "+
				"/ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode "+
				"/Decode [1 0]", []byte{0, 85, 170, 255}))
		img := paintDo(t, in, payload, opt)
		wantDoPixel(t, img, 0, 0, 255, 255, 255)
		wantDoPixel(t, img, 1, 0, 170, 170, 170)
		wantDoPixel(t, img, 0, 1, 85, 85, 85)
		wantDoPixel(t, img, 1, 1, 0, 0, 0)
	})
}

// doPolicyPDF builds a one-page 2 by 2 PDF with the page resources and body
// objects the Do policy cases need. Bodies start at object 5.
func doPolicyPDF(t *testing.T, content, resources string, bodies ...[]byte) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 2 2] " +
			"/Contents 4 0 R /Resources " + resources + " >>"),
		flateStream(t, content),
	}
	return classicXref(t, append(objects, bodies...))
}

// doPolicyImage builds one Flate image object whose decoded bytes are plain.
func doPolicyImage(t *testing.T, dict string, plain []byte) []byte {
	t.Helper()
	compressed := flateBytes(t, plain)
	var body bytes.Buffer
	writef(t, &body, "<< /Length %d %s>>\nstream\n", len(compressed), dict)
	writeAll(t, &body, compressed)
	writeString(t, &body, "\nendstream")
	return body.Bytes()
}

// doPolicyRGBPixels is a 2 by 2 image: red, green on the top row and blue,
// white on the bottom row.
func doPolicyRGBPixels() []byte {
	return []byte{
		255, 0, 0, 0, 255, 0,
		0, 0, 255, 255, 255, 255,
	}
}

// refuseDo proves one Do case returns undefined in Do and paints nothing.
func refuseDo(t *testing.T, in *spectreps.Instance, payload []byte, opt spectreps.RunOptions) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	img, err := in.RasterizePage(t.Context(), doc, 0, opt)
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != "Do" || job.Msg != undefinedMsg {
		t.Fatalf("error = %v, want undefined in Do", err)
	}
	requireZeroPageImage(t, img)
}

// paintDo rasterizes the 2 by 2 page of one Do case.
func paintDo(
	t *testing.T, in *spectreps.Instance, payload []byte, opt spectreps.RunOptions,
) spectreps.PageImage {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	img, err := in.RasterizePage(t.Context(), doc, 0, opt)
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 2 || img.Height != 2 {
		t.Fatalf("page = %dx%d, want 2x2", img.Width, img.Height)
	}
	return img
}

// wantDoPixel checks one top-down page pixel.
func wantDoPixel(
	t *testing.T, img spectreps.PageImage, col, row int, red, green, blue byte,
) {
	t.Helper()
	at := row*img.Stride + col*3
	gotRed := img.Pixels[at]
	gotGreen := img.Pixels[at+1]
	gotBlue := img.Pixels[at+2]
	if gotRed == red && gotGreen == green && gotBlue == blue {
		return
	}
	t.Fatalf(
		"pixel (%d,%d) = %d,%d,%d, want %d,%d,%d",
		col, row, gotRed, gotGreen, gotBlue, red, green, blue,
	)
}
