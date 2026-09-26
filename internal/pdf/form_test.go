package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// formBody builds one /Subtype /Form stream. extra carries /Matrix, /BBox, or
// /Resources entries.
func formBody(extra, content string) string {
	dict := "/Type /XObject /Subtype /Form"
	if extra != "" {
		dict += " " + extra
	}
	return streamBody(dict, []byte(content))
}

// TestPaintFormXObject proves Do runs a form content stream: /Matrix
// concatenates into the CTM, /BBox clips the form's marks, and the implicit
// q/Q keeps the form's state changes from reaching the caller.
func TestPaintFormXObject(t *testing.T) {
	t.Run("matrix scales", checkFormMatrix)
	t.Run("bbox clips", checkFormBBox)
	t.Run("bbox matrix", checkFormBBoxMatrix)
	t.Run("state restored", checkFormState)
	t.Run("malformed", checkFormMalformed)
}

func checkFormMatrix(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/Matrix [2 0 0 2 0 0] /BBox [0 0 10 10]", "1 0 0 rg 0 0 5 5 re f"))
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 7, 10, 255, 0, 0)
	wantPixel(t, img, 12, 10, whiteByte, whiteByte, whiteByte)
}

func checkFormBBox(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/BBox [0 0 5 5]", "1 0 0 rg 0 0 20 20 re f"))
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 2, 17, 255, 0, 0)
	wantPixel(t, img, 7, 17, whiteByte, whiteByte, whiteByte)
	wantPixel(t, img, 2, 12, whiteByte, whiteByte, whiteByte)
}

func checkFormBBoxMatrix(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/Matrix [2 0 0 2 0 0] /BBox [0 0 5 5]", "1 0 0 rg 0 0 20 20 re f"))
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 7, 10, 255, 0, 0)
	wantPixel(t, img, 12, 10, whiteByte, whiteByte, whiteByte)
}

func checkFormState(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do 0 1 0 rg 0 0 5 5 re f", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/BBox [0 0 20 20]", "1 0 0 rg"))
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	wantPixel(t, shown(t, pixmap), 2, 17, 0, 255, 0)
}

func checkFormMalformed(t *testing.T) {
	t.Helper()
	notStream := "<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] >>"
	cases := []struct {
		name string
		body string
	}{
		{name: "no bbox", body: formBody("", "0 0 m 1 0 l S")},
		{name: "bad bbox", body: formBody("/BBox [0 0 1]", "0 0 m 1 0 l S")},
		{name: "bad matrix", body: formBody("/BBox [0 0 1 1] /Matrix [1 2 3]", "0 0 m 1 0 l S")},
		{name: "form type", body: formBody("/FormType 2 /BBox [0 0 1 1]", "0 0 m 1 0 l S")},
		{name: "not a stream", body: notStream},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>", testCase.body)
			pixmap := graphics.NewPixmap(pageSide, pageSide)
			err := file.PaintPage(t.Context(), 0, pixmap, 1)
			wantJobErr(t, err, "Do", nameUndefined)
		})
	}
}

// TestPaintFormResources proves a form runs with its own /Resources and, when
// it has none, with the page's resources.
func TestPaintFormResources(t *testing.T) {
	t.Run("local", checkFormLocalResources)
	t.Run("inherited", checkFormInheritedResources)
	t.Run("unknown local name", checkFormUnknownLocal)
}

func checkFormLocalResources(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/BBox [0 0 2 2] /Resources << /XObject << /Im0 6 0 R >> >>",
			"2 0 0 2 0 0 cm /Im0 Do"),
		rgbImageBody(t))
	checkFormRGB(t, file)
}

func checkFormInheritedResources(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R /Im0 6 0 R >> >>",
		formBody("/BBox [0 0 2 2]", "2 0 0 2 0 0 cm /Im0 Do"),
		rgbImageBody(t))
	checkFormRGB(t, file)
}

func checkFormRGB(t *testing.T, file *File) {
	t.Helper()
	pixmap := graphics.NewPixmap(2, 2)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 0, 0, 255, 0, 0)
	wantPixel(t, img, 1, 0, 0, 255, 0)
	wantPixel(t, img, 0, 1, 0, 0, 255)
	wantPixel(t, img, 1, 1, whiteByte, whiteByte, whiteByte)
}

func checkFormUnknownLocal(t *testing.T) {
	t.Helper()
	file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
		formBody("/BBox [0 0 1 1] /Resources << /XObject << /Other 6 0 R >> >>", "/Nope Do"),
		rgbImageBody(t))
	pixmap := graphics.NewPixmap(2, 2)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, "Do", nameUndefined)
}

// TestPaintFormRecursion proves nested form execution stops at the depth cap
// with limitcheck.
func TestPaintFormRecursion(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R >> >>",
			formBody("/BBox [0 0 1 1]", "/Fm0 Do"))
		checkFormLimit(t, file)
	})
	t.Run("mutual", func(t *testing.T) {
		file := doPage(t, "/Fm0 Do", "<< /XObject << /Fm0 5 0 R /Fm1 6 0 R >> >>",
			formBody("/BBox [0 0 1 1]", "/Fm1 Do"),
			formBody("/BBox [0 0 1 1]", "/Fm0 Do"))
		checkFormLimit(t, file)
	})
}

func checkFormLimit(t *testing.T, file *File) {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := file.PaintPage(t.Context(), 0, pixmap, 1)
	wantJobErr(t, err, "Do", nameLimit)
	img := shown(t, pixmap)
	if !rowWhite(img, 0) {
		t.Fatal("a refused form painted pixels")
	}
}
