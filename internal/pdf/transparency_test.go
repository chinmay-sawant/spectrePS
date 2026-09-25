package pdf

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestPaintExtGStateAlpha proves /ca and /CA reach the pixmap through the
// optional AlphaMarker seam: a fill composites at the fill alpha and a stroke
// at the stroke alpha, rounded per channel.
func TestPaintExtGStateAlpha(t *testing.T) {
	t.Run("fill alpha", func(t *testing.T) {
		file := extGStatePage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /ca 0.5 >>")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 128, 128)
	})
	t.Run("stroke alpha", func(t *testing.T) {
		file := extGStatePage(t, "/GS0 gs 0 0 0 RG 0 0 m 10 0 l S",
			"<< /Type /ExtGState /CA 0.25 >>")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 19, 191, 191, 191)
	})
	t.Run("alpha clamps", func(t *testing.T) {
		file := extGStatePage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /ca 2 >>")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("bad value", func(t *testing.T) {
		file := extGStatePage(t, "/GS0 gs", "<< /Type /ExtGState /ca (half) >>")
		err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
		wantJobErr(t, err, "gs", nameType)
	})
}

// TestPaintExtGStateRestore proves q/Q restore the alpha and the blend mode,
// so a mark after Q paints with the state from before q.
func TestPaintExtGStateRestore(t *testing.T) {
	t.Run("alpha", func(t *testing.T) {
		file := extGStatePage(t,
			"q /GS0 gs 1 0 0 rg 0 0 20 20 re f Q 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /ca 0.5 >>")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("blend", func(t *testing.T) {
		file := extGStatePage(t,
			"0 1 0 rg 0 0 20 20 re f q /GS0 gs Q 1 0 0 rg 0 0 2 2 re f",
			"<< /Type /ExtGState /BM /Multiply >>")
		img := paintPage(t, file)
		wantPixel(t, img, 1, 18, 255, 0, 0)
	})
	t.Run("soft mask", func(t *testing.T) {
		file := softMaskPage(t, "q /GS0 gs Q 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /SMask << /S /Alpha /G 6 0 R >> >>", "1 g 0 0 20 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, 255, 0, 0)
		wantPixel(t, img, 15, 10, 255, 0, 0)
	})
}

// TestPaintBlendMode paints over a backdrop of byte 200 with a source of byte
// 100 and locks the exact Multiply and Screen bytes through the PDF layer.
// The four non-separable modes are a named refusal.
func TestPaintBlendMode(t *testing.T) {
	t.Run("multiply", func(t *testing.T) {
		file := extGStatePage(t,
			"0.784314 0.784314 0.784314 rg 0 0 2 2 re f "+
				"/GS0 gs 0.392157 0.392157 0.392157 rg 0 0 2 2 re f",
			"<< /Type /ExtGState /BM /Multiply >>")
		img := paintPage(t, file)
		checkBlendBlock(t, img, 78)
	})
	t.Run("screen", func(t *testing.T) {
		file := extGStatePage(t,
			"0.784314 0.784314 0.784314 rg 0 0 2 2 re f "+
				"/GS0 gs 0.392157 0.392157 0.392157 rg 0 0 2 2 re f",
			"<< /Type /ExtGState /BM /Screen >>")
		img := paintPage(t, file)
		checkBlendBlock(t, img, 222)
	})
	t.Run("all separable names", func(t *testing.T) {
		names := []string{
			"Normal", "Multiply", "Screen", "Overlay", "Darken", "Lighten",
			"ColorDodge", "ColorBurn", "HardLight", "SoftLight", "Difference",
			"Exclusion",
		}
		for _, name := range names {
			file := extGStatePage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
				"<< /Type /ExtGState /BM /"+name+" >>")
			// A name that resolves paints; the exact formulas are locked by
			// TestPixmapBlendMode and by the Multiply and Screen subtests.
			_ = paintPage(t, file)
		}
	})
	t.Run("non separable", func(t *testing.T) {
		for _, name := range []string{"Hue", "Saturation", "Color", "Luminosity"} {
			file := extGStatePage(t, "/GS0 gs", "<< /Type /ExtGState /BM /"+name+" >>")
			err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
			wantJobErr(t, err, "gs", nameUndefined)
		}
	})
	t.Run("array refuses", func(t *testing.T) {
		file := extGStatePage(t, "/GS0 gs", "<< /Type /ExtGState /BM [/Multiply] >>")
		err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
		wantJobErr(t, err, "gs", nameUndefined)
	})
}

// TestPaintBlendModeDefault proves a gs without /BM, and one that names
// Normal, leave the source byte in place.
func TestPaintBlendModeDefault(t *testing.T) {
	t.Run("no blend entry", func(t *testing.T) {
		file := extGStatePage(t,
			"0.784314 0.784314 0.784314 rg 0 0 2 2 re f "+
				"/GS0 gs 0.392157 0.392157 0.392157 rg 0 0 2 2 re f",
			"<< /Type /ExtGState /LW 1 >>")
		img := paintPage(t, file)
		checkBlendBlock(t, img, 100)
	})
	t.Run("normal", func(t *testing.T) {
		file := extGStatePage(t,
			"0.784314 0.784314 0.784314 rg 0 0 2 2 re f "+
				"/GS0 gs 0.392157 0.392157 0.392157 rg 0 0 2 2 re f",
			"<< /Type /ExtGState /BM /Normal >>")
		img := paintPage(t, file)
		checkBlendBlock(t, img, 100)
	})
}

// checkBlendBlock checks the four bottom-left pixels of the blend fixture,
// where the two unit-square fills land.
func checkBlendBlock(t *testing.T, img graphics.Image, want byte) {
	t.Helper()
	for row := range 2 {
		for col := range 2 {
			wantPixel(t, img, col, pageSide-1-row, want, want, want)
		}
	}
}

// TestPaintSMask proves an image /SMask decodes to an alpha plane and blends
// the base image through DrawImage, and that /Matte is a no-op.
func TestPaintSMask(t *testing.T) {
	t.Run("alpha plane", func(t *testing.T) {
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			maskedImageBody(t),
			grayMaskBody(t, ""))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 1, 0, 170, whiteByte, 170)
		wantPixel(t, img, 0, 1, 85, 85, whiteByte)
		wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
	})
	t.Run("matte ignored", func(t *testing.T) {
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			maskedImageBody(t),
			grayMaskBody(t, " /Matte [0 0 0]"))
		plain := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			maskedImageBody(t), grayMaskBody(t, ""))
		if !bytes.Equal(paintSmallPage(t, file).Pixels, paintSmallPage(t, plain).Pixels) {
			t.Fatal("a /Matte entry changed the pixels")
		}
	})
}

