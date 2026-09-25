package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestPaintBlendModeNames maps the 12 separable /BM names to the device enum
// and refuses the four non-separable modes by name.
func TestPaintBlendModeNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mode graphics.BlendMode
	}{
		{name: "Normal", mode: graphics.BlendNormal},
		{name: "Multiply", mode: graphics.BlendMultiply},
		{name: "Screen", mode: graphics.BlendScreen},
		{name: "Overlay", mode: graphics.BlendOverlay},
		{name: "Darken", mode: graphics.BlendDarken},
		{name: "Lighten", mode: graphics.BlendLighten},
		{name: "ColorDodge", mode: graphics.BlendColorDodge},
		{name: "ColorBurn", mode: graphics.BlendColorBurn},
		{name: "HardLight", mode: graphics.BlendHardLight},
		{name: "SoftLight", mode: graphics.BlendSoftLight},
		{name: "Difference", mode: graphics.BlendDifference},
		{name: "Exclusion", mode: graphics.BlendExclusion},
	}
	for _, testCase := range cases {
		mode, ok := blendModeName(testCase.name)
		if !ok || mode != testCase.mode {
			t.Fatalf("%s = %v, %v want %v", testCase.name, mode, ok, testCase.mode)
		}
	}
	for _, name := range []string{"Hue", "Saturation", "Color", "Luminosity", "Compatible", "Nope"} {
		if _, ok := blendModeName(name); ok {
			t.Fatalf("%s resolved to a separable mode", name)
		}
	}
}
