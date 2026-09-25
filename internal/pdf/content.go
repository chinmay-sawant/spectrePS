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

	ctok struct {
		kind tokenKind
		num  float64
		text string
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
		path    []point
		hasPt   bool
		curX    float64
		curY    float64
		subX    float64
		subY    float64
		subOpen bool
		ctm     graphics.Matrix
		width   float64
		red     float64
		green   float64
		blue    float64
	}

	item struct {
		num    float64
		isNum  bool
		name   string
		isName bool
	}

	runner struct {
		marker   graphics.Marker
		scale    float64
		xobjects map[string]Value
		images   map[string]image.Image
		stack    []item
		path     []point
		hasPt    bool
		curX     float64
		curY     float64
		subX     float64
		subY     float64
		subOpen  bool
		ctm      graphics.Matrix
		width    float64
		red      float64
		green    float64
		blue     float64
		saves    []*snapshot
	}
)

const (
	tokNumber tokenKind = iota
	tokOperand
	tokOperator
	ctokName
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
	return PaintOptions{Resources: Resources{XObjects: nil}}
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
	run.xobjects = opt.Resources.XObjects
	lex := scanner{src: content, pos: 0}
	return run.play(ctx, &lex)
}

func newRunner(marker graphics.Marker, scale float64) *runner {
	return &runner{
		marker:   marker,
		scale:    scale,
		xobjects: nil,
		images:   nil,
		stack:    nil,
		path:     nil,
		hasPt:    false,
		curX:     0,
		curY:     0,
		subX:     0,
		subY:     0,
		subOpen:  false,
		ctm:      graphics.Identity(),
		width:    defaultWidth,
		red:      0,
		green:    0,
		blue:     0,
		saves:    nil,
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
		if err := run.take(tok); err != nil {
			return err
		}
	}
}

func (run *runner) take(tok ctok) error {
	if tok.kind != tokOperator {
		run.stack = append(run.stack, item{
			num:    tok.num,
			isNum:  tok.kind == tokNumber,
			name:   tok.text,
			isName: tok.kind == ctokName,
		})
		return nil
	}
	if handled, err := run.takePath(tok.text); handled {
		return err
	}
	if handled, err := run.takePaint(tok.text); handled {
		return err
	}
	if handled, err := run.takeDo(tok.text); handled {
		return err
	}
	if handled, err := run.takeState(tok.text); handled {
		return err
	}
	return NewError(tok.text, errUndefined)
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

func (run *runner) takePaint(opName string) (bool, error) {
	switch opName {
	case "S":
		run.stroke()
		return true, nil
	case "s":
		if err := run.closepath("s"); err != nil {
			return true, err
		}
		run.stroke()
		return true, nil
	case "f":
		run.fill(false)
		return true, nil
	case "f*":
		run.fill(true)
		return true, nil
	case "n":
		run.clearPath()
		return true, nil
	default:
		return false, nil
	}
}

func (run *runner) takeState(opName string) (bool, error) {
	switch opName {
	case "q":
		return true, run.save()
	case "Q":
		return true, run.restore()
	case "cm":
		return true, run.ctmConcat()
	case "w":
		return true, run.setWidth()
	case "RG", "rg":
		return true, run.setRGB(opName)
	case "G", "g":
		return true, run.setGray(opName)
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
		return lex.skipped(lex.skipString)
	case '<':
		return lex.skipped(lex.skipAngle)
	case '[':
		return lex.skipped(lex.skipArray)
	case ']', ')', '>', '{', '}':
		return zeroToken(), contentSyntax()
	default:
		return lex.scanWord()
	}
}

func (lex *scanner) skipped(skip func() error) (ctok, error) {
	if err := skip(); err != nil {
		return zeroToken(), err
	}
	return operandToken(), nil
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

func (lex *scanner) skipString() error {
	lex.pos++
	depth := 1
	for lex.pos < len(lex.src) && depth > 0 {
		cur := lex.src[lex.pos]
		lex.pos++
		next, bad := lex.stringStep(cur, depth)
		if bad {
			return contentSyntax()
		}
		depth = next
	}
	if depth != 0 {
		return contentSyntax()
	}
	return nil
}

func (lex *scanner) stringStep(cur byte, depth int) (int, bool) {
	if cur == '\\' {
		return depth, !lex.skipEscape()
	}
	if cur == '(' {
		return depth + 1, false
	}
	if cur == ')' {
		return depth - 1, false
	}
	return depth, false
}

func (lex *scanner) skipEscape() bool {
	if lex.pos >= len(lex.src) {
		return false
	}
	cur := lex.src[lex.pos]
	lex.pos++
	if cur == '\r' {
		lex.skipLF()
		return true
	}
	if contentOctal(cur) {
		lex.skipOctal()
	}
	return true
}

func (lex *scanner) skipLF() {
	if lex.pos < len(lex.src) && lex.src[lex.pos] == '\n' {
		lex.pos++
	}
}

func (lex *scanner) skipOctal() {
	extra := octalExtra
	for extra > 0 && lex.pos < len(lex.src) && contentOctal(lex.src[lex.pos]) {
		lex.pos++
		extra--
	}
}

func (lex *scanner) skipAngle() error {
	if lex.startsPair('<') {
		return lex.skipDict()
	}
	return lex.skipHex()
}

func (lex *scanner) skipHex() error {
	lex.pos++
	for lex.pos < len(lex.src) {
		if lex.src[lex.pos] == '>' {
			lex.pos++
			return nil
		}
		lex.pos++
	}
	return contentSyntax()
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
		run.marker.Stroke(run.devicePoints(), run.deviceWidth(), run.red, run.green, run.blue)
	}
	run.clearPath()
}

func (run *runner) fill(evenOdd bool) {
	if run.marker != nil {
		run.marker.Fill(run.devicePoints(), run.red, run.green, run.blue, evenOdd)
	}
	run.clearPath()
}

// deviceWidth is the stroke width in device pixels. The CTM scale matches
// the PostScript device, which uses the length of the transformed x axis.
func (run *runner) deviceWidth() float64 {
	return run.width * run.scale * ctmScale(run.ctm)
}

func (run *runner) devicePoints() []graphics.Point {
	pts := make([]graphics.Point, len(run.path))
	for idx, step := range run.path {
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
	return nil
}

func (run *runner) snap() *snapshot {
	return &snapshot{
		path:    slices.Clone(run.path),
		hasPt:   run.hasPt,
		curX:    run.curX,
		curY:    run.curY,
		subX:    run.subX,
		subY:    run.subY,
		subOpen: run.subOpen,
		ctm:     run.ctm,
		width:   run.width,
		red:     run.red,
		green:   run.green,
		blue:    run.blue,
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
	run.red = red
	run.green = green
	run.blue = blue
	return nil
}

func (run *runner) setGray(opName string) error {
	gray, err := run.popNum(opName)
	if err != nil {
		return err
	}
	run.red = gray
	run.green = gray
	run.blue = gray
	return nil
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
	if !last.isNum {
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
	if !last.isName {
		return "", NewError(opName, errType)
	}
	return last.name, nil
}

func zeroToken() ctok {
	return ctok{kind: tokNumber, num: 0, text: ""}
}

func numberToken(num float64) ctok {
	return ctok{kind: tokNumber, num: num, text: ""}
}

func operandToken() ctok {
	return ctok{kind: tokOperand, num: 0, text: ""}
}

func operatorToken(text string) ctok {
	return ctok{kind: tokOperator, num: 0, text: text}
}

func nameToken(text string) ctok {
	return ctok{kind: ctokName, num: 0, text: text}
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