// TestPaintSMaskDecode proves the mask /Decode array remaps the mask samples
// before the alpha blend.
func TestPaintSMaskDecode(t *testing.T) {
	file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
		"<< /XObject << /Im0 5 0 R >> >>",
		maskedImageBody(t),
		grayMaskBody(t, " /Decode [1 0]"))
	img := paintSmallPage(t, file)
	wantPixel(t, img, 0, 0, 255, 0, 0)
	wantPixel(t, img, 1, 0, 85, whiteByte, 85)
	wantPixel(t, img, 0, 1, 170, 170, whiteByte)
	wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
}

// grayMaskBody builds one 2 by 2 DeviceGray Flate soft mask over grayPixels.
func grayMaskBody(t *testing.T, extra string) string {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceGray " +
		"/BitsPerComponent 8 /Filter /FlateDecode" + extra
	return streamBody(dict, flateRaw(t, grayPixels()))
}

// TestPaintImageMask proves /ImageMask true decodes one bit per sample, paints
// the current fill color where the mask is 1, and honors /Decode.
func TestPaintImageMask(t *testing.T) {
	t.Run("fill color", func(t *testing.T) {
		file := doPage(t, "1 0 0 rg 2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>", imageMaskBody(t, ""))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, 255, 0, 0)
		wantPixel(t, img, 1, 0, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 0, 1, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 1, 1, 255, 0, 0)
	})
	t.Run("decode inverts", func(t *testing.T) {
		file := doPage(t, "1 0 0 rg 2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>", imageMaskBody(t, " /Decode [1 0]"))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 1, 0, 255, 0, 0)
		wantPixel(t, img, 0, 1, 255, 0, 0)
		wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
	})
	t.Run("blue fill", func(t *testing.T) {
		file := doPage(t, "0 0 1 rg 2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>", imageMaskBody(t, ""))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, 0, 0, 255)
	})
	t.Run("bad bit depth", func(t *testing.T) {
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			streamBody("/Subtype /Image /Width 2 /Height 2 /ImageMask true "+
				"/BitsPerComponent 8 /Filter /FlateDecode", flateRaw(t, grayPixels())))
		err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "Do", nameUndefined)
	})
}

