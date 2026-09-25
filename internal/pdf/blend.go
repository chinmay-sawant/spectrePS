package pdf

import "github.com/chinmay-sawant/spectrePS/internal/graphics"

// blendModeNames maps the separable /BM names to the device enum. The four
// non-separable modes and any other name are absent, so the caller refuses
// them by name.
//
//nolint:gochecknoglobals // the ISO 32000-1 blend name table is fixed
var blendModeNames = map[string]graphics.BlendMode{
	"Normal":     graphics.BlendNormal,
	"Multiply":   graphics.BlendMultiply,
	"Screen":     graphics.BlendScreen,
	"Overlay":    graphics.BlendOverlay,
	"Darken":     graphics.BlendDarken,
	"Lighten":    graphics.BlendLighten,
	"ColorDodge": graphics.BlendColorDodge,
	"ColorBurn":  graphics.BlendColorBurn,
	"HardLight":  graphics.BlendHardLight,
	"SoftLight":  graphics.BlendSoftLight,
	"Difference": graphics.BlendDifference,
	"Exclusion":  graphics.BlendExclusion,
}

// blendModeName maps one /BM name to the device blend mode. The four
// non-separable modes (Hue, Saturation, Color, and Luminosity) and any other
// name return false, so the caller refuses them by name instead of painting a
// wrong blend.
func blendModeName(name string) (graphics.BlendMode, bool) {
	mode, ok := blendModeNames[name]
	return mode, ok
}
