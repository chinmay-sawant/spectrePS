package pdf

import (
	"context"
	"image"
	"math"
	"slices"
	"strconv"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

type (
	tokenKind int

	itemKind int

	ctok struct {
		kind tokenKind
		num  float64
		text string
		str  []byte
		arr  []item
		val  Value
	}

	scanner struct {
		src []byte
		pos int
	}

	point struct {
		posX float64
		posY float64
		move bool
	}

	snapshot struct {
		path        []point
		hasPt       bool
		curX        float64
		curY        float64
		subX        float64
		subY        float64
		subOpen     bool
		ctm         graphics.Matrix
		width       float64
		red         float64
		green       float64
		blue        float64
		fillAlpha   float64
		strokeAlpha float64
		blendMode   graphics.BlendMode
		softMask    []byte
		space       colorSpace
		values      []float64
		text        textState
		clips       []graphics.Clip
	}

	item struct {
		kind itemKind
		num  float64
		name string
		str  []byte
		arr  []item
		val  Value
	}

	runner struct {
		marker      graphics.Marker
		scale       float64
		file        *File
		xobjects    map[string]Value
		images      map[string]image.Image
		masks       map[string]*image.Alpha
		colors      map[string]Value
		fonts       map[string]*Font
		extgstates  map[string]Value
		properties  map[string]Value
		ocOff       map[int]bool
		sink        GlyphSink
		runs        TextRunSink
		mcSink      MarkedContentSink
		mcStack     []mcFrame
		clips       []graphics.Clip
		formDepth   int
		skipDepth   int
		stack       []item
		path        []point
		hasPt       bool
		curX        float64
		curY        float64
		subX        float64
		subY        float64
		subOpen     bool
		ctm         graphics.Matrix
		width       float64
		red         float64
		green       float64
		blue        float64
		fillAlpha   float64
		strokeAlpha float64
		blendMode   graphics.BlendMode
		softMask    []byte
		space       colorSpace
		values      []float64
		saves       []*snapshot
		text        textState
		textMatrix  graphics.Matrix
		textLine    graphics.Matrix
	}
)

const (
	tokNumber tokenKind = iota
	tokOperand
	tokOperator
	ctokName
	ctokString
	ctokArray
	ctokDict
)

const (
	itemNumber itemKind = iota
	itemName
	itemString
	itemArray
	itemDict
	itemOther
)

const (
	defaultWidth = 1
	maxSaveDepth = 32
	curveCount   = 3
	octalExtra   = 2
	pairLen      = 2
	matrixLen    = 6

	errUnderflow = "stackunderflow"
	errNoPoint   = "nocurrentpoint"

	panicNilContext = "pdf: nil context"
	syntaxOp        = "content"
)

// Paint runs one PDF content stream onto marker.
// scale matches PostScript UsePixmap: a user unit becomes scale device pixels. 72 dpi uses 1.
// Paint does not call ShowPage.
func Paint(ctx context.Context, content []byte, marker graphics.Marker, scale float64) error {
	return PaintWith(ctx, content, marker, scale, emptyOptions())
}

// emptyOptions returns options with no page resources.
func emptyOptions() PaintOptions {
	return PaintOptions{
		Resources:     emptyResources(),
		Text:          TextOptions{Fonts: nil, Sink: nil, Runs: nil},
		MarkedContent: nil,
	}
}

// PaintWith runs one PDF content stream with the page resources the
// interpreter reads. Paint is PaintWith with empty options.
func PaintWith(
	ctx context.Context,
	content []byte,
	marker graphics.Marker,
	scale float64,
	opt PaintOptions,
) error {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	run := newRunner(marker, scale)
	run.file = opt.Resources.file
	run.xobjects = opt.Resources.XObjects
	run.fonts = opt.Resources.Fonts
	run.extgstates = opt.Resources.ExtGStates
	run.properties = opt.Resources.Properties
	run.colors = opt.Resources.Colors
	if opt.Text.Fonts != nil {
		run.fonts = opt.Text.Fonts
	}
	run.sink = opt.Text.Sink
	run.runs = opt.Text.Runs
	run.mcSink = opt.MarkedContent
	ocOff, err := opt.Resources.ocOffGroups()
	if err != nil {
		return err
	}
	run.ocOff = ocOff
	lex := scanner{src: content, pos: 0}
	return run.play(ctx, &lex)
}

func newRunner(marker graphics.Marker, scale float64) *runner {
	return &runner{
		marker:      marker,
		scale:       scale,
		file:        nil,
		xobjects:    nil,
		images:      nil,
		masks:       nil,
		colors:      nil,
		fonts:       nil,
		extgstates:  nil,
		properties:  nil,
		ocOff:       nil,
		sink:        nil,
		runs:        nil,
		mcSink:      nil,
		mcStack:     nil,
		clips:       nil,
		formDepth:   0,
		skipDepth:   0,
		stack:       nil,
		path:        nil,
		hasPt:       false,
		curX:        0,
		curY:        0,
		subX:        0,
		subY:        0,
		subOpen:     false,
		ctm:         graphics.Identity(),
		width:       defaultWidth,
		red:         0,
		green:       0,
		blue:        0,
		fillAlpha:   1,
		strokeAlpha: 1,
		blendMode:   graphics.BlendNormal,
		softMask:    nil,
		// The initial color space is DeviceGray with a black component, so a
		// page that never sets color paints black, as it does today.
		space:  deviceSpace(1, grayPreview),
		values: []float64{0},
		saves:  nil,
		text: textState{
			font: nil, fontName: "", size: 0, hscale: 1,
			leading: 0, charSpacing: 0, wordSpacing: 0, rise: 0,
		},
		textMatrix: graphics.Identity(),
		textLine:   graphics.Identity(),
	}
}

func (run *runner) play(ctx context.Context, lex *scanner) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, ok, err := lex.next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if tok.kind == tokOperator && tok.text == "BX" {
			if err := lex.skipCompat(); err != nil {
				return err
			}
			continue
		}
		if err := run.take(ctx, tok); err != nil {
			return err
		}
	}
}