// imageMaskBody builds a 2 by 2 bilevel image mask whose bits are 10 on the
// top row and 01 on the bottom row.
func imageMaskBody(t *testing.T, extra string) string {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ImageMask true " +
		"/Filter /FlateDecode" + extra
	return streamBody(dict, flateRaw(t, []byte{0x80, 0x40}))
}

// TestPaintColorKeyMask proves a color-key /Mask array keys out pixels whose
// samples fall inside every range, and a stream /Mask becomes a stencil.
func TestPaintColorKeyMask(t *testing.T) {
	t.Run("color key array", func(t *testing.T) {
		dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Mask [0.5 1 0 0.25 0 0.25]"
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			streamBody(dict, flateRaw(t, rgbPixels())))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 1, 0, 0, 255, 0)
		wantPixel(t, img, 0, 1, 0, 0, 255)
		wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
	})
	t.Run("stencil stream", func(t *testing.T) {
		dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Mask 6 0 R"
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>",
			streamBody(dict, flateRaw(t, rgbPixels())),
			imageMaskBody(t, ""))
		img := paintSmallPage(t, file)
		wantPixel(t, img, 0, 0, 255, 0, 0)
		wantPixel(t, img, 1, 0, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 0, 1, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
	})
}

// TestImageDecodeArray proves /Decode remaps gray and RGB samples before the
// color conversion.
func TestImageDecodeArray(t *testing.T) {
	t.Parallel()
	t.Run("gray invert", func(t *testing.T) {
		t.Parallel()
		dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceGray " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Decode [1 0]"
		file, num := oneImageDoc(t, dict, flateRaw(t, grayPixels()))
		pic, err := file.DecodeImage(num)
		if err != nil {
			t.Fatal(err)
		}
		checkGrayPixels(t, pic, []byte{255, 170, 85, 0})
	})
	t.Run("rgb swap", func(t *testing.T) {
		t.Parallel()
		dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Decode [0 1 1 0 0 1]"
		file, num := oneImageDoc(t, dict, flateRaw(t, rgbPixels()))
		pic, err := file.DecodeImage(num)
		if err != nil {
			t.Fatal(err)
		}
		// Only the green component inverts, so red becomes yellow, green
		// becomes black, blue becomes cyan, and white becomes magenta.
		checkRGBPixels(t, pic, []byte{
			255, 255, 0, 0, 0, 0,
			0, 255, 255, 255, 0, 255,
		})
	})
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceGray " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Decode [1]"
		file, num := oneImageDoc(t, dict, flateRaw(t, grayPixels()))
		_, err := file.DecodeImage(num)
		wantErr(t, err, opImage, errUndefined)
	})
	t.Run("indexed refuses", func(t *testing.T) {
		t.Parallel()
		dict := "/Subtype /Image /Width 2 /Height 2 " +
			"/ColorSpace [/Indexed /DeviceRGB 1 <FF0000 00FF00>] " +
			"/BitsPerComponent 8 /Filter /FlateDecode /Decode [1 0]"
		file, num := oneImageDoc(t, dict, flateRaw(t, []byte{0, 1, 0, 1}))
		_, err := file.DecodeImage(num)
		wantErr(t, err, opImage, errUndefined)
	})
}

