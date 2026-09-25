package pdf

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const (
	opShow       = "Tj"
	opShowArray  = "TJ"
	opShowQuote  = "'"
	opShowDQuote = "\""
	opTextFont   = "Tf"
	opTextBegin  = "BT"

	glyphPadding   = 1
	maxGlyphSide   = 20000
	maxGlyphPixels = 40000000
	percentScale   = 100
	fixedShift     = 64
	maxAlpha       = 255
	boxAscent      = 0.75
	boxDescent     = 0.25
)

// GlyphSink receives each positioned glyph the text machine shows.
type GlyphSink interface {
	// Glyph receives one shown character code.
	Glyph(g Glyph)
}

// Box is one device-space rectangle, origin at the lower left.
type Box struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// Glyph is one shown character.
type Glyph struct {
	// Code is the character code in the font's encoding.
	Code uint32
	// Unicode is the /ToUnicode or encoding result. It is empty when unknown.
	Unicode string
	// Advance is the horizontal text-space displacement of the code before
	// the text matrix, including character and word spacing.
	Advance float64
	// X and Y are the device-space glyph origin in pixels.
	X float64
	Y float64
	// Box is the device-space advance box. Its vertical extent is the
	// nominal ascent and descent of the text size, so building it never
	// needs an outline program.
	Box Box
}

// TextRun is one show operator as an event sink sees it. Bytes holds the shown
// text: for TJ the string elements concatenated, with the numbers omitted.
type TextRun struct {
	FontName    string
	Size        float64
	TextMatrix  graphics.Matrix
	LineMatrix  graphics.Matrix
	Rise        float64
	CharSpacing float64
	WordSpacing float64
	HScale      float64
	Bytes       []byte
}

// TextRunSink receives one TextRun per show operator, in stream order, before
// the operator paints its glyphs. A nil sink keeps the old behavior.
type TextRunSink interface {
	TextRun(r TextRun)
}

// TextOptions carries the text seam for one content run.
type TextOptions struct {
	// Fonts resolves a /Font resource name. A nil map uses the fonts in
	// PaintOptions.Resources.
	Fonts map[string]*Font
	// Sink receives positioned glyphs. A nil sink discards them.
	Sink GlyphSink
	// Runs receives one event per show operator. A nil sink discards them.
	Runs TextRunSink
}

// glyphMarker is implemented by markers that blend glyph coverage. A marker
// without it refuses text with undefined, which keeps the rewrite recorder
// text-free.
type glyphMarker interface {
	DrawGlyph(mask *image.Alpha, originX, originY int, red, green, blue float64)
}

// textState is the text state that q and Q save. The text matrices are not
// part of the graphics state and BT resets them.
type textState struct {
	font        *Font
	fontName    string
	size        float64
	hscale      float64
	leading     float64
	charSpacing float64
	wordSpacing float64
	rise        float64
}

func (run *runner) takeText(opName string) (bool, error) {
	if handled, err := run.takeTextSetup(opName); handled {
		return true, err
	}
	if handled, err := run.takeTextSpacing(opName); handled {
		return true, err
	}
	return run.takeTextShow(opName)
}

// takeTextSetup dispatches the operators that move the text object or set the
// font and matrices.
func (run *runner) takeTextSetup(opName string) (bool, error) {
	switch opName {
	case opTextBegin:
		run.beginText()
		return true, nil
	case "ET":
		return true, nil
	case "Tf":
		return true, run.setFont()
	case "Td":
		return true, run.moveTextOp()
	case "TD":
		return true, run.moveTextLeading()
	case "Tm":
		return true, run.setTextMatrix()
	case "T*":
		return true, run.nextLine()
	default:
		return false, nil
	}
}

// takeTextSpacing dispatches the text state parameters.
func (run *runner) takeTextSpacing(opName string) (bool, error) {
	switch opName {
	case "Tc":
		return true, run.setCharSpacing()
	case "Tw":
		return true, run.setWordSpacing()
	case "Tz":
		return true, run.setHScale()
	case "TL":
		return true, run.setTextLeading()
	case "Ts":
		return true, run.setTextRise()
	default:
		return false, nil
	}
}

// takeTextShow dispatches the four string show operators.
func (run *runner) takeTextShow(opName string) (bool, error) {
	switch opName {
	case opShow:
		return true, run.showString()
	case opShowArray:
		return true, run.showArray()
	case opShowQuote:
		return true, run.quoteShow()
	case opShowDQuote:
		return true, run.doubleQuoteShow()
	default:
		return false, nil
	}
}

func (run *runner) beginText() {
	run.textMatrix = graphics.Identity()
	run.textLine = graphics.Identity()
}