// skipCompat skips one BX compatibility section through the matching EX.
// End of stream inside the section is syntaxerror.
func (lex *scanner) skipCompat() error {
	for {
		tok, ok, err := lex.next()
		if err != nil {
			return err
		}
		if !ok {
			return contentSyntax()
		}
		if tok.kind == tokOperator && tok.text == "EX" {
			return nil
		}
	}
}

func (run *runner) take(ctx context.Context, tok ctok) error {
	if tok.kind != tokOperator {
		run.stack = append(run.stack, itemOf(tok))
		return nil
	}
	if run.skipDepth > 0 {
		return run.takeSkipped(tok.text)
	}
	if handled, err := run.takePath(tok.text); handled {
		return err
	}
	if handled, err := run.takePaint(tok.text); handled {
		return err
	}
	if handled, err := run.takeDo(ctx, tok.text); handled {
		return err
	}
	if handled, err := run.takeState(ctx, tok.text); handled {
		return err
	}
	if handled, err := run.takeMarked(tok.text); handled {
		return err
	}
	if handled, err := run.takeText(tok.text); handled {
		return err
	}
	if handled, err := run.takeStateText(tok.text); handled {
		return err
	}
	if tok.text == "EX" {
		return nil
	}
	return NewError(tok.text, errUndefined)
}

// takeSkipped walks operators inside content hidden by an OFF optional
// content group. Only BMC, BDC, and EMC matter, because they change the
// skip nesting. Operands still pile up on the stack and are dropped at the
// matching EMC, because a skipped section may hold any operator. The EMC that
// closes the OFF group also closes the marked-content frame, so the sink stays
// paired while the painting is skipped.
func (run *runner) takeSkipped(opName string) error {
	switch opName {
	case "BMC", "BDC":
		run.skipDepth++
	case "EMC":
		run.skipDepth--
		if run.skipDepth == 0 {
			run.stack = nil
			return run.endMarked()
		}
	}
	return nil
}

func (run *runner) takePath(opName string) (bool, error) {
	switch opName {
	case "m":
		return true, run.moveto()
	case "l":
		return true, run.lineto()
	case "c":
		return true, run.curveto()
	case "h":
		return true, run.closepath("h")
	case "re":
		return true, run.rectangle()
	default:
		return false, nil
	}
}

// takePaint dispatches the painting and clipping operators.
func (run *runner) takePaint(opName string) (bool, error) {
	if handled, err := run.takePaintPath(opName); handled {
		return true, err
	}
	return run.takePaintClip(opName)
}

// takePaintPath dispatches the path painting operators.
func (run *runner) takePaintPath(opName string) (bool, error) {
	switch opName {
	case "S":
		run.stroke()
	case "s":
		if err := run.closepath("s"); err != nil {
			return true, err
		}
		run.stroke()
	case "f":
		run.fill(false)
	case "f*":
		run.fill(true)
	case "n":
		run.clearPath()
	default:
		return false, nil
	}
	return true, nil
}

// takePaintClip dispatches the fill-and-stroke and clip operators.
func (run *runner) takePaintClip(opName string) (bool, error) {
	switch opName {
	case "B":
		return true, run.fillStroke(false, false, "B")
	case "B*":
		return true, run.fillStroke(true, false, "B*")
	case "b":
		return true, run.fillStroke(false, true, "b")
	case "b*":
		return true, run.fillStroke(true, true, "b*")
	case "W":
		return true, run.clip(false)
	case "W*":
		return true, run.clip(true)
	default:
		return false, nil
	}
}

// takeState dispatches the graphics state operators.
func (run *runner) takeState(ctx context.Context, opName string) (bool, error) {
	if handled, err := run.takeStateCore(ctx, opName); handled {
		return true, err
	}
	if handled, err := run.takeStateLine(opName); handled {
		return true, err
	}
	return run.takeStateColor(opName)
}

// takeStateCore dispatches the save, matrix, width, and gs operators.
func (run *runner) takeStateCore(ctx context.Context, opName string) (bool, error) {
	switch opName {
	case "q":
		return true, run.save()
	case "Q":
		return true, run.restore()
	case "cm":
		return true, run.ctmConcat()
	case "w":
		return true, run.setWidth()
	case "gs":
		return true, run.setExtGState(ctx)
	default:
		return false, nil
	}
}

// takeStateLine dispatches the accepted line parameter no-ops.
func (run *runner) takeStateLine(opName string) (bool, error) {
	switch opName {
	case "J", "j", "M", "i":
		return true, run.discardNum(opName)
	case "ri":
		return true, run.discardName(opName)
	case "d":
		return true, run.discardDash(opName)
	default:
		return false, nil
	}
}

// takeStateText is the fallback for the text rendering mode. The text machine
// does not read Tr, and this subset paints glyphs in fill mode only, so the
// operand is accepted as a no-op with a documented deviation. The fallback
// runs after takeText, so a text-state implementation takes precedence when
// one lands.
func (run *runner) takeStateText(opName string) (bool, error) {
	if opName == "Tr" {
		return true, run.discardNum(opName)
	}
	return false, nil
}

// takeStateColor dispatches the color operators. The stroke and non-stroking
// forms share one current color, the behavior this interpreter had before
// the color space work, so an RG color still fills a path.
func (run *runner) takeStateColor(opName string) (bool, error) {
	switch opName {
	case "RG", "rg":
		return true, run.setRGB(opName)
	case "G", "g":
		return true, run.setGray(opName)
	case "K", "k":
		return true, run.setCMYK()
	case "CS", "cs":
		return true, run.setColorSpaceOp(opName)
	case "SC", "sc":
		return true, run.setComponents(opName)
	case "SCN", "scn":
		return true, run.setComponentsName(opName)
	default:
		return false, nil
	}
}

func (lex *scanner) next() (ctok, bool, error) {
	lex.skipIgnored()
	if lex.pos >= len(lex.src) {
		return zeroToken(), false, nil
	}
	tok, err := lex.one()
	if err != nil {
		return zeroToken(), false, err
	}
	return tok, true, nil
}

