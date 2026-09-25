package graphics

import "testing"

// TestPixmapAlphaBlend composites fill and stroke marks with a constant alpha.
// out = src*a + dst*(1-a) per channel, rounded.
func TestPixmapAlphaBlend(t *testing.T) {
	t.Run("fill", func(t *testing.T) {
		pixmap := NewPixmap(2, 2)
		pixmap.SetFillAlpha(0.5)
		fillSquare(pixmap, 1, 0, 0)
		img := shownPixmap(t, pixmap)
		checkStampBlock(t, img, 0, 0, 1, 1, 255, 128, 128)
	})
	t.Run("stroke", func(t *testing.T) {
		pixmap := NewPixmap(4, 1)
		pixmap.SetStrokeAlpha(0.25)
		pixmap.Stroke(lineAcross(pixmap), 1, 1, 0, 0)
		img := shownPixmap(t, pixmap)
		checkStampBlock(t, img, 0, 0, 3, 0, 255, 191, 191)
	})
	t.Run("fill alpha does not touch stroke", func(t *testing.T) {
		pixmap := NewPixmap(4, 1)
		pixmap.SetFillAlpha(0.5)
		pixmap.Stroke(lineAcross(pixmap), 1, 1, 0, 0)
		img := shownPixmap(t, pixmap)
		checkStampBlock(t, img, 0, 0, 3, 0, 255, 0, 0)
	})
	t.Run("zero alpha leaves the page", func(t *testing.T) {
		pixmap := NewPixmap(1, 1)
		pixmap.SetFillAlpha(0)
		fillSquare(pixmap, 0, 0, 1)
		img := shownPixmap(t, pixmap)
		checkStampPixel(t, img, 0, 0, 255, 255, 255)
	})
	t.Run("alpha clamps", func(t *testing.T) {
		high := NewPixmap(1, 1)
		high.SetFillAlpha(2)
		fillSquare(high, 0, 1, 0)
		checkStampPixel(t, shownPixmap(t, high), 0, 0, 0, 255, 0)
		low := NewPixmap(1, 1)
		low.SetFillAlpha(-1)
		fillSquare(low, 0, 1, 0)
		checkStampPixel(t, shownPixmap(t, low), 0, 0, 255, 255, 255)
	})
}

// TestPixmapBlendMode locks one exact byte per separable blend mode over a
// backdrop of byte 200 and a source of byte 100. Multiply and Screen are the
// two the plan names.
func TestPixmapBlendMode(t *testing.T) {
	cases := []struct {
		name string
		mode BlendMode
		want byte
	}{
		{name: "normal", mode: BlendNormal, want: 100},
		{name: "multiply", mode: BlendMultiply, want: 78},
		{name: "screen", mode: BlendScreen, want: 222},
		{name: "overlay", mode: BlendOverlay, want: 188},
		{name: "darken", mode: BlendDarken, want: 100},
		{name: "lighten", mode: BlendLighten, want: 200},
		{name: "color dodge", mode: BlendColorDodge, want: 255},
		{name: "color burn", mode: BlendColorBurn, want: 115},
		{name: "hard light", mode: BlendHardLight, want: 157},
		{name: "soft light", mode: BlendSoftLight, want: 191},
		{name: "difference", mode: BlendDifference, want: 100},
		{name: "exclusion", mode: BlendExclusion, want: 143},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pixmap := NewPixmap(1, 1)
			fillSquare(pixmap, backdrop, backdrop, backdrop)
			pixmap.SetBlendMode(testCase.mode)
			fillSquare(pixmap, source, source, source)
			checkStampPixel(t, shownPixmap(t, pixmap), 0, 0,
				testCase.want, testCase.want, testCase.want)
		})
	}
	t.Run("multiply with alpha", func(t *testing.T) {
		pixmap := NewPixmap(1, 1)
		fillSquare(pixmap, backdrop, backdrop, backdrop)
		pixmap.SetBlendMode(BlendMultiply)
		pixmap.SetFillAlpha(0.5)
		fillSquare(pixmap, source, source, source)
		// round(255 * ((200*100/255) + 200) / (2*255)) = 139.
		checkStampPixel(t, shownPixmap(t, pixmap), 0, 0, 139, 139, 139)
	})
	t.Run("unknown mode is normal", func(t *testing.T) {
		pixmap := NewPixmap(1, 1)
		fillSquare(pixmap, backdrop, backdrop, backdrop)
		pixmap.SetBlendMode(BlendMode(200))
		fillSquare(pixmap, source, source, source)
		checkStampPixel(t, shownPixmap(t, pixmap), 0, 0, 100, 100, 100)
	})
}

// backdrop and source are the byte values the blend table is computed for.
const (
	backdrop = 200.0 / 255
	source   = 100.0 / 255
)

// fillSquare paints the whole pixmap with one color at the current alpha.
func fillSquare(pixmap *Pixmap, red, green, blue float64) {
	width := float64(pixmap.w)
	height := float64(pixmap.h)
	pixmap.Fill([]Point{
		{X: 0, Y: 0, Move: true},
		{X: width, Y: 0},
		{X: width, Y: height},
		{X: 0, Y: height},
	}, red, green, blue, false)
}

// lineAcross returns a one-pixel stroke across the bottom row.
func lineAcross(pixmap *Pixmap) []Point {
	return []Point{
		{X: 0, Y: 0.5, Move: true},
		{X: float64(pixmap.w), Y: 0.5},
	}
}