func (run *runner) setFont() error {
	size, err := run.popNum(opTextFont)
	if err != nil {
		return err
	}
	name, err := run.popName(opTextFont)
	if err != nil {
		return err
	}
	fnt, ok := run.fonts[name]
	if !ok || fnt == nil {
		return NewError(opTextFont, errUndefined)
	}
	run.text.font = fnt
	run.text.fontName = name
	run.text.size = size
	return nil
}

func (run *runner) moveTextOp() error {
	posX, posY, err := run.popXY("Td")
	if err != nil {
		return err
	}
	run.moveText(posX, posY)
	return nil
}

func (run *runner) moveTextLeading() error {
	posX, posY, err := run.popXY("TD")
	if err != nil {
		return err
	}
	run.text.leading = -posY
	run.moveText(posX, posY)
	return nil
}

func (run *runner) moveText(posX, posY float64) {
	run.textLine = run.textLine.Translate(posX, posY)
	run.textMatrix = run.textLine
}

func (run *runner) setTextMatrix() error {
	const opName = "Tm"
	vals := make([]float64, matrixLen)
	for idx := len(vals) - 1; idx >= 0; idx-- {
		val, err := run.popNum(opName)
		if err != nil {
			return err
		}
		vals[idx] = val
	}
	run.textLine = graphics.Matrix{
		A: vals[0],
		B: vals[1],
		C: vals[2],
		D: vals[3],
		E: vals[4],
		F: vals[5],
	}
	run.textMatrix = run.textLine
	return nil
}

func (run *runner) nextLine() error {
	run.moveText(0, -run.text.leading)
	return nil
}

func (run *runner) setCharSpacing() error {
	value, err := run.popNum("Tc")
	if err != nil {
		return err
	}
	run.text.charSpacing = value
	return nil
}

func (run *runner) setWordSpacing() error {
	value, err := run.popNum("Tw")
	if err != nil {
		return err
	}
	run.text.wordSpacing = value
	return nil
}

func (run *runner) setHScale() error {
	value, err := run.popNum("Tz")
	if err != nil {
		return err
	}
	run.text.hscale = value / percentScale
	return nil
}

func (run *runner) setTextLeading() error {
	value, err := run.popNum("TL")
	if err != nil {
		return err
	}
	run.text.leading = value
	return nil
}

func (run *runner) setTextRise() error {
	value, err := run.popNum("Ts")
	if err != nil {
		return err
	}
	run.text.rise = value
	return nil
}

func (run *runner) showString() error {
	text, err := run.popStr(opShow)
	if err != nil {
		return err
	}
	run.fireRun(text)
	return run.showBytes(opShow, text)
}

// showArray walks a TJ array. A number moves the text position and a string
// shows. Any other element is typecheck.
func (run *runner) showArray() error {
	items, err := run.popItems(opShowArray)
	if err != nil {
		return err
	}
	shown, err := arrayText(items)
	if err != nil {
		return err
	}
	run.fireRun(shown)
	for _, element := range items {
		if element.kind == itemString {
			if err := run.showBytes(opShowArray, element.str); err != nil {
				return err
			}
			continue
		}
		if element.kind == itemNumber {
			run.offsetText(-element.num / emScale * run.text.size * run.text.hscale)
			continue
		}
		return NewError(opShowArray, errType)
	}
	return nil
}

// arrayText concatenates the string elements of one TJ array and validates the
// other elements.
func arrayText(items []item) ([]byte, error) {
	var out []byte
	for _, element := range items {
		if element.kind == itemString {
			out = append(out, element.str...)
			continue
		}
		if element.kind != itemNumber {
			return nil, NewError(opShowArray, errType)
		}
	}
	return out, nil
}

func (run *runner) quoteShow() error {
	text, err := run.popStr(opShowQuote)
	if err != nil {
		return err
	}
	if err := run.nextLine(); err != nil {
		return err
	}
	run.fireRun(text)
	return run.showBytes(opShowQuote, text)
}

// doubleQuoteShow pops aw, ac, and the string, sets the spacing, moves to the
// next line, and shows.
func (run *runner) doubleQuoteShow() error {
	text, err := run.popStr(opShowDQuote)
	if err != nil {
		return err
	}
	charSpacing, err := run.popNum(opShowDQuote)
	if err != nil {
		return err
	}
	wordSpacing, err := run.popNum(opShowDQuote)
	if err != nil {
		return err
	}
	run.text.charSpacing = charSpacing
	run.text.wordSpacing = wordSpacing
	if err := run.nextLine(); err != nil {
		return err
	}
	run.fireRun(text)
	return run.showBytes(opShowDQuote, text)
}

// fireRun delivers one show operator to the run sink with the current text
// state, before the operator paints its glyphs. A nil sink discards it.
func (run *runner) fireRun(text []byte) {
	if !activeSink(run.runs) {
		return
	}
	run.runs.TextRun(TextRun{
		FontName:    run.text.fontName,
		Size:        run.text.size,
		TextMatrix:  run.textMatrix,
		LineMatrix:  run.textLine,
		Rise:        run.text.rise,
		CharSpacing: run.text.charSpacing,
		WordSpacing: run.text.wordSpacing,
		HScale:      run.text.hscale,
		Bytes:       slices.Clone(text),
	})
}