func (lex *scanner) one() (ctok, error) {
	switch lex.src[lex.pos] {
	case '/':
		return lex.name()
	case '(':
		return lex.literalToken()
	case '<':
		return lex.angleToken()
	case '[':
		return lex.arrayToken()
	case ']', ')', '>', '{', '}':
		return zeroToken(), contentSyntax()
	default:
		return lex.scanWord()
	}
}

func (lex *scanner) scanWord() (ctok, error) {
	start := lex.pos
	lex.skipWord()
	if start == lex.pos {
		return zeroToken(), contentSyntax()
	}
	text := string(lex.src[start:lex.pos])
	if keywordOperand(text) {
		return operandToken(), nil
	}
	if !numberSyntax(text) {
		return operatorToken(text), nil
	}
	num, ok := finiteNumber(text)
	if !ok {
		return zeroToken(), contentSyntax()
	}
	return numberToken(num), nil
}

func (lex *scanner) skipIgnored() {
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		if contentSpace(cur) {
			lex.pos++
			continue
		}
		if cur != '%' {
			return
		}
		lex.skipComment()
	}
}

func (lex *scanner) skipComment() {
	lex.pos++
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		if cur == '\n' || cur == '\r' {
			return
		}
		lex.pos++
	}
}

// name scans one PDF name and returns its text, without the slash.
// Names nested inside arrays and dictionaries are skipped by the caller and
// never reach the operand stack.
func (lex *scanner) name() (ctok, error) {
	start := lex.pos + 1
	lex.skipName()
	return nameToken(string(lex.src[start:lex.pos])), nil
}

func (lex *scanner) skipName() {
	lex.pos++
	lex.skipWord()
}

func (lex *scanner) skipWord() {
	for lex.pos < len(lex.src) && !isBreak(lex.src[lex.pos]) {
		lex.pos++
	}
}

// literalToken scans one string in parentheses and decodes its escapes.
func (lex *scanner) literalToken() (ctok, error) {
	raw, err := lex.literal()
	if err != nil {
		return zeroToken(), err
	}
	return stringToken(raw), nil
}

// literal scans one string in parentheses. Nesting counts, escapes decode,
// and an end-of-line marker becomes a line feed.
func (lex *scanner) literal() ([]byte, error) {
	lex.pos++
	buf := make([]byte, 0)
	depth := 1
	for depth > 0 {
		if lex.pos >= len(lex.src) {
			return nil, contentSyntax()
		}
		cur := lex.src[lex.pos]
		lex.pos++
		next, err := lex.literalByte(cur, depth, &buf)
		if err != nil {
			return nil, err
		}
		depth = next
	}
	return buf, nil
}

// literalByte appends one string byte and returns the new nesting depth.
func (lex *scanner) literalByte(cur byte, depth int, buf *[]byte) (int, error) {
	switch {
	case cur == '\\':
		got, keep, err := lex.escape()
		if err != nil {
			return depth, err
		}
		if keep {
			*buf = append(*buf, got)
		}
		return depth, nil
	case cur == '(':
		*buf = append(*buf, cur)
		return depth + 1, nil
	case cur == ')':
		return closeLiteral(depth, buf), nil
	case cur == '\r':
		lex.skipLF()
		*buf = append(*buf, '\n')
		return depth, nil
	default:
		*buf = append(*buf, cur)
		return depth, nil
	}
}

// escape reads one backslash escape. The second result is false for a line
// continuation, which contributes no byte.
func (lex *scanner) escape() (byte, bool, error) {
	if lex.pos >= len(lex.src) {
		return 0, false, contentSyntax()
	}
	cur := lex.src[lex.pos]
	lex.pos++
	if named, ok := namedEscape(cur); ok {
		return named, true, nil
	}
	switch cur {
	case '\n':
		return 0, false, nil
	case '\r':
		lex.skipLF()
		return 0, false, nil
	}
	if contentOctal(cur) {
		return lex.octalByte(cur), true, nil
	}
	return cur, true, nil
}

func (lex *scanner) octalByte(first byte) byte {
	value := int(first - '0')
	extra := octalExtra
	for extra > 0 && lex.pos < len(lex.src) && contentOctal(lex.src[lex.pos]) {
		value = value*octalBase + int(lex.src[lex.pos]-'0')
		lex.pos++
		extra--
	}
	return byte(value & byteMask)
}

func (lex *scanner) skipLF() {
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '\n' {
		lex.pos++
	}
}

func (lex *scanner) skipString() error {
	_, err := lex.literal()
	return err
}

func (lex *scanner) angleToken() (ctok, error) {
	if lex.startsPair('<') {
		val, err := lex.dictValue()
		if err != nil {
			return zeroToken(), err
		}
		return dictToken(val), nil
	}
	raw, err := lex.hex()
	if err != nil {
		return zeroToken(), err
	}
	return stringToken(raw), nil
}

// dictValue parses one << ... >> operand into a PDF dictionary. The content
// skip finds the close, then the object lexer parses the same bytes.
func (lex *scanner) dictValue() (Value, error) {
	start := lex.pos
	if err := lex.skipDict(); err != nil {
		return NullVal(), err
	}
	val, _, err := ParseValue(lex.src[start:lex.pos], 0)
	if err != nil || val.Kind != KindDict {
		return NullVal(), contentSyntax()
	}
	return val, nil
}

// hex scans one hex string. An odd final nibble is stored as if a 0 followed it.
func (lex *scanner) hex() ([]byte, error) {
	lex.pos++
	buf := make([]byte, 0)
	high := byte(0)
	odd := false
	for lex.pos < len(lex.src) {
		cur := lex.src[lex.pos]
		lex.pos++
		if cur == '>' {
			if odd {
				buf = append(buf, high<<nibbleShift)
			}
			return buf, nil
		}
		if contentSpace(cur) {
			continue
		}
		nib, ok := hexValue(cur)
		if !ok {
			return nil, contentSyntax()
		}
		if !odd {
			high = nib
			odd = true
			continue
		}
		buf = append(buf, high<<nibbleShift|nib)
		odd = false
	}
	return nil, contentSyntax()
}

func (lex *scanner) skipHex() error {
	_, err := lex.hex()
	return err
}

