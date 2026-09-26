package pdf

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestValidationUnsupportedOps locks the content operators the interpreter
// still refuses. The plan's literal refusal list, W, W*, B, B*, b, b*, BX,
// EX, gs, K, k, CS, cs, SC, sc, SCN, scn, sh, BMC, BDC, and EMC, is
// superseded by the landed clip, fill-and-stroke, compatibility, graphics
// state, color, and marked-content features, so all of those run except sh.
// d and Tr left the set as documented no-ops. Every remaining operator returns
// undefined with its own name and leaves a white page.
func TestValidationUnsupportedOps(t *testing.T) {
	t.Run("refused per operator", checkUnsupportedOps)
	t.Run("landed operators accept", checkLandedOperators)
	t.Run("gs benign entries", checkExtGStateBenign)
	t.Run("d and Tr shapes", checkDashAndRenderMode)
}

// checkUnsupportedOps proves each still-refused operator names itself and
// paints nothing.
func checkUnsupportedOps(t *testing.T) {
	t.Helper()
	cases := []struct {
		name   string
		src    string
		opName string
	}{
		{name: "inline image BI", src: "BI", opName: "BI"},
		{name: "inline image ID", src: "ID", opName: "ID"},
		{name: "inline image EI", src: "EI", opName: "EI"},
		{name: "shading sh", src: "sh", opName: "sh"},
		{name: "curve v", src: "0 0 1 1 0 0 v", opName: "v"},
		{name: "curve y", src: "0 0 1 1 0 0 y", opName: "y"},
		{name: "fill synonym F", src: "F", opName: "F"},
		{name: "type 3 d0", src: "0 0 d0", opName: "d0"},
		{name: "type 3 d1", src: "0 0 0 0 0 0 d1", opName: "d1"},
		{name: "pattern space CS", src: "/Pattern CS", opName: "CS"},
		{name: "pattern space cs", src: "/Pattern cs", opName: "cs"},
		{name: "pattern color SCN", src: "1 0 0 RG /P1 SCN", opName: "SCN"},
		{name: "pattern color scn", src: "1 0 0 rg /P1 scn", opName: "scn"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pixmap := graphics.NewPixmap(2, 2)
			err := Paint(t.Context(), []byte(testCase.src), pixmap, 1)
			wantJobErr(t, err, testCase.opName, nameUndefined)
			assertWhitePage(t, pixmap)
		})
	}
}

// checkLandedOperators runs the plan's superseded list in one stream. Every
// operator resolves, and the trailing stroke proves painting still reaches the
// pixmap.
func checkLandedOperators(t *testing.T) {
	t.Helper()
	state, _, err := ParseValue([]byte("<< /Type /ExtGState /LW 2 >>"), 0)
	if err != nil {
		t.Fatal(err)
	}
	src := "1 J 1 j 10 M 5 i /AbsoluteColorimetric ri [] 0 d 0 Tr " +
		"0 0 0 1 k 0 0 0 1 K " +
		"/DeviceCMYK CS 0 0 0 1 SC /DeviceCMYK cs 0 0 0 1 sc " +
		"0 0 0 1 SCN 0 0 0 1 scn " +
		"0 0 20 20 re W n " +
		"0 0 20 20 re B 0 0 10 10 re B* 0 0 m 15 0 l b 1 0 m 15 0 l b* " +
		"BX (ignored) Tj EX /Tag BMC EMC /Tag << /MCID 0 >> BDC EMC " +
		"/GS0 gs 0 0 0 RG 0 0 m 10 0 l S"
	res := Resources{ExtGStates: map[string]Value{"GS0": state}}
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := PaintWith(t.Context(), []byte(src), pixmap, 1,
		PaintOptions{Resources: res}); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if rowWhite(img, img.Height-1) {
		t.Fatal("the landed operators left a white page")
	}
}

// checkExtGStateBenign proves the CMYK JPEG page's ExtGState paints: the
// default-off booleans, /OPM, /SA, /Type, and /SMask /None are no-ops, while a
// true overprint or alpha-is-shape flag and any other entry still refuse.
func checkExtGStateBenign(t *testing.T) {
	t.Helper()
	t.Run("CMYK JPEG entry", func(t *testing.T) {
		body := "<< /AIS false /BM /Normal /CA 1.0 /OP false /OPM 1 /SA true " +
			"/SMask /None /Type /ExtGState /ca 1.0 /op false >>"
		file := extGStatePage(t, "/GS0 gs 0 0 m 10 0 l S", body)
		if rowWhite(paintPage(t, file), pageSide-1) {
			t.Fatal("the CMYK JPEG ExtGState painted no stroke")
		}
	})
	t.Run("refusals", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{name: "op true", body: "<< /Type /ExtGState /op true >>"},
			{name: "OP true", body: "<< /Type /ExtGState /OP true >>"},
			{name: "AIS true", body: "<< /Type /ExtGState /AIS true >>"},
			{name: "OPM 2", body: "<< /Type /ExtGState /OPM 2 >>"},
			{name: "OPM name", body: "<< /Type /ExtGState /OPM /One >>"},
			{name: "SA number", body: "<< /Type /ExtGState /SA 1 >>"},
			{name: "unknown entry", body: "<< /Type /ExtGState /HT /Nope >>"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				file := extGStatePage(t, "/GS0 gs", testCase.body)
				err := file.PaintPage(t.Context(), 0, graphics.NewPixmap(pageSide, pageSide), 1)
				wantJobErr(t, err, "gs", nameUndefined)
			})
		}
	})
}

// checkDashAndRenderMode proves d and Tr are no-ops that still check their
// operand shape, the same way J, j, M, i, and ri do.
func checkDashAndRenderMode(t *testing.T) {
	t.Helper()
	t.Run("capsule", func(t *testing.T) {
		noops := paintImage(t, "[3 2] 0 d 3 Tr 0 0 m 10 0 l S")
		plain := paintImage(t, "0 0 m 10 0 l S")
		if !bytes.Equal(noops.Pixels, plain.Pixels) {
			t.Fatal("d or Tr changed the capsule stroke")
		}
	})
	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			src     string
			opName  string
			errName string
		}{
			{src: "d", opName: "d", errName: nameUnderflow},
			{src: "0 d", opName: "d", errName: nameUnderflow},
			{src: "0 (not an array) d", opName: "d", errName: nameType},
			{src: "[] (not a number) d", opName: "d", errName: nameType},
			{src: "Tr", opName: "Tr", errName: nameUnderflow},
			{src: "(filled) Tr", opName: "Tr", errName: nameType},
		}
		for _, testCase := range cases {
			t.Run(testCase.src, func(t *testing.T) {
				err := Paint(t.Context(), []byte(testCase.src), graphics.NewPixmap(2, 2), 1)
				wantJobErr(t, err, testCase.opName, testCase.errName)
			})
		}
	})
}

// assertWhitePage proves a refused stream left the pixmap untouched.
func assertWhitePage(t *testing.T, pixmap *graphics.Pixmap) {
	t.Helper()
	img := shown(t, pixmap)
	for row := range img.Height {
		if !rowWhite(img, row) {
			t.Fatalf("row %d is not white after a refusal", row)
		}
	}
}