// showBytes shows one string. A Type0 font reads two-byte codes and ignores a
// trailing odd byte.
func (run *runner) showBytes(opName string, text []byte) error {
	fnt := run.text.font
	if fnt == nil {
		return NewError(opName, errUndefined)
	}
	if fnt.twoByteCodes() {
		for idx := 0; idx+1 < len(text); idx += 2 {
			code := uint32(text[idx])<<byteShift | uint32(text[idx+1])
			if err := run.showCode(opName, fnt, code); err != nil {
				return err
			}
		}
		return nil
	}
	for _, code := range text {
		if err := run.showCode(opName, fnt, uint32(code)); err != nil {
			return err
		}
	}
	return nil
}

// showCode paints one glyph, delivers it to the sink, and advances. The
// device check runs first, so a device without glyph support reports
// undefined before the font's outline source is consulted.
func (run *runner) showCode(opName string, fnt *Font, code uint32) error {
	if run.marker != nil {
		if err := run.paintGlyph(opName, fnt, code); err != nil {
			return err
		}
	}
	advance := run.advance(fnt, code)
	if run.sink != nil {
		unicode, _ := fnt.Unicode(code)
		posX, posY := run.textDeviceMatrix().Apply(0, 0)
		run.sink.Glyph(Glyph{
			Code:    code,
			Unicode: unicode,
			Advance: advance,
			X:       posX,
			Y:       posY,
			Box:     run.glyphBox(advance),
		})
	}
	run.offsetText(advance)
	return nil
}

// glyphBox is the device advance box of one glyph: the baseline origin to the
// advanced origin horizontally, and the nominal ascent and descent of the text
// size vertically. The text matrix and the CTM rotate and translate it.
func (run *runner) glyphBox(advance float64) Box {
	device := run.textDeviceMatrix()
	originX, originY := device.Apply(0, 0)
	nextX, nextY := device.Apply(advance, 0)
	ascentX, ascentY := run.glyphMatrix().Apply(0, boxAscent)
	descentX, descentY := run.glyphMatrix().Apply(0, -boxDescent)
	return Box{
		MinX: math.Min(math.Min(originX, nextX), math.Min(ascentX, descentX)),
		MinY: math.Min(math.Min(originY, nextY), math.Min(ascentY, descentY)),
		MaxX: math.Max(math.Max(originX, nextX), math.Max(ascentX, descentX)),
		MaxY: math.Max(math.Max(originY, nextY), math.Max(ascentY, descentY)),
	}
}

func (run *runner) offsetText(tx float64) {
	run.textMatrix = run.textMatrix.Translate(tx, 0)
}

// advance is the horizontal displacement of one code in text space:
// (w0/1000 * Tfs + Tc + Tw) * Th, with Tw on the space code.
func (run *runner) advance(fnt *Font, code uint32) float64 {
	width := fnt.Width(code)
	tx := (width/1000*run.text.size + run.text.charSpacing) * run.text.hscale
	if code == ' ' {
		tx += run.text.wordSpacing * run.text.hscale
	}
	return tx
}

// paintGlyph blends one glyph into the marker. A marker without glyph support
// refuses text with undefined. A font without an outline source is
// invalidfont, the policy in documentation/fonts.md.
func (run *runner) paintGlyph(opName string, fnt *Font, code uint32) error {
	painter, ok := run.marker.(glyphMarker)
	if !ok {
		return NewError(opName, errUndefined)
	}
	mask, origin, ok := run.glyphMask(fnt, code)
	if !ok {
		return NewError(opName, errInvalidFont)
	}
	if len(run.clips) > 0 {
		if target, ok := run.marker.(graphics.ClipMarker); ok {
			target.DrawGlyphClipped(run.clips, mask, origin.X, origin.Y, run.red, run.green, run.blue)
			return nil
		}
	}
	painter.DrawGlyph(mask, origin.X, origin.Y, run.red, run.green, run.blue)
	return nil
}