func (lex *scanner) skipAngle() error {
	if lex.startsPair('<') {
		return lex.skipDict()
	}
	return lex.skipHex()
}

// arrayToken scans one array operand.
func (lex *scanner) arrayToken() (ctok, error) {
	items, err := lex.arrayItems()
	if err != nil {
		return zeroToken(), err
	}
	return arrayOfToken(items), nil
}

// arrayItems scans the elements of an array. Strings, numbers, and names are
// kept. A dictionary becomes an empty element, so TJ rejects it as typecheck.
func (lex *scanner) arrayItems() ([]item, error) {
	lex.pos++
	items := make([]item, 0)
	for {
		lex.skipIgnored()
		if lex.pos >= len(lex.src) {
			return nil, contentSyntax()
		}
		if lex.src[lex.pos] == ']' {
			lex.pos++
			return items, nil
		}
		element, err := lex.arrayElem()
		if err != nil {
			return nil, err
		}
		items = append(items, element)
	}
}

func (lex *scanner) arrayElem() (item, error) {
	switch lex.src[lex.pos] {
	case '(':
		return lex.arrayLiteral()
	case '<':
		return lex.arrayAngle()
	case '[':
		return lex.arrayNested()
	case '/':
		return lex.arrayName()
	default:
		return lex.arrayWord()
	}
}

func (lex *scanner) arrayLiteral() (item, error) {
	raw, err := lex.literal()
	if err != nil {
		return otherItem(), err
	}
	return stringItem(raw), nil
}

func (lex *scanner) arrayAngle() (item, error) {
	if lex.startsPair('<') {
		val, err := lex.dictValue()
		if err != nil {
			return otherItem(), err
		}
		return dictItem(val), nil
	}
	raw, err := lex.hex()
	if err != nil {
		return otherItem(), err
	}
	return stringItem(raw), nil
}

func (lex *scanner) arrayNested() (item, error) {
	nested, err := lex.arrayItems()
	if err != nil {
		return otherItem(), err
	}
	return arrayItem(nested), nil
}

func (lex *scanner) arrayName() (item, error) {
	tok, err := lex.name()
	if err != nil {
		return otherItem(), err
	}
	return nameItem(tok.text), nil
}

func (lex *scanner) arrayWord() (item, error) {
	tok, err := lex.scanWord()
	if err != nil {
		return otherItem(), err
	}
	if tok.kind == tokNumber {
		return numberItem(tok.num), nil
	}
	return otherItem(), nil
}

func (lex *scanner) skipDict() error {
	lex.pos += pairLen
	return lex.skipNested(lex.dictStep)
}

func (lex *scanner) skipArray() error {
	lex.pos++
	return lex.skipNested(lex.arrayStep)
}

func (lex *scanner) skipNested(step func(*int) error) error {
	depth := 1
	for lex.pos < len(lex.src) && depth > 0 {
		if err := step(&depth); err != nil {
			return err
		}
	}
	if depth != 0 {
		return contentSyntax()
	}
	return nil
}

func (lex *scanner) arrayStep(depth *int) error {
	lex.skipIgnored()
	if lex.pos >= len(lex.src) {
		return nil
	}
	switch lex.src[lex.pos] {
	case '[':
		*depth++
		lex.pos++
	case ']':
		*depth--
		lex.pos++
	case '(':
		return lex.skipString()
	case '<':
		return lex.skipAngle()
	case '/':
		lex.skipName()
	case '{', '}', ')', '>':
		return contentSyntax()
	default:
		lex.skipWord()
	}
	return nil
}

func (lex *scanner) dictStep(depth *int) error {
	lex.skipIgnored()
	if lex.pos >= len(lex.src) {
		return nil
	}
	switch lex.src[lex.pos] {
	case '<':
		return lex.dictAngle(depth)
	case '>':
		return lex.dictGreater(depth)
	case '(':
		return lex.skipString()
	case '[':
		return lex.skipArray()
	case '/':
		lex.skipName()
	case ']', ')', '{', '}':
		return contentSyntax()
	default:
		lex.skipWord()
	}
	return nil
}

func (lex *scanner) dictAngle(depth *int) error {
	if lex.startsPair('<') {
		*depth++
		lex.pos += pairLen
		return nil
	}
	return lex.skipHex()
}

func (lex *scanner) dictGreater(depth *int) error {
	if !lex.startsPair('>') {
		return contentSyntax()
	}
	*depth--
	lex.pos += pairLen
	return nil
}

func (lex *scanner) startsPair(mark byte) bool {
	next := lex.pos + 1
	return next < len(lex.src) && lex.src[lex.pos] == mark && lex.src[next] == mark
}

func (run *runner) moveto() error {
	posX, posY, err := run.popXY("m")
	if err != nil {
		return err
	}
	run.moveTo(posX, posY)
	return nil
}

func (run *runner) lineto() error {
	posX, posY, err := run.popXY("l")
	if err != nil {
		return err
	}
	if err := run.requirePoint("l"); err != nil {
		return err
	}
	run.addPoint(posX, posY, false)
	return nil
}

// curveto appends the three control points as straight segments.
// The PostScript operator does not flatten the Bezier, and neither does this.
func (run *runner) curveto() error {
	const opName = "c"
	steps := make([]point, curveCount)
	for idx := curveCount - 1; idx >= 0; idx-- {
		posY, err := run.popNum(opName)
		if err != nil {
			return err
		}
		posX, err := run.popNum(opName)
		if err != nil {
			return err
		}
		steps[idx] = point{posX: posX, posY: posY, move: false}
	}
	if err := run.requirePoint(opName); err != nil {
		return err
	}
	for _, step := range steps {
		run.addPoint(step.posX, step.posY, false)
	}
	return nil
}

func (run *runner) closepath(opName string) error {
	if err := run.requirePoint(opName); err != nil {
		return err
	}
	if !run.subOpen {
		return NewError(opName, errNoPoint)
	}
	run.addPoint(run.subX, run.subY, false)
	return nil
}

