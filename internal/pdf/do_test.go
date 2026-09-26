package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestPaintDoImageRGB paints a 2 by 2 DeviceRGB image over a 2 by 2 pixmap
// through the page's own resources and through inherited resources.
func TestPaintDoImageRGB(t *testing.T) {
	t.Run("direct resources", func(t *testing.T) {
		file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>", rgbImageBody(t))
		checkDoRGB(t, file)
	})
	t.Run("inherited resources", func(t *testing.T) {
		file := inheritedDoPage(t, "2 0 0 2 0 0 cm /Im0 Do",
			"<< /XObject << /Im0 5 0 R >> >>", rgbImageBody(t))
		checkDoRGB(t, file)
	})
}

func checkDoRGB(t *testing.T, file *File) {
	t.Helper()
	pixmap := graphics.NewPixmap(2, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 0, 0, 255, 0, 0)
	wantPixel(t, img, 1, 0, 0, 255, 0)
	wantPixel(t, img, 0, 1, 0, 0, 255)
	wantPixel(t, img, 1, 1, 255, 255, 255)
}

// TestPaintDoImageGray paints a 2 by 2 DeviceGray image.
func TestPaintDoImageGray(t *testing.T) {
	file := doPage(t, "2 0 0 2 0 0 cm /Im0 Do",
		"<< /XObject << /Im0 5 0 R >> >>",
		imageStream(t, 2, 2, "/DeviceGray", "/FlateDecode", flateRaw(t, grayPixels())))
	pixmap := graphics.NewPixmap(2, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 0, 0, 0, 0, 0)
	wantPixel(t, img, 1, 0, 85, 85, 85)
	wantPixel(t, img, 0, 1, 170, 170, 170)
	wantPixel(t, img, 1, 1, 255, 255, 255)
}

// TestPaintDoCM proves the stamp uses the current matrix, not the identity.
func TestPaintDoCM(t *testing.T) {
	file := doPage(t, "2 0 0 2 1 0 cm /Im0 Do",
		"<< /XObject << /Im0 5 0 R >> >>", rgbImageBody(t))
	pixmap := graphics.NewPixmap(3, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 0, 0, 255, 255, 255)
	wantPixel(t, img, 0, 1, 255, 255, 255)
	wantPixel(t, img, 1, 0, 255, 0, 0)
	wantPixel(t, img, 2, 0, 0, 255, 0)
	wantPixel(t, img, 1, 1, 0, 0, 255)
	wantPixel(t, img, 2, 1, 255, 255, 255)
}

// TestPaintDoMissing rejects every name Do cannot decode: no resources, an
// unknown name, a non-image subtype, and a stream whose filter cannot decode.
func TestPaintDoMissing(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		resources string
		bodies    []string
	}{
		{name: "no resources", content: "/Im0 Do"},
		{
			name:      "unknown name",
			content:   "/Nope Do",
			resources: "<< /XObject << /Im0 5 0 R >> >>",
			bodies:    []string{rgbImageBody(t)},
		},
		{
			name:      "form subtype",
			content:   "/Fm0 Do",
			resources: "<< /XObject << /Fm0 5 0 R >> >>",
			bodies:    []string{"<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] >>"},
		},
		{
			name:      "unknown filter",
			content:   "/Im0 Do",
			resources: "<< /XObject << /Im0 5 0 R >> >>",
			bodies:    []string{imageStream(t, 2, 2, "/DeviceRGB", "/LZWDecode", rgbPixels())},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			file := doPage(t, testCase.content, testCase.resources, testCase.bodies...)
			checkDoRejected(t, file)
		})
	}
}

// checkDoRejected proves a refused image painted no pixels. The /SMask
// rejection row was retired in v0.0.4; TestPaintSMask is its positive canary.
func checkDoRejected(t *testing.T, file *File) {
	t.Helper()
	pixmap := graphics.NewPixmap(2, 2)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJob(t, err, "Do", errUndefined)
	img := shown(t, pixmap)
	if !rowWhite(img, 0) || !rowWhite(img, 1) {
		t.Fatal("a rejected image painted pixels")
	}
}

// doPage builds a one-page PDF whose content is content and whose page
// resources are resources. Each body becomes one object starting at 5.
func doPage(t *testing.T, content, resources string, bodies ...string) *File {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R"
	if resources != "" {
		page += " /Resources " + resources
	}
	page += " >>"
	doc.object(page)
	doc.object(streamBody("", []byte(content)))
	for _, body := range bodies {
		doc.object(body)
	}
	return mustOpen(t, doc.classic(""))
}

// inheritedDoPage is doPage with /Resources on the /Pages ancestor.
func inheritedDoPage(t *testing.T, content, resources string, bodies ...string) *File {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 /Resources " + resources + " >>")
	doc.object(pageBody)
	doc.object(streamBody("", []byte(content)))
	for _, body := range bodies {
		doc.object(body)
	}
	return mustOpen(t, doc.classic(""))
}

func rgbImageBody(t *testing.T) string {
	t.Helper()
	return imageStream(t, 2, 2, "/DeviceRGB", "/FlateDecode", flateRaw(t, rgbPixels()))
}

func maskedImageBody(t *testing.T) string {
	t.Helper()
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode /SMask 6 0 R"
	return streamBody(dict, flateRaw(t, rgbPixels()))
}
