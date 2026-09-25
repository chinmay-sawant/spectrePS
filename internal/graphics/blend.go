package graphics

import "math"

// BlendMode is one separable blend mode from ISO 32000-1 table 136. The
// non-separable modes (Hue, Saturation, Color, and Luminosity) are refused at
// the PDF layer and have no constant here.
type BlendMode uint8

const (
	// BlendNormal keeps the source color, so out = src*a + dst*(1-a).
	BlendNormal BlendMode = iota
	// BlendMultiply is dst*src.
	BlendMultiply
	// BlendScreen is dst + src - dst*src.
	BlendScreen
	// BlendOverlay is HardLight with the operands swapped.
	BlendOverlay
	// BlendDarken is min(dst, src).
	BlendDarken
	// BlendLighten is max(dst, src).
	BlendLighten
	// BlendColorDodge brightens the backdrop by the source.
	BlendColorDodge
	// BlendColorBurn darkens the backdrop by the source.
	BlendColorBurn
	// BlendHardLight is Multiply or Screen on the source's side of one half.
	BlendHardLight
	// BlendSoftLight is the ISO 32000-1 soft-light formula.
	BlendSoftLight
	// BlendDifference is abs(dst-src).
	BlendDifference
	// BlendExclusion is dst + src - 2*dst*src.
	BlendExclusion
)

// The blend formulas split the unit interval at one half and one quarter, and
// the soft-light shape uses the ISO 32000-1 polynomial coefficients.
const (
	blendHalf    = 0.5
	blendQuarter = 0.25
	blendTwo     = 2
	blendFour    = 4
	blendTwelve  = 12
	blendSixteen = 16
)

// AlphaMarker is implemented by devices that composite marks with a constant
// alpha and a blend mode. It sits beside the narrow Marker interface, so a
// device that cannot write alpha, such as the rewrite recorder, simply does
// not implement it. A Pixmap does.
type AlphaMarker interface {
	// SetFillAlpha sets the fill mark alpha, clamped to 0 through 1.
	SetFillAlpha(alpha float64)
	// SetStrokeAlpha sets the stroke mark alpha, clamped to 0 through 1.
	SetStrokeAlpha(alpha float64)
	// SetBlendMode selects the blend function marks composite with. An
	// unknown mode composites as Normal.
	SetBlendMode(mode BlendMode)
}

// SoftMaskMarker is implemented by devices that clip later marks with a
// per-pixel coverage plane. It sits beside AlphaMarker, so a device that
// cannot mask, such as the rewrite recorder, simply does not implement it. A
// Pixmap does.
type SoftMaskMarker interface {
	// SetSoftMask installs the coverage plane, row 0 at the top, one byte per
	// page pixel. A nil or wrong-sized plane clears the mask.
	SetSoftMask(mask []byte)
}

// GroupMarker is implemented by devices that host a transparency group or a
// state soft mask. The scratch page has the page pixel size, and the group is
// composited once when its content has finished.
type GroupMarker interface {
	// PageSize returns the device page in pixels.
	PageSize() (int, int)
	// CompositeGroup composites a rendered group with its alpha and blend mode.
	CompositeGroup(group *Pixmap, alpha float64, mode BlendMode)
	// CompositeGroupClipped composites a group through the active clips.
	CompositeGroupClipped(clips []Clip, group *Pixmap, alpha float64, mode BlendMode)
}

var (
	_ AlphaMarker    = (*Pixmap)(nil)
	_ SoftMaskMarker = (*Pixmap)(nil)
	_ GroupMarker    = (*Pixmap)(nil)
)

// SetFillAlpha sets the constant alpha for fill and glyph marks.
func (p *Pixmap) SetFillAlpha(alpha float64) {
	p.fillAlpha = clampUnit(alpha)
}

// SetStrokeAlpha sets the constant alpha for stroke marks.
func (p *Pixmap) SetStrokeAlpha(alpha float64) {
	p.strokeAlpha = clampUnit(alpha)
}

// SetBlendMode selects the blend function fill and stroke marks composite with.
// The previous mode stays in effect until this is called again. A device state
// save and restore is the caller's job.
func (p *Pixmap) SetBlendMode(mode BlendMode) {
	p.blendMode = mode
}

// paint composites one mark pixel: the blend function of the backdrop and the
// source color, then the constant alpha. Each channel is
// blend(src, dst)*alpha + dst*(1-alpha), rounded to the nearest byte. An
// active soft mask multiplies the alpha, and the mark updates the coverage
// plane a group composite reads.
func (p *Pixmap) paint(col, row int, red, green, blue, alpha float64) {
	if col < 0 || row < 0 || col >= p.w || row >= p.h {
		return
	}
	alpha = p.maskedAlpha(col, row, alpha)
	if alpha <= 0 {
		return
	}
	p.markAlpha(col, row, alpha)
	if alpha >= 1 && p.blendMode == BlendNormal {
		p.set(col, row, colorByte(red), colorByte(green), colorByte(blue))
		return
	}
	offset := row*p.w*bytesPerPixel + col*bytesPerPixel
	p.pix[offset] = compositeByte(p.pix[offset], red, p.blendMode, alpha)
	p.pix[offset+1] = compositeByte(p.pix[offset+1], green, p.blendMode, alpha)
	p.pix[offset+2] = compositeByte(p.pix[offset+2], blue, p.blendMode, alpha)
}

