package ps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/font"
)

const (
	fontNameKey = "FontName"
	fontSizeKey = "FontSize"

	opFindFont    = "findfont"
	opScaleFont   = "scalefont"
	opSetFont     = "setfont"
	opShow        = "show"
	opStringWidth = "stringwidth"

	errInvalidFont = "invalidfont"
	baseFontSize   = 1.0
	emScale        = 1000
)

// registerTextOps installs the standard 14 font operators. A font is a plain
// dictionary with /FontName and /FontSize, so findfont, scalefont, and
// setfont stay PostScript data.
func registerTextOps(interp *Interp) {
	interp.Install(opFindFont, opFindFontRun)
	interp.Install(opScaleFont, opScaleFontRun)
	interp.Install(opSetFont, opSetFontRun)
	interp.Install(opShow, opShowRun)
	interp.Install(opStringWidth, opStringWidthRun)
}

// opFindFontRun looks up one of the standard 14 names. Any other name is
// invalidfont, because Spectre ships no substitute outlines.
func opFindFontRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := popName(interp, opFindFont)
	if err != nil {
		return err
	}
	if _, ok := font.Standard14(name); !ok {
		return errOf(errInvalidFont, opFindFont)
	}
	dict := newDict(false)
	dict.putNew(fontNameKey, LiteralName(name))
	dict.putNew(fontSizeKey, RealObj(baseFontSize))
	return interp.Push(DictObj(dict))
}

// opScaleFontRun returns a font with /FontSize set to the popped size.
func opScaleFontRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	size, _, err := interp.PopNum()
	if err != nil {
		return err
	}
	dict, err := popFontDict(interp, opScaleFont)
	if err != nil {
		return err
	}
	scaled := newDict(false)
	scaled.putNew(fontNameKey, LiteralName(fontNameOf(dict)))
	scaled.putNew(fontSizeKey, RealObj(size))
	return interp.Push(DictObj(scaled))
}

// opSetFontRun makes one font the current font, saved and restored by gsave
// and grestore.
func opSetFontRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dict, err := popFontDict(interp, opSetFont)
	if err != nil {
		return err
	}
	size := baseFontSize
	if obj, ok := dict.get(fontSizeKey); ok {
		if val, okNum := textNumber(obj); okNum {
			size = val
		}
	}
	state := gsFor(interp)
	state.fontName = fontNameOf(dict)
	state.fontSize = size
	return nil
}

// opShowRun shows one string. The standard 14 fonts have metrics but no
// outline source, so a device run returns invalidfont, the policy in
// documentation/fonts.md. Without a device the current point still advances.
func opShowRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if obj.Kind != KindString || obj.Str == nil {
		return errOf(errTypeCheck, opShow)
	}
	state := gsFor(interp)
	metrics, ok := font.Standard14(state.fontName)
	if !ok {
		return errOf(errInvalidFont, opShow)
	}
	if err := state.requirePoint(opShow); err != nil {
		return err
	}
	if state.pix != nil {
		return errOf(errInvalidFont, opShow)
	}
	for _, code := range obj.Str.Bytes {
		advance := float64(glyphWidth(metrics, code)) / emScale * state.fontSize
		state.moveBy(advance)
	}
	return nil
}

// glyphWidth is the advance in 1/1000 em from the StandardEncoding name.
func glyphWidth(metrics *font.Metrics, code byte) font.Width {
	name := font.EncodingStandard.GlyphName(code)
	if name == "" {
		return 0
	}
	width, _ := metrics.WidthByName(name)
	return width
}

// opStringWidthRun pushes the user-space width and the vertical displacement
// of one string in the current font and size. The standard 14 are horizontal,
// so the second value is 0. stringwidth does not move the current point and
// does not need one.
func opStringWidthRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if obj.Kind != KindString || obj.Str == nil {
		return errOf(errTypeCheck, opStringWidth)
	}
	state := gsFor(interp)
	metrics, ok := font.Standard14(state.fontName)
	if !ok {
		return errOf(errInvalidFont, opStringWidth)
	}
	width := 0.0
	for _, code := range obj.Str.Bytes {
		width += float64(glyphWidth(metrics, code)) / emScale * state.fontSize
	}
	if err := interp.Push(RealObj(width)); err != nil {
		return err
	}
	return interp.Push(RealObj(0))
}

// popFontDict pops one font dictionary. Anything else is invalidfont.
func popFontDict(interp *Interp, opName string) (*Dict, error) {
	obj, err := interp.Pop()
	if err != nil {
		return nil, err
	}
	if obj.Kind != KindDict || obj.Dict == nil {
		return nil, errOf(errInvalidFont, opName)
	}
	if _, ok := obj.Dict.get(fontNameKey); !ok {
		return nil, errOf(errInvalidFont, opName)
	}
	return obj.Dict, nil
}

func fontNameOf(dict *Dict) string {
	obj, _ := dict.get(fontNameKey)
	return obj.Name
}

// textNumber returns an int or real as a float64.
func textNumber(obj Object) (float64, bool) {
	if obj.Kind == KindInt {
		return float64(obj.Int), true
	}
	if obj.Kind == KindReal {
		return obj.Real, true
	}
	return 0, false
}

// moveBy advances the device current point along the x axis of the current
// coordinate system and starts a new subpath there.
func (state *gstate) moveBy(delta float64) {
	state.devX += state.ctm.a * delta
	state.devY += state.ctm.b * delta
	state.hasPt = true
	state.path = append(state.path, devPt{x: state.devX, y: state.devY, move: true})
}