func (run *runner) rectangle() error {
	const opName = "re"
	height, err := run.popNum(opName)
	if err != nil {
		return err
	}
	width, err := run.popNum(opName)
	if err != nil {
		return err
	}
	posY, err := run.popNum(opName)
	if err != nil {
		return err
	}
	posX, err := run.popNum(opName)
	if err != nil {
		return err
	}
	run.appendRect(posX, posY, width, height)
	return nil
}

func (run *runner) appendRect(posX, posY, width, height float64) {
	run.moveTo(posX, posY)
	run.addPoint(posX+width, posY, false)
	run.addPoint(posX+width, posY+height, false)
	run.addPoint(posX, posY+height, false)
	run.addPoint(posX, posY, false)
}

func (run *runner) moveTo(posX, posY float64) {
	run.addPoint(posX, posY, true)
	run.subX = posX
	run.subY = posY
	run.subOpen = true
}

func (run *runner) addPoint(posX, posY float64, move bool) {
	run.path = append(run.path, point{posX: posX, posY: posY, move: move})
	run.curX = posX
	run.curY = posY
	run.hasPt = true
}

func (run *runner) requirePoint(opName string) error {
	if run.hasPt {
		return nil
	}
	return NewError(opName, errNoPoint)
}

func (run *runner) clearPath() {
	run.path = nil
	run.hasPt = false
	run.subOpen = false
	run.curX = 0
	run.curY = 0
	run.subX = 0
	run.subY = 0
}

func (run *runner) stroke() {
	if run.marker != nil {
		run.markerStroke()
	}
	run.clearPath()
}

func (run *runner) fill(evenOdd bool) {
	if run.marker != nil {
		run.markerFill(evenOdd)
	}
	run.clearPath()
}

// fillStroke paints B, B*, b, and b*: fill then stroke on one path. The close
// flag closes the subpath first, as s does.
func (run *runner) fillStroke(evenOdd, closed bool, opName string) error {
	if closed {
		if err := run.closepath(opName); err != nil {
			return err
		}
	}
	if run.marker != nil {
		run.markerFill(evenOdd)
		run.markerStroke()
	}
	run.clearPath()
	return nil
}

// clip intersects the current path into the clip region. A marker that cannot
// apply a clip refuses W with undefined, which is how the rewrite recorder
// keeps its refusal.
func (run *runner) clip(evenOdd bool) error {
	opName := "W"
	if evenOdd {
		opName = "W*"
	}
	if run.marker != nil {
		if _, ok := run.marker.(graphics.ClipMarker); !ok {
			return NewError(opName, errUndefined)
		}
	}
	run.clips = append(run.clips, graphics.Clip{Pts: run.devicePoints(), EvenOdd: evenOdd})
	return nil
}

// markerFill paints the current path through every active clip. A marker that
// applied a W or a form /BBox implements graphics.ClipMarker by construction.
func (run *runner) markerFill(evenOdd bool) {
	pts := run.devicePoints()
	if len(run.clips) == 0 {
		run.marker.Fill(pts, run.red, run.green, run.blue, evenOdd)
		return
	}
	target, ok := run.marker.(graphics.ClipMarker)
	if !ok {
		return
	}
	target.FillClipped(run.clips, pts, run.red, run.green, run.blue, evenOdd)
}

// markerStroke strokes the current path through every active clip.
func (run *runner) markerStroke() {
	pts := run.devicePoints()
	if len(run.clips) == 0 {
		run.marker.Stroke(pts, run.deviceWidth(), run.red, run.green, run.blue)
		return
	}
	target, ok := run.marker.(graphics.ClipMarker)
	if !ok {
		return
	}
	target.StrokeClipped(run.clips, pts, run.deviceWidth(), run.red, run.green, run.blue)
}

// deviceWidth is the stroke width in device pixels. The CTM scale matches
// the PostScript device, which uses the length of the transformed x axis.
func (run *runner) deviceWidth() float64 {
	return run.width * run.scale * ctmScale(run.ctm)
}

// devicePoints returns the current path in device pixels.
func (run *runner) devicePoints() []graphics.Point {
	return run.devicePath(run.path)
}

// devicePath maps one user-space path through the CTM and the paint scale.
func (run *runner) devicePath(path []point) []graphics.Point {
	pts := make([]graphics.Point, len(path))
	for idx, step := range path {
		posX, posY := run.ctm.Apply(step.posX, step.posY)
		pts[idx] = graphics.Point{
			X:    posX * run.scale,
			Y:    posY * run.scale,
			Move: step.move,
		}
	}
	return pts
}

// ctmScale is the effective scale of one user unit on the transformed x axis.
// The PostScript device uses the same measure at stroke time.
func ctmScale(ctm graphics.Matrix) float64 {
	return math.Hypot(ctm.A, ctm.B)
}

func (run *runner) save() error {
	if len(run.saves) >= maxSaveDepth {
		return NewError("q", errLimit)
	}
	run.saves = append(run.saves, run.snap())
	return nil
}

func (run *runner) restore() error {
	count := len(run.saves)
	if count == 0 {
		return NewError("Q", errLimit)
	}
	saved := run.saves[count-1]
	run.saves = run.saves[:count-1]
	run.apply(saved)
	run.syncState()
	return nil
}

func (run *runner) snap() *snapshot {
	return &snapshot{
		path:        slices.Clone(run.path),
		hasPt:       run.hasPt,
		curX:        run.curX,
		curY:        run.curY,
		subX:        run.subX,
		subY:        run.subY,
		subOpen:     run.subOpen,
		ctm:         run.ctm,
		width:       run.width,
		red:         run.red,
		green:       run.green,
		blue:        run.blue,
		fillAlpha:   run.fillAlpha,
		strokeAlpha: run.strokeAlpha,
		blendMode:   run.blendMode,
		softMask:    run.softMask,
		space:       run.space,
		values:      slices.Clone(run.values),
		text:        run.text,
		clips:       slices.Clone(run.clips),
	}
}