// maskedAlpha multiplies one mark alpha by the active soft mask coverage.
func (p *Pixmap) maskedAlpha(col, row int, alpha float64) float64 {
	if p.softMask == nil {
		return alpha
	}
	return alpha * float64(p.softMask[row*p.w+col]) / colorScale
}

// markAlpha composites one mark alpha into the coverage plane with
// a + dst*(1-a), so overlapping marks accumulate coverage toward 255.
func (p *Pixmap) markAlpha(col, row int, alpha float64) {
	offset := row*p.w + col
	backdrop := float64(p.alpha[offset]) / colorScale
	out := alpha + backdrop*(1-alpha)
	if out >= 1 {
		p.alpha[offset] = whiteByte
		return
	}
	p.alpha[offset] = byte(math.Round(out * colorScale))
}

// CompositeGroup composites one rendered transparency group onto the page.
// coverage is the group alpha times the coverage plane, so an untouched group
// pixel changes nothing. Each channel is
// blend(src, dst)*coverage + dst*(1-coverage), rounded.
func (p *Pixmap) CompositeGroup(group *Pixmap, alpha float64, mode BlendMode) {
	if group == nil {
		return
	}
	alpha = clampUnit(alpha)
	if alpha <= 0 {
		return
	}
	width := min(p.w, group.w)
	height := min(p.h, group.h)
	for row := range height {
		for col := range width {
			coverage := alpha * float64(group.alpha[row*group.w+col]) / colorScale
			if p.softMask != nil {
				coverage *= float64(p.softMask[row*p.w+col]) / colorScale
			}
			if coverage <= 0 {
				continue
			}
			offset := row*group.w*bytesPerPixel + col*bytesPerPixel
			for part := range bytesPerPixel {
				dst := row*p.w*bytesPerPixel + col*bytesPerPixel + part
				src := float64(group.pix[offset+part]) / colorScale
				p.pix[dst] = compositeByte(p.pix[dst], src, mode, coverage)
			}
			p.markAlpha(col, row, coverage)
		}
	}
}

// compositeByte blends one channel with alpha, rounded to a byte.
func compositeByte(dst byte, src float64, mode BlendMode, alpha float64) byte {
	backdrop := float64(dst) / colorScale
	blended := blendChannel(backdrop, clampUnit(src), mode)
	out := blended*alpha + backdrop*(1-alpha)
	return byte(math.Round(out * colorScale))
}

// blendFunctions maps each separable mode to its ISO 32000-1 formula.
//
//nolint:gochecknoglobals // the ISO 32000-1 blend table is fixed
var blendFunctions = map[BlendMode]func(backdrop, source float64) float64{
	BlendNormal: func(_, source float64) float64 {
		return source
	},
	BlendMultiply: func(backdrop, source float64) float64 {
		return backdrop * source
	},
	BlendScreen: func(backdrop, source float64) float64 {
		return backdrop + source - backdrop*source
	},
	BlendOverlay:    overlayChannel,
	BlendDarken:     math.Min,
	BlendLighten:    math.Max,
	BlendColorDodge: colorDodgeChannel,
	BlendColorBurn:  colorBurnChannel,
	BlendHardLight:  hardLightChannel,
	BlendSoftLight:  softLightChannel,
	BlendDifference: func(backdrop, source float64) float64 {
		return math.Abs(backdrop - source)
	},
	BlendExclusion: func(backdrop, source float64) float64 {
		return backdrop + source - blendTwo*backdrop*source
	},
}

// blendChannel applies one separable blend function to normalized channels.
// The backdrop and source run 0 through 1, and an unknown mode is Normal.
func blendChannel(backdrop, source float64, mode BlendMode) float64 {
	blend, ok := blendFunctions[mode]
	if !ok {
		return source
	}
	return blend(backdrop, source)
}

// overlayChannel is HardLight with the operands swapped.
func overlayChannel(backdrop, source float64) float64 {
	if backdrop <= blendHalf {
		return blendTwo * backdrop * source
	}
	return 1 - blendTwo*(1-backdrop)*(1-source)
}

func colorDodgeChannel(backdrop, source float64) float64 {
	if backdrop <= 0 {
		return 0
	}
	if source >= 1 {
		return 1
	}
	return math.Min(1, backdrop/(1-source))
}

func colorBurnChannel(backdrop, source float64) float64 {
	if backdrop >= 1 {
		return 1
	}
	if source <= 0 {
		return 0
	}
	return 1 - math.Min(1, (1-backdrop)/source)
}

func hardLightChannel(backdrop, source float64) float64 {
	if source <= blendHalf {
		return blendTwo * backdrop * source
	}
	return 1 - blendTwo*(1-backdrop)*(1-source)
}

func softLightChannel(backdrop, source float64) float64 {
	if source <= blendHalf {
		return backdrop - (1-blendTwo*source)*backdrop*(1-backdrop)
	}
	shaped := math.Sqrt(backdrop)
	if backdrop <= blendQuarter {
		shaped = ((blendSixteen*backdrop-blendTwelve)*backdrop + blendFour) * backdrop
	}
	return backdrop + (blendTwo*source-1)*(shaped-backdrop)
}

// clampUnit limits one alpha or color value to 0 through 1.
func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
