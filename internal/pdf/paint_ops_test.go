package pdf

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestPaintCompatSection proves BX skips everything through the matching EX
// and that an unterminated section is syntaxerror in content.
func TestPaintCompatSection(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		img := paintImage(t, "BX /Nope Do (EX) Tj [ nonsense ] Tj EX 0 0 m 10 0 l S")
		if rowWhite(img, img.Height-1) {
			t.Fatal("the section ate the trailing stroke")
		}
	})
	t.Run("nested", func(t *testing.T) {
		img := paintImage(t, "BX"+" BX EX"+" EX 0 0 m 10 0 l S")
		if rowWhite(img, img.Height-1) {
			t.Fatal("the nested section ate the trailing stroke")
		}
	})
	t.Run("ex alone", func(t *testing.T) {
		img := paintImage(t, "EX 0 0 m 10 0 l S")
		if rowWhite(img, img.Height-1) {
			t.Fatal("an unmatched EX failed the page")
		}
	})
	t.Run("unterminated", func(t *testing.T) {
		err := Paint(t.Context(), []byte("BX 999"), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, syntaxOp, errSyntax)
		err = Paint(t.Context(), []byte("BX"), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, syntaxOp, errSyntax)
	})
}

// TestPaintFillStroke proves B, B*, b, and b* fill and stroke one path, with
// the even-odd rule for the starred forms and a close for b and b*.
func TestPaintFillStroke(t *testing.T) {
	t.Run("B no close", func(t *testing.T) {
		img := paintImage(t, "4 w 0 0 0 rg 5 5 m 15 5 l 15 15 l 5 15 l B")
		wantPixel(t, img, 7, 10, 0, 0, 0)
		wantPixel(t, img, 3, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("b closes", func(t *testing.T) {
		img := paintImage(t, "4 w 0 0 0 rg 5 5 m 15 5 l 15 15 l 5 15 l b")
		wantPixel(t, img, 7, 10, 0, 0, 0)
		wantPixel(t, img, 3, 10, 0, 0, 0)
	})
	t.Run("B* even-odd", func(t *testing.T) {
		img := paintImage(t, "0 0 20 20 re 5 5 10 10 re B*")
		wantPixel(t, img, 2, 10, 0, 0, 0)
		wantPixel(t, img, 10, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("b* even-odd", func(t *testing.T) {
		img := paintImage(t, "0 0 20 20 re 5 5 10 10 re b*")
		wantPixel(t, img, 2, 10, 0, 0, 0)
		wantPixel(t, img, 10, 10, whiteByte, whiteByte, whiteByte)
	})
	t.Run("empty B", func(t *testing.T) {
		if err := Paint(t.Context(), []byte("B"), graphics.NewPixmap(2, 2), 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("no point", func(t *testing.T) {
		err := Paint(t.Context(), []byte("b"), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "b", nameNoPoint)
		err = Paint(t.Context(), []byte("b*"), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "b*", nameNoPoint)
	})
}

// TestPaintExtGStateResource proves gs resolves /ExtGState from /Resources,
// applies /LW, restores on Q, and refuses an unknown name or an unsupported
// entry with undefined in gs.
func TestPaintExtGStateResource(t *testing.T) {
	t.Run("width", checkExtGStateWidth)
	t.Run("restore", checkExtGStateRestore)
	t.Run("line params", checkExtGStateParams)
	t.Run("errors", checkExtGStateErrors)
}

func extGStatePage(t *testing.T, content, body string) *File {
	t.Helper()
	return doPage(t, content, "<< /ExtGState << /GS0 5 0 R >> >>", body)
}

func checkExtGStateWidth(t *testing.T) {
	t.Helper()
	file := extGStatePage(t, "q /GS0 gs 0 0 m 10 0 l S Q", "<< /Type /ExtGState /LW 8 >>")
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 5, 16, 0, 0, 0)
	wantPixel(t, img, 5, 15, whiteByte, whiteByte, whiteByte)
}

func checkExtGStateRestore(t *testing.T) {
	t.Helper()
	file := extGStatePage(t, "1 w q /GS0 gs Q 0 0 m 10 0 l S", "<< /Type /ExtGState /LW 8 >>")
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 5, 19, 0, 0, 0)
	wantPixel(t, img, 5, 16, whiteByte, whiteByte, whiteByte)
}

func checkExtGStateParams(t *testing.T) {
	t.Helper()
	body := "<< /Type /ExtGState /LC 1 /LJ 1 /ML 4 /RI /AbsoluteColorimetric >>"
	file := extGStatePage(t, "/GS0 gs 0 0 m 10 0 l S", body)
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 5, 19, 0, 0, 0)
	wantPixel(t, img, 5, 16, whiteByte, whiteByte, whiteByte)
}

func checkExtGStateErrors(t *testing.T) {
	t.Helper()
	cases := []struct {
		name    string
		content string
		body    string
		opName  string
		errName string
	}{
		{name: "unknown name", content: "/Nope gs", body: "<< /Type /ExtGState /LW 8 >>",
			opName: "gs", errName: nameUndefined},
		{name: "unsupported entry", content: "/GS0 gs", body: "<< /Type /ExtGState /HT /Nope >>",
			opName: "gs", errName: nameUndefined},
		{name: "not a dict", content: "/GS0 gs", body: "[1 2 3]",
			opName: "gs", errName: nameUndefined},
		{name: "bad width", content: "/GS0 gs", body: "<< /Type /ExtGState /LW (wide) >>",
			opName: "gs", errName: nameType},
		{name: "underflow", content: "gs", body: "<< /Type /ExtGState /LW 8 >>",
			opName: "gs", errName: nameUnderflow},
		{name: "type", content: "1 gs", body: "<< /Type /ExtGState /LW 8 >>",
			opName: "gs", errName: nameType},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			file := extGStatePage(t, testCase.content, testCase.body)
			err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
			wantJobErr(t, err, testCase.opName, testCase.errName)
		})
	}
}

// TestPaintLineParams proves J, j, M, i, and ri are accepted no-ops under the
// capsule stroke and that bad operands still refuse.
func TestPaintLineParams(t *testing.T) {
	t.Run("capsule", func(t *testing.T) {
		params := paintImage(t, "1 J 1 j 10 M 5 i /AbsoluteColorimetric ri 0 0 m 10 0 l S")
		plain := paintImage(t, "0 0 m 10 0 l S")
		if !bytes.Equal(params.Pixels, plain.Pixels) {
			t.Fatal("line parameters changed the capsule stroke")
		}
	})
	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			src     string
			opName  string
			errName string
		}{
			{src: "(x) J", opName: "J", errName: nameType},
			{src: "j", opName: "j", errName: nameUnderflow},
			{src: "M", opName: "M", errName: nameUnderflow},
			{src: "i", opName: "i", errName: nameUnderflow},
			{src: "ri", opName: "ri", errName: nameUnderflow},
			{src: "1 ri", opName: "ri", errName: nameType},
		}
		for _, testCase := range cases {
			t.Run(testCase.src, func(t *testing.T) {
				err := Paint(t.Context(), []byte(testCase.src), graphics.NewPixmap(2, 2), 1)
				wantJobErr(t, err, testCase.opName, testCase.errName)
			})
		}
	})
}

// paintImage paints one content stream on a blank page and returns the image.
func paintImage(t *testing.T, src string) graphics.Image {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := Paint(t.Context(), []byte(src), pixmap, 1); err != nil {
		t.Fatal(err)
	}
	return shown(t, pixmap)
}