// TestPaintSoftMask proves an /ExtGState /SMask with /S /Alpha builds the soft
// mask and applies it to later marks, and /None clears it.
func TestPaintSoftMask(t *testing.T) {
	t.Run("alpha mask", func(t *testing.T) {
		file := softMaskPage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /SMask << /S /Alpha /G 6 0 R >> >>",
			"1 g 0 0 10 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, 255, 0, 0)
		wantPixel(t, img, 15, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("identity transfer", func(t *testing.T) {
		file := softMaskPage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /SMask << /S /Alpha /G 6 0 R /TR /Identity >> >>",
			"1 g 0 0 10 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, 255, 0, 0)
		wantPixel(t, img, 15, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("none clears", func(t *testing.T) {
		file := softMaskPage(t,
			"/GS0 gs /GS1 gs 1 0 0 rg 0 0 20 20 re f",
			"<< /Type /ExtGState /SMask << /S /Alpha /G 6 0 R >> >>",
			"1 g 0 0 20 20 re f",
			"<< /Type /ExtGState /SMask /None >>")
		img := paintPage(t, file)
		wantPixel(t, img, 5, 10, 255, 0, 0)
		wantPixel(t, img, 15, 10, 255, 0, 0)
	})
	t.Run("bad transfer", func(t *testing.T) {
		file := softMaskPage(t, "/GS0 gs",
			"<< /Type /ExtGState /SMask << /S /Alpha /G 6 0 R /TR 5 0 R >> >>",
			"1 g 0 0 10 20 re f")
		err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
		wantJobErr(t, err, "gs", nameUndefined)
	})
}

// TestPaintSoftMaskLuminosity proves /S /Luminosity uses the ISO weighted
// channel sum as the coverage.
func TestPaintSoftMaskLuminosity(t *testing.T) {
	file := softMaskPage(t, "/GS0 gs 1 0 0 rg 0 0 20 20 re f",
		"<< /Type /ExtGState /SMask << /S /Luminosity /G 6 0 R >> >>",
		"0.5 g 0 0 10 20 re f")
	img := paintPage(t, file)
	wantPixel(t, img, 5, 10, 255, 127, 127)
	wantPixel(t, img, 15, 10, whiteByte, whiteByte, whiteByte)
}

// softMaskPage builds one page with two ExtGState names and one form for the
// mask group. The second body is the form; a third body is an extra state.
func softMaskPage(t *testing.T, content, state, form string, extra ...string) *File {
	t.Helper()
	bodies := []string{state, formBody("/BBox [0 0 20 20]", form)}
	bodies = append(bodies, extra...)
	resources := "<< /ExtGState << /GS0 5 0 R"
	if len(extra) > 0 {
		resources += " /GS1 7 0 R"
	}
	resources += " >> >>"
	return doPage(t, content, resources, bodies...)
}

// TestPaintGroupTransparency proves a Form XObject with /Group /S
// /Transparency paints into a scratch page and composites once with the group
// alpha.
func TestPaintGroupTransparency(t *testing.T) {
	t.Run("half alpha", func(t *testing.T) {
		file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, "<< /S /Transparency /CA 0.5 >>",
				"1 0 0 rg 0 0 20 20 re f"))
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 128, 128)
	})
	t.Run("opaque", func(t *testing.T) {
		file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, "<< /S /Transparency >>",
				"1 0 0 rg 0 0 20 20 re f"))
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("cs i k read", func(t *testing.T) {
		group := "<< /S /Transparency /CS /DeviceRGB /I true /K false >>"
		file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, group, "1 0 0 rg 0 0 20 20 re f"))
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("malformed", func(t *testing.T) {
		cases := []struct {
			name  string
			group string
		}{
			{name: "subtype", group: "<< /S /Isolated >>"},
			{name: "isolated type", group: "<< /S /Transparency /I 1 >>"},
			{name: "knockout type", group: "<< /S /Transparency /K /Yes >>"},
			{name: "alpha type", group: "<< /S /Transparency /CA (half) >>"},
			{name: "space", group: "<< /S /Transparency /CS /Lab >>"},
			{name: "unknown key", group: "<< /S /Transparency /X 1 >>"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
					transparencyFormBody(t, testCase.group, "1 0 0 rg 0 0 1 1 re f"))
				err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
				wantJobErr(t, err, "Do", nameUndefined)
			})
		}
	})
}

// TestPaintGroupIsolation proves the scratch page starts transparent: pixels
// the group never paints keep the page color, so a small mark does not blank
// the page.
func TestPaintGroupIsolation(t *testing.T) {
	t.Run("isolated", func(t *testing.T) {
		file := doPage(t, "0 1 0 rg 0 0 20 20 re f /Fm0 Do",
			"<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, "<< /S /Transparency /I true >>",
				"1 0 0 rg 0 0 5 5 re f"))
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 0, 255, 0)
		wantPixel(t, img, 2, 17, 255, 0, 0)
	})
	t.Run("non isolated", func(t *testing.T) {
		file := doPage(t, "0 1 0 rg 0 0 20 20 re f /Fm0 Do",
			"<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, "<< /S /Transparency /I false >>",
				"1 0 0 rg 0 0 5 5 re f"))
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 0, 255, 0)
		wantPixel(t, img, 2, 17, 255, 0, 0)
	})
	t.Run("clipped out", func(t *testing.T) {
		file := doPage(t, "10 0 10 20 re W n /Fm0 Do",
			"<< /XObject << /Fm0 5 0 R >> >>",
			transparencyFormBody(t, "<< /S /Transparency >>",
				"1 0 0 rg 0 0 5 5 re f"))
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 2, 17, whiteByte, whiteByte, whiteByte)
		wantPixel(t, img, 15, 10, whiteByte, whiteByte, whiteByte)
	})
}

