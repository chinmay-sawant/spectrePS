package pdf

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	nameUndefined = "undefined"
	nameLimit     = "limitcheck"
	nameNoPoint   = "nocurrentpoint"
	nameUnderflow = "stackunderflow"
	nameType      = "typecheck"
	rgbBytes      = 3
	whiteByte     = 255
	pageSide      = 20
)

func TestPaintLineBottom(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("0 0 m 10 0 l S"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if !rowWhite(img, 0) {
		t.Fatal("row 0 is not white")
	}
	bottom := img.Height - 1
	if rowWhite(img, bottom) {
		t.Fatal("bottom row is white")
	}
	wantPixel(t, img, 0, bottom, 0, 0, 0)
}

func TestPaintRedRect(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("1 0 0 rg 0 0 10 10 re f"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	wantPixel(t, img, 5, 15, whiteByte, 0, 0)
	wantPixel(t, img, 0, 0, whiteByte, whiteByte, whiteByte)
	wantPixel(t, img, 15, 15, whiteByte, whiteByte, whiteByte)
}

// TestContentScannerNameText locks the seam that carries name text to the
// operand stack, so Do and Tf can read name operands.
func TestContentScannerNameText(t *testing.T) {
	lex := scanner{src: []byte("/Im Do"), pos: 0}
	tok, ok, err := lex.next()
	if err != nil || !ok {
		t.Fatalf("next: ok=%v err=%v", ok, err)
	}
	if tok.kind != ctokName || tok.text != "Im" {
		t.Fatalf("name token = %+v", tok)
	}
	run := newRunner(nil, 1)
	if err := run.take(t.Context(), tok); err != nil {
		t.Fatal(err)
	}
	name, err := run.popName("Do")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Im" {
		t.Fatalf("popName = %q", name)
	}
}

// TestPaintNameOperand locks the name seam: the scanner carries name text to
// the operand stack and popName returns it. A non-name operand is typecheck
// and a missing operand is stackunderflow, both with the Do operator name.
func TestPaintNameOperand(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		errName string
	}{
		{name: "name", src: "/Im", want: "Im"},
		{name: "missing", errName: nameUnderflow},
		{name: "number", src: "1", errName: nameType},
		{name: "string", src: "(Hi)", errName: nameType},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			run := newRunner(nil, 1)
			lex := scanner{src: []byte(tt.src), pos: 0}
			for {
				tok, ok, err := lex.next()
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					break
				}
				if err := run.take(t.Context(), tok); err != nil {
					t.Fatal(err)
				}
			}
			got, err := run.popName("Do")
			if tt.errName != "" {
				wantJobErr(t, err, "Do", tt.errName)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("popName = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPaintUndefined(t *testing.T) {
	pixmap := graphics.NewPixmap(4, 4)
	cases := []struct {
		src    string
		opName string
		want   string
	}{
		{src: "(Hi) Tj", opName: "Tj", want: nameUndefined},
		{src: "/Im Do", opName: "Do", want: nameUndefined},
		{src: "1 Do", opName: "Do", want: nameType},
		{src: "<< /Im /X >> Do", opName: "Do", want: nameType},
		{src: "[ (Hi) 20 ] TJ", opName: "TJ", want: nameUndefined},
		{src: "(Hi) '", opName: "'", want: nameUndefined},
		{src: "1 1 (Hi) \"", opName: "\"", want: nameUndefined}, {src: "<4869> Tj", opName: "Tj", want: nameUndefined},
	}
	for _, tt := range cases {
		t.Run(tt.src, func(t *testing.T) {
			err := Paint(t.Context(), []byte(tt.src), pixmap, 1)
			wantJobErr(t, err, tt.opName, tt.want)
		})
	}
}

func TestPaintSaveLimit(t *testing.T) {
	pixmap := graphics.NewPixmap(2, 2)
	err := Paint(t.Context(), []byte(strings.Repeat("q ", 32)), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = Paint(t.Context(), []byte(strings.Repeat("q ", 33)), pixmap, 1)
	wantJobErr(t, err, "q", nameLimit)
	err = Paint(t.Context(), []byte("Q"), pixmap, 1)
	wantJobErr(t, err, "Q", nameLimit)
}

func TestPaintCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(ctx, []byte("0 0 m 10 0 l S"), pixmap, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	img := shown(t, pixmap)
	if !rowWhite(img, img.Height-1) {
		t.Fatal("canceled paint changed the bottom row")
	}
}

func TestPaintNilContext(t *testing.T) {
	defer func() {
		got := recover()
		text, ok := got.(string)
		if !ok || text != "pdf: nil context" {
			t.Fatalf("panic = %v, want pdf: nil context", got)
		}
	}()
	_ = Paint(nil, nil, nil, 1) //nolint:staticcheck // nil context is the case under test
}

func TestPaintNoCurrentPoint(t *testing.T) {
	pixmap := graphics.NewPixmap(2, 2)
	cases := []struct {
		src    string
		opName string
	}{
		{src: "0 0 l", opName: "l"},
		{src: "h", opName: "h"},
		{src: "s", opName: "s"},
		{src: "0 0 1 1 2 2 c", opName: "c"},
	}
	for _, tt := range cases {
		t.Run(tt.opName, func(t *testing.T) {
			err := Paint(t.Context(), []byte(tt.src), pixmap, 1)
			wantJobErr(t, err, tt.opName, nameNoPoint)
		})
	}
}

func TestPaintStackUnderflow(t *testing.T) {
	pixmap := graphics.NewPixmap(2, 2)
	cases := []struct {
		src    string
		opName string
	}{
		{src: "m", opName: "m"},
		{src: "0 m", opName: "m"},
		{src: "re", opName: "re"},
		{src: "cm", opName: "cm"},
		{src: "1 2 3 4 5 cm", opName: "cm"},
		{src: "w", opName: "w"},
		{src: "rg", opName: "rg"},
		{src: "G", opName: "G"},
		{src: "Do", opName: "Do"},
	}
	for _, tt := range cases {
		t.Run(tt.src, func(t *testing.T) {
			err := Paint(t.Context(), []byte(tt.src), pixmap, 1)
			wantJobErr(t, err, tt.opName, nameUnderflow)
		})
	}
}

func TestPaintRestore(t *testing.T) {
	t.Run("color", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		src := "1 0 0 rg q 0 g Q 0 0 10 10 re f"
		err := Paint(t.Context(), []byte(src), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		wantPixel(t, shown(t, pixmap), 5, 15, whiteByte, 0, 0)
	})
	t.Run("path", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		err := Paint(t.Context(), []byte("0 0 m 10 0 l q n Q S"), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		if !rowWhite(img, 0) {
			t.Fatal("row 0 is not white")
		}
		if rowWhite(img, img.Height-1) {
			t.Fatal("restored stroke missed the bottom row")
		}
	})
	t.Run("width", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		src := "10 w q 1 w Q 0 0 m 10 0 l S"
		err := Paint(t.Context(), []byte(src), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		wantPixel(t, shown(t, pixmap), 0, 15, 0, 0, 0)
	})
}

func TestCTM(t *testing.T) {
	t.Run("translate", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		err := Paint(t.Context(), []byte("1 0 0 1 3 4 cm 1 0 0 rg 0 0 5 5 re f"), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 5, 13, whiteByte, 0, 0)
		wantPixel(t, img, 10, 13, whiteByte, whiteByte, whiteByte)
	})
	t.Run("scale", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		err := Paint(t.Context(), []byte("2 0 0 2 0 0 cm 1 0 0 rg 0 0 5 5 re f"), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 5, 15, whiteByte, 0, 0)
		wantPixel(t, img, 15, 15, whiteByte, whiteByte, whiteByte)
	})
	t.Run("stroke width", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		err := Paint(t.Context(), []byte("4 0 0 4 0 0 cm 0 0 m 5 0 l S"), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		wantPixel(t, img, 5, 18, 0, 0, 0)
		wantPixel(t, img, 5, 17, whiteByte, whiteByte, whiteByte)
	})
	t.Run("restore", func(t *testing.T) {
		pixmap := graphics.NewPixmap(pageSide, pageSide)
		err := Paint(t.Context(), []byte("q 1 0 0 1 10 0 cm Q 0 0 m 10 0 l S"), pixmap, 1)
		if err != nil {
			t.Fatal(err)
		}
		img := shown(t, pixmap)
		bottom := img.Height - 1
		wantPixel(t, img, 5, bottom, 0, 0, 0)
		wantPixel(t, img, 15, bottom, whiteByte, whiteByte, whiteByte)
	})
}

func TestPaintCloseStroke(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("0 0 m 10 0 l 10 10 l 0 10 l s"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantPixel(t, shown(t, pixmap), 0, 15, 0, 0, 0)
}

func TestPaintEvenOdd(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("1 0 0 RG 0 0 10 10 re f*"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantPixel(t, shown(t, pixmap), 5, 15, whiteByte, 0, 0)
}

func TestPaintCurve(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("0 0 m 0 10 10 10 10 0 c S"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if !rowWhite(img, 0) {
		t.Fatal("row 0 is not white")
	}
	if rowWhite(img, img.Height-1) {
		t.Fatal("curve stroke missed the bottom row")
	}
}

func TestPaintEndPath(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	err := Paint(t.Context(), []byte("0 0 m 10 0 l n S"), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if !rowWhite(img, img.Height-1) {
		t.Fatal("n did not clear the path")
	}
}

func TestPaintComment(t *testing.T) {
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	src := "% bottom\n0 0 m\n10 0 l\nS\n"
	err := Paint(t.Context(), []byte(src), pixmap, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if !rowWhite(img, 0) || rowWhite(img, img.Height-1) {
		t.Fatal("commented stroke did not match the bottom line")
	}
}

func TestPaintEmpty(t *testing.T) {
	pixmap := graphics.NewPixmap(2, 2)
	if err := Paint(t.Context(), nil, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	if err := Paint(t.Context(), []byte(" \n% comment\n"), pixmap, 1); err != nil {
		t.Fatal(err)
	}
}

func TestPaintBadSyntax(t *testing.T) {
	pixmap := graphics.NewPixmap(2, 2)
	err := Paint(t.Context(), []byte("(Hi"), pixmap, 1)
	got, ok := jobErr(err)
	if !ok || got.Name != "syntaxerror" {
		t.Fatalf("error = %v, want syntaxerror", err)
	}
}

func shown(t *testing.T, pixmap *graphics.Pixmap) graphics.Image {
	t.Helper()
	if len(pixmap.Pages()) != 0 {
		t.Fatal("ShowPage ran before the snapshot")
	}
	pixmap.ShowPage()
	pages := pixmap.Pages()
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	return pages[0]
}

func rowWhite(img graphics.Image, row int) bool {
	start := row * img.Stride
	end := start + img.Width*rgbBytes
	for _, channel := range img.Pixels[start:end] {
		if channel != whiteByte {
			return false
		}
	}
	return true
}

func wantPixel(t *testing.T, img graphics.Image, col, row int, red, green, blue byte) {
	t.Helper()
	idx := row*img.Stride + col*rgbBytes
	gotR := img.Pixels[idx]
	gotG := img.Pixels[idx+1]
	gotB := img.Pixels[idx+2]
	if gotR == red && gotG == green && gotB == blue {
		return
	}
	t.Fatalf(
		"pixel (%d,%d) = %d,%d,%d, want %d,%d,%d",
		col, row, gotR, gotG, gotB, red, green, blue,
	)
}

func wantJobErr(t *testing.T, err error, opName, errName string) {
	t.Helper()
	got, ok := jobErr(err)
	if !ok || got.Name != errName || got.Op != opName {
		t.Fatalf("error = %v, want %s from %s", err, errName, opName)
	}
}

func jobErr(err error) (*Error, bool) {
	var got *Error
	if !errors.As(err, &got) {
		return nil, false
	}
	return got, true
}