// glyphMask rasterizes one glyph outline to device coverage. The origin is
// the device pixel of the mask's bottom-left corner. The bool is false when
// the font has no outline for the code.
func (run *runner) glyphMask(fnt *Font, code uint32) (*image.Alpha, image.Point, bool) {
	segments, ok := fnt.outline(code)
	if !ok {
		return nil, image.Point{X: 0, Y: 0}, false
	}
	mat := run.glyphMatrix()
	minX, minY, maxX, maxY, hasBox := glyphBounds(mat, segments)
	if !hasBox {
		return nil, image.Point{X: 0, Y: 0}, true
	}
	left := int(math.Floor(minX)) - glyphPadding
	bottom := int(math.Floor(minY)) - glyphPadding
	right := int(math.Ceil(maxX)) + glyphPadding
	top := int(math.Ceil(maxY)) + glyphPadding
	width := right - left
	height := top - bottom
	if width <= 0 || height <= 0 {
		return nil, image.Point{X: 0, Y: 0}, true
	}
	if width > maxGlyphSide || height > maxGlyphSide || width*height > maxGlyphPixels {
		return nil, image.Point{X: 0, Y: 0}, false
	}
	mask := image.NewAlpha(image.Rect(0, 0, width, height))
	raster := vector.NewRasterizer(width, height)
	raster.DrawOp = draw.Src
	rasterizeGlyph(raster, mat, segments, left, top)
	raster.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: maxAlpha}), image.Point{X: 0, Y: 0})
	return mask, image.Pt(left, bottom), true
}

// glyphMatrix maps glyph space, 1/1000 em with Y up, to device pixels.
func (run *runner) glyphMatrix() graphics.Matrix {
	fontScale := graphics.Matrix{
		A: run.text.size * run.text.hscale,
		B: 0,
		C: 0,
		D: run.text.size,
		E: 0,
		F: run.text.rise,
	}
	return graphics.Concat(fontScale, run.textDeviceMatrix())
}

// textDeviceMatrix maps text space to device pixels. The font size and rise
// are not part of it.
func (run *runner) textDeviceMatrix() graphics.Matrix {
	deviceScale := graphics.Matrix{A: run.scale, B: 0, C: 0, D: run.scale, E: 0, F: 0}
	return graphics.Concat(run.textMatrix, graphics.Concat(run.ctm, deviceScale))
}

// glyphBounds returns the device bounds of the transformed segments.
func glyphBounds(mat graphics.Matrix, segments sfnt.Segments) (float64, float64, float64, float64, bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, seg := range segments {
		for _, arg := range segmentArgs(seg) {
			devX, devY := glyphDevice(mat, arg)
			minX = math.Min(minX, devX)
			maxX = math.Max(maxX, devX)
			minY = math.Min(minY, devY)
			maxY = math.Max(maxY, devY)
		}
	}
	if maxX < minX || maxY < minY {
		return 0, 0, 0, 0, false
	}
	return minX, minY, maxX, maxY, true
}

// rasterizeGlyph draws the transformed segments into the mask space. Mask
// row 0 is the top of the device box, so Y flips around top.
func rasterizeGlyph(raster *vector.Rasterizer, mat graphics.Matrix, segments sfnt.Segments, left, top int) {
	for _, seg := range segments {
		args := segmentArgs(seg)
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			posX, posY := maskPoint(mat, args[0], left, top)
			raster.MoveTo(posX, posY)
		case sfnt.SegmentOpLineTo:
			posX, posY := maskPoint(mat, args[0], left, top)
			raster.LineTo(posX, posY)
		case sfnt.SegmentOpQuadTo:
			ctrlX, ctrlY := maskPoint(mat, args[0], left, top)
			posX, posY := maskPoint(mat, args[1], left, top)
			raster.QuadTo(ctrlX, ctrlY, posX, posY)
		case sfnt.SegmentOpCubeTo:
			ctrl1X, ctrl1Y := maskPoint(mat, args[0], left, top)
			ctrl2X, ctrl2Y := maskPoint(mat, args[1], left, top)
			posX, posY := maskPoint(mat, args[2], left, top)
			raster.CubeTo(ctrl1X, ctrl1Y, ctrl2X, ctrl2Y, posX, posY)
		}
	}
}

// segmentArgs returns the points of one segment, one for a move or line, two
// for a quadratic curve, and three for a cubic curve.
func segmentArgs(seg sfnt.Segment) []fixed.Point26_6 {
	switch seg.Op {
	case sfnt.SegmentOpMoveTo, sfnt.SegmentOpLineTo:
		return seg.Args[:1]
	case sfnt.SegmentOpQuadTo:
		return seg.Args[:2]
	case sfnt.SegmentOpCubeTo:
		return seg.Args[:3]
	default:
		return seg.Args[:1]
	}
}

// glyphDevice maps one segment point to device pixels. The segments carry
// 26.6 coordinates at outlinePPEM pixels per em with Y growing down.
func glyphDevice(mat graphics.Matrix, point fixed.Point26_6) (float64, float64) {
	posX := float64(point.X) / fixedShift / outlinePPEM
	posY := -float64(point.Y) / fixedShift / outlinePPEM
	return mat.Apply(posX, posY)
}

func maskPoint(mat graphics.Matrix, point fixed.Point26_6, left, top int) (float32, float32) {
	devX, devY := glyphDevice(mat, point)
	return float32(devX - float64(left)), float32(float64(top) - devY)
}