func (run *runner) apply(saved *snapshot) {
	run.path = slices.Clone(saved.path)
	run.hasPt = saved.hasPt
	run.curX = saved.curX
	run.curY = saved.curY
	run.subX = saved.subX
	run.subY = saved.subY
	run.subOpen = saved.subOpen
	run.ctm = saved.ctm
	run.width = saved.width
	run.red = saved.red
	run.green = saved.green
	run.blue = saved.blue
	run.fillAlpha = saved.fillAlpha
	run.strokeAlpha = saved.strokeAlpha
	run.blendMode = saved.blendMode
	run.softMask = saved.softMask
	run.space = saved.space
	run.values = slices.Clone(saved.values)
	run.text = saved.text
	run.clips = slices.Clone(saved.clips)
}

func (run *runner) setWidth() error {
	width, err := run.popNum("w")
	if err != nil {
		return err
	}
	run.width = width
	return nil
}

// ctmConcat pops a b c d e f and concatenates the matrix with the CTM, so the
// user point is mapped by the new matrix first and the saved CTM second.
func (run *runner) ctmConcat() error {
	const opName = "cm"
	vals := make([]float64, matrixLen)
	for idx := len(vals) - 1; idx >= 0; idx-- {
		val, err := run.popNum(opName)
		if err != nil {
			return err
		}
		vals[idx] = val
	}
	mat := graphics.Matrix{
		A: vals[0],
		B: vals[1],
		C: vals[2],
		D: vals[3],
		E: vals[4],
		F: vals[5],
	}
	run.ctm = graphics.Concat(mat, run.ctm)
	return nil
}

func (run *runner) setRGB(opName string) error {
	blue, err := run.popNum(opName)
	if err != nil {
		return err
	}
	green, err := run.popNum(opName)
	if err != nil {
		return err
	}
	red, err := run.popNum(opName)
	if err != nil {
		return err
	}
	run.space = deviceSpace(rgbComponents, rgbPreview)
	run.values = []float64{red, green, blue}
	run.setPreview()
	return nil
}

func (run *runner) setGray(opName string) error {
	gray, err := run.popNum(opName)
	if err != nil {
		return err
	}
	run.space = deviceSpace(1, grayPreview)
	run.values = []float64{gray}
	run.setPreview()
	return nil
}

// setCMYK pops c m y k and selects DeviceCMYK, the K operator.
func (run *runner) setCMYK() error {
	const opName = "k"
	black, err := run.popNum(opName)
	if err != nil {
		return err
	}
	yellow, err := run.popNum(opName)
	if err != nil {
		return err
	}
	magenta, err := run.popNum(opName)
	if err != nil {
		return err
	}
	cyan, err := run.popNum(opName)
	if err != nil {
		return err
	}
	run.space = deviceSpace(cmykComponents, cmykPreview)
	run.values = []float64{cyan, magenta, yellow, black}
	run.setPreview()
	return nil
}

// setColorSpaceOp pops a name and selects the current color space. A name
// resolves in /Resources /ColorSpace first and as a device space name second.
// The components reset to 0, the initial color of the new space.
func (run *runner) setColorSpaceOp(opName string) error {
	name, err := run.popName(opName)
	if err != nil {
		return err
	}
	space, err := run.colorSpaceFor(name, opName)
	if err != nil {
		return err
	}
	run.space = space
	run.values = make([]float64, space.components)
	run.setPreview()
	return nil
}

// colorSpaceFor resolves one CS operand: a /ColorSpace resource name, then a
// device space name. An unlisted name is undefined in opName.
func (run *runner) colorSpaceFor(name, opName string) (colorSpace, error) {
	if entry, ok := run.colors[name]; ok {
		space, err := run.file.resolveColorSpace(entry, opName)
		if err != nil {
			return colorSpace{}, err
		}
		return space, nil
	}
	return deviceColorSpace(name, opName)
}

// setComponents pops one value per current color component for SC and sc.
func (run *runner) setComponents(opName string) error {
	count := run.space.components
	if count < 1 {
		count = 1
	}
	values := make([]float64, count)
	for idx := count - 1; idx >= 0; idx-- {
		value, err := run.popNum(opName)
		if err != nil {
			return err
		}
		values[idx] = value
	}
	run.values = values
	run.setPreview()
	return nil
}

// setComponentsName pops the SCN and scn operands. A trailing name selects a
// pattern color, which this subset does not paint, so it is undefined.
func (run *runner) setComponentsName(opName string) error {
	count := len(run.stack)
	if count > 0 && run.stack[count-1].kind == itemName {
		return NewError(opName, errUndefined)
	}
	return run.setComponents(opName)
}

// setPreview recomputes the painted RGB triple from the current color space
// and components, so every mark reaches the RGB pixmap through the preview.
func (run *runner) setPreview() {
	run.red, run.green, run.blue = run.space.rgb(run.values)
}

// setExtGState resolves one /ExtGState name and applies the parameters this
// subset can honor. An unknown name is undefined in gs.
func (run *runner) setExtGState(ctx context.Context) error {
	const opName = "gs"
	name, err := run.popName(opName)
	if err != nil {
		return err
	}
	entry, ok := run.extgstates[name]
	if !ok {
		return NewError(opName, errUndefined)
	}
	resolved, err := run.derefValue(entry, opName)
	if err != nil {
		return err
	}
	if resolved.Kind != KindDict {
		return NewError(opName, errUndefined)
	}
	return run.applyExtGState(ctx, resolved, opName)
}