// TestScratchCaps proves the scratch page obeys the page pixel and side caps.
func TestScratchCaps(t *testing.T) {
	if !scratchFits(pageSide, pageSide) {
		t.Fatal("a page-sized scratch refused")
	}
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "width side", width: maxScratchSide + 1, height: 1},
		{name: "height side", width: 1, height: maxScratchSide + 1},
		{name: "pixels", width: maxScratchSide, height: maxScratchSide},
		{name: "zero", width: 0, height: 1},
	}
	for _, testCase := range cases {
		if scratchFits(testCase.width, testCase.height) {
			t.Fatalf("%s fit the caps", testCase.name)
		}
	}
}

// transparencyFormBody builds one /Subtype /Form stream with a transparency
// group.
func transparencyFormBody(t *testing.T, group, content string) string {
	t.Helper()
	dict := "/Type /XObject /Subtype /Form /BBox [0 0 20 20] /Group " + group
	return streamBody(dict, []byte(content))
}

// paintPage rasterizes page 0 of file onto a pageSide square and returns the
// image.
func paintPage(t *testing.T, file *File) graphics.Image {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	return shown(t, pixmap)
}

// paintSmallPage rasterizes page 0 of file onto a 2 by 2 page, the size the
// image fixtures stamp over.
func paintSmallPage(t *testing.T, file *File) graphics.Image {
	t.Helper()
	pixmap := graphics.NewPixmap(2, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	return shown(t, pixmap)
}