// applyExtGState validates every entry before it changes the state, so a
// refusal leaves the previous state intact. /LW, /CA, /ca, /BM, and /SMask
// apply through the optional marker seams. /Type, /LC, /LJ, /ML, /RI, /OPM,
// and /SA are no-ops, and so are /AIS false, /OP false, and /op false. Any
// other entry, and any true overprint or alpha-is-shape flag, refuses with
// undefined in gs instead of skipping the state, and so does an alpha or mask
// entry when the marker cannot host the seam, which keeps the rewrite
// recorder honest.
//
//nolint:cyclop // one case per ExtGState entry
func (run *runner) applyExtGState(ctx context.Context, entry Value, opName string) error {
	state := extGState{
		width:       run.width,
		fillAlpha:   run.fillAlpha,
		strokeAlpha: run.strokeAlpha,
		blendMode:   run.blendMode,
		softMask:    run.softMask,
	}
	needsAlpha, needsMask := false, false
	for key, item := range entry.Dict {
		var err error
		switch key {
		case keyType, "LC", "LJ", "ML", "RI":
		case "AIS", "OP", "op":
			err = defaultOff(item, opName)
		case "OPM":
			err = overprintMode(item, opName)
		case "SA":
			err = strokeAdjust(item, opName)
		case "LW":
			err = state.setWidth(item, opName)
		case "CA":
			needsAlpha = true
			err = state.setAlpha(item, opName, true)
		case "ca":
			needsAlpha = true
			err = state.setAlpha(item, opName, false)
		case "BM":
			needsAlpha = true
			err = state.setBlend(item, opName)
		case "SMask":
			needsMask = true
			state.softMask, err = run.stateSoftMask(ctx, item, opName)
		case "TR":
			err = identityTransfer(item, opName)
		default:
			return NewError(opName, errUndefined)
		}
		if err != nil {
			return err
		}
	}
	if err := run.checkStateSeams(needsAlpha, needsMask, opName); err != nil {
		return err
	}
	run.width = state.width
	run.fillAlpha = state.fillAlpha
	run.strokeAlpha = state.strokeAlpha
	run.blendMode = state.blendMode
	run.softMask = state.softMask
	run.syncState()
	return nil
}

// extGState carries the parameters one gs applies after the whole entry
// validates, so a refusal never leaves a partly applied state.
type extGState struct {
	width       float64
	fillAlpha   float64
	strokeAlpha float64
	blendMode   graphics.BlendMode
	softMask    []byte
}

func (state *extGState) setWidth(item Value, opName string) error {
	number, ok := valueNum(item)
	if !ok {
		return NewError(opName, errType)
	}
	state.width = number
	return nil
}

// setAlpha reads /CA or /ca. The value clamps to 0 through 1.
func (state *extGState) setAlpha(item Value, opName string, stroke bool) error {
	number, ok := valueNum(item)
	if !ok {
		return NewError(opName, errType)
	}
	if stroke {
		state.strokeAlpha = clampNumber(number, 0, 1)
		return nil
	}
	state.fillAlpha = clampNumber(number, 0, 1)
	return nil
}

// setBlend reads /BM. A name maps through blendModeName; the four
// non-separable modes and any other name are undefined in gs.
func (state *extGState) setBlend(item Value, opName string) error {
	if item.Kind != KindName {
		return NewError(opName, errUndefined)
	}
	mode, ok := blendModeName(item.Name)
	if !ok {
		return NewError(opName, errUndefined)
	}
	state.blendMode = mode
	return nil
}

// defaultOff accepts a boolean entry whose default is false, /AIS, /OP, and
// /op, as a no-op. The true value changes compositing in a way the RGB
// preview cannot honor, so it refuses in gs like any other unsupported entry.
func defaultOff(item Value, opName string) error {
	if item.Kind == KindBool && !item.Bool {
		return nil
	}
	return NewError(opName, errUndefined)
}

// overprintMode accepts /OPM 0 and 1 as a no-op. The mode only changes a
// compositing path that /OP true and /op true already refuse, so the operand
// is inert here. Any other value refuses.
func overprintMode(item Value, opName string) error {
	mode, ok := valueNum(item)
	if ok && (mode == 0 || mode == 1) {
		return nil
	}
	return NewError(opName, errUndefined)
}

// strokeAdjust accepts /SA as a no-op. The capsule stroke has no automatic
// stroke adjustment, a deviation recorded in documentation/devices.md.
func strokeAdjust(item Value, opName string) error {
	if item.Kind == KindBool {
		return nil
	}
	return NewError(opName, errUndefined)
}

// identityTransfer accepts /TR /Identity and the absent default. Any other
// transfer function is undefined in gs, because this subset builds only an
// identity state soft mask.
func identityTransfer(item Value, opName string) error {
	if item.Kind == KindNull {
		return nil
	}
	if item.Kind == KindName && item.Name == nameIdentity {
		return nil
	}
	return NewError(opName, errUndefined)
}

// checkStateSeams requires the marker to implement the optional seam a gs
// entry needs. A marker without AlphaMarker or SoftMaskMarker keeps its
// refusal, so the rewrite recorder never drops an alpha, blend, or mask effect.
func (run *runner) checkStateSeams(needsAlpha, needsMask bool, opName string) error {
	if run.marker == nil {
		return nil
	}
	if needsAlpha {
		if _, ok := run.marker.(graphics.AlphaMarker); !ok {
			return NewError(opName, errUndefined)
		}
	}
	if needsMask {
		if _, ok := run.marker.(graphics.SoftMaskMarker); !ok {
			return NewError(opName, errUndefined)
		}
	}
	return nil
}

// syncState pushes the alpha, blend, and soft mask state to the marker when
// it implements the optional seams. A marker without them ignores the state,
// exactly as it did before the seams existed.
func (run *runner) syncState() {
	if run.marker == nil {
		return
	}
	if alpha, ok := run.marker.(graphics.AlphaMarker); ok {
		alpha.SetFillAlpha(run.fillAlpha)
		alpha.SetStrokeAlpha(run.strokeAlpha)
		alpha.SetBlendMode(run.blendMode)
	}
	if mask, ok := run.marker.(graphics.SoftMaskMarker); ok {
		mask.SetSoftMask(run.softMask)
	}
}

// discardNum pops and ignores one numeric operand. J, j, M, and i are line
// parameters the capsule stroke cannot honor, and Tr is the text rendering
// mode the fill-only text painter ignores, so all are accepted as no-ops.
func (run *runner) discardNum(opName string) error {
	_, err := run.popNum(opName)
	return err
}

// discardName pops and ignores one name operand. ri is a no-op.
func (run *runner) discardName(opName string) error {
	_, err := run.popName(opName)
	return err
}

// discardDash pops the setdash operands and ignores them. The stroke model is
// a solid capsule with no dash support, so d is accepted as a no-op. The
// operands are still checked: a phase number, then the dash array.
func (run *runner) discardDash(opName string) error {
	if _, err := run.popNum(opName); err != nil {
		return err
	}
	_, err := run.popItems(opName)
	return err
}

// valueNum returns the number in one KindInt or KindReal value.
func valueNum(val Value) (float64, bool) {
	if val.Kind == KindInt {
		return float64(val.Int), true
	}
	if val.Kind == KindReal {
		return val.Real, true
	}
	return 0, false
}

func (run *runner) popXY(opName string) (float64, float64, error) {
	posY, err := run.popNum(opName)
	if err != nil {
		return 0, 0, err
	}
	posX, err := run.popNum(opName)
	if err != nil {
		return 0, 0, err
	}
	return posX, posY, nil
}

func (run *runner) popNum(opName string) (float64, error) {
	count := len(run.stack)
	if count == 0 {
		return 0, NewError(opName, errUnderflow)
	}
	last := run.stack[count-1]
	run.stack = run.stack[:count-1]
	if last.kind != itemNumber {
		return 0, NewError(opName, errType)
	}
	return last.num, nil
}

// popName pops one name operand. A missing or non-name operand is an error.
func (run *runner) popName(opName string) (string, error) {
	count := len(run.stack)
	if count == 0 {
		return "", NewError(opName, errUnderflow)
	}
	last := run.stack[count-1]
	run.stack = run.stack[:count-1]
	if last.kind != itemName {
		return "", NewError(opName, errType)
	}
	return last.name, nil
}

// popStr pops one string operand. Anything else is typecheck.
func (run *runner) popStr(opName string) ([]byte, error) {
	count := len(run.stack)
	if count == 0 {
		return nil, NewError(opName, errUnderflow)
	}
	last := run.stack[count-1]
	run.stack = run.stack[:count-1]
	if last.kind != itemString {
		return nil, NewError(opName, errType)
	}
	return last.str, nil
}

// popItems pops one array operand. Anything else is typecheck.
func (run *runner) popItems(opName string) ([]item, error) {
	count := len(run.stack)
	if count == 0 {
		return nil, NewError(opName, errUnderflow)
	}
	last := run.stack[count-1]
	run.stack = run.stack[:count-1]
	if last.kind != itemArray {
		return nil, NewError(opName, errType)
	}
	return last.arr, nil
}

// itemOf converts one scanned token to the operand it pushes.
func itemOf(tok ctok) item {
	switch tok.kind {
	case tokNumber:
		return numberItem(tok.num)
	case ctokName:
		return nameItem(tok.text)
	case ctokString:
		return stringItem(tok.str)
	case ctokArray:
		return arrayItem(tok.arr)
	case ctokDict:
		return dictItem(tok.val)
	case tokOperand, tokOperator:
		return otherItem()
	default:
		return otherItem()
	}
}

func numberItem(value float64) item {
	return item{kind: itemNumber, num: value, name: "", str: nil, arr: nil, val: NullVal()}
}

func nameItem(name string) item {
	return item{kind: itemName, num: 0, name: name, str: nil, arr: nil, val: NullVal()}
}

func stringItem(raw []byte) item {
	return item{kind: itemString, num: 0, name: "", str: raw, arr: nil, val: NullVal()}
}

func arrayItem(items []item) item {
	return item{kind: itemArray, num: 0, name: "", str: nil, arr: items, val: NullVal()}
}

func dictItem(val Value) item {
	return item{kind: itemDict, num: 0, name: "", str: nil, arr: nil, val: val}
}

func otherItem() item {
	return item{kind: itemOther, num: 0, name: "", str: nil, arr: nil, val: NullVal()}
}

func zeroToken() ctok {
	return ctok{kind: tokNumber, num: 0, text: "", str: nil, arr: nil, val: NullVal()}
}

func numberToken(num float64) ctok {
	return ctok{kind: tokNumber, num: num, text: "", str: nil, arr: nil, val: NullVal()}
}

func operandToken() ctok {
	return ctok{kind: tokOperand, num: 0, text: "", str: nil, arr: nil, val: NullVal()}
}

func operatorToken(text string) ctok {
	return ctok{kind: tokOperator, num: 0, text: text, str: nil, arr: nil, val: NullVal()}
}

func nameToken(text string) ctok {
	return ctok{kind: ctokName, num: 0, text: text, str: nil, arr: nil, val: NullVal()}
}

// stringToken returns one string operand. The bytes are decoded.
func stringToken(raw []byte) ctok {
	return ctok{kind: ctokString, num: 0, text: "", str: raw, arr: nil, val: NullVal()}
}

// arrayOfToken returns one array operand.
func arrayOfToken(items []item) ctok {
	return ctok{kind: ctokArray, num: 0, text: "", str: nil, arr: items, val: NullVal()}
}

// dictToken returns one dictionary operand.
func dictToken(val Value) ctok {
	return ctok{kind: ctokDict, num: 0, text: "", str: nil, arr: nil, val: val}
}

func contentSyntax() error {
	return NewError(syntaxOp, errSyntax)
}

func keywordOperand(text string) bool {
	switch text {
	case "true", "false", "null":
		return true
	default:
		return false
	}
}

func numberSyntax(text string) bool {
	idx, ok := numberStart(text)
	if !ok {
		return false
	}
	return mantissa(text, idx)
}

func numberStart(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	idx := 0
	if text[0] == '+' || text[0] == '-' {
		idx++
	}
	if idx == len(text) {
		return 0, false
	}
	return idx, true
}

func mantissa(text string, idx int) bool {
	sawDigit := false
	sawDot := false
	for idx < len(text) {
		if !mantStep(text[idx], &sawDigit, &sawDot) {
			return false
		}
		idx++
	}
	return sawDigit
}

func mantStep(cur byte, sawDigit, sawDot *bool) bool {
	if cur >= '0' && cur <= '9' {
		*sawDigit = true
		return true
	}
	if cur != '.' || *sawDot {
		return false
	}
	*sawDot = true
	return true
}

func finiteNumber(text string) (float64, bool) {
	num, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return num, true
}

func contentSpace(cur byte) bool {
	switch cur {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func contentDelim(cur byte) bool {
	switch cur {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

func isBreak(cur byte) bool {
	return contentSpace(cur) || contentDelim(cur)
}

func contentOctal(cur byte) bool {
	return cur >= '0' && cur <= '7'
}
