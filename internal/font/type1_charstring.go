package font

import (
	"encoding/binary"
	"fmt"
)

// Type 1 charstring opcodes and interpreter caps.
const (
	t1OpHStem     = 1
	t1OpVStem     = 3
	t1OpVMoveTo   = 4
	t1OpRLineTo   = 5
	t1OpHLineTo   = 6
	t1OpVLineTo   = 7
	t1OpRRCurveTo = 8
	t1OpClosePath = 9
	t1OpCallSubr  = 10
	t1OpReturn    = 11
	t1OpEscape    = 12
	t1OpHSBW      = 13
	t1OpEndChar   = 14
	t1OpRMoveTo   = 21
	t1OpHMoveTo   = 22
	t1OpVHCurveTo = 30
	t1OpHVCurveTo = 31

	t1EscDotsection      = 0
	t1EscVStem3          = 1
	t1EscHStem3          = 2
	t1EscSeac            = 6
	t1EscSBW             = 7
	t1EscDiv             = 12
	t1EscCallOtherSubr   = 16
	t1EscPop             = 17
	t1EscSetCurrentPoint = 33

	t1OtherSubrFlexEnd   = 0
	t1OtherSubrFlexBegin = 1
	t1OtherSubrFlexPoint = 2
	t1OtherSubrHints     = 3
	t1OtherSubrBlend14   = 14
	t1OtherSubrBlend15   = 15
	t1OtherSubrBlend16   = 16
	t1OtherSubrBlend17   = 17
	t1OtherSubrBlend18   = 18

	type1MaxOperands  = 48
	type1MaxCallDepth = 10
	type1MaxPoints    = 100000
	type1EmScale      = 1000

	type1EscapeLen    = 2
	type1NumberMin    = 32
	type1NumberMax1   = 246
	type1NumberMax2   = 250
	type1NumberMax3   = 254
	type1NumberBias   = 139
	type1NumberBase2  = 247
	type1NumberBase3  = 251
	type1NumberOffset = 108
	type1NumberWeight = 256
	type1NumberLong   = 5
	type1NumberPair   = 2
	type1Operands2    = 2
	type1Operands4    = 4
	type1Operands5    = 5
	type1CurveGroup   = 6
	type1FlexPoles    = 6

	blendResults1 = 1
	blendResults2 = 2
	blendResults3 = 3
	blendResults4 = 4
	blendResults6 = 6
)

// t1Result tells run whether an operator ended the current charstring or the
// whole glyph.
type t1Result uint8

const (
	t1Continue t1Result = iota
	t1Returned
	t1Ended
)

// t1Interp interprets one charstring into segments. The zero value is not
// usable; make one with glyphNamed.
type t1Interp struct {
	font      *Type1Font
	stack     []float64
	segments  []Type1Segment
	points    int
	x         float64
	y         float64
	startX    float64
	startY    float64
	subOpen   bool
	widthX    float64
	widthY    float64
	hasWidth  bool
	sbx       float64
	flexing   bool
	flexPoles []Type1Point
	known     int
	allowSeac bool
}

// glyphNamed interprets one glyph and lets the caller decide whether a nested
// seac is legal. It is the shared body of Glyph and of seac composition.
func (f *Type1Font) glyphNamed(name string, allowSeac bool) (Type1Glyph, error) {
	code, ok := f.charStrings[name]
	if !ok {
		return Type1Glyph{}, fmt.Errorf("%w: no glyph %q", errType1Syntax, name)
	}
	in := &t1Interp{
		font:      f,
		stack:     nil,
		segments:  nil,
		points:    0,
		x:         0,
		y:         0,
		startX:    0,
		startY:    0,
		subOpen:   false,
		widthX:    0,
		widthY:    0,
		hasWidth:  false,
		sbx:       0,
		flexing:   false,
		flexPoles: nil,
		known:     0,
		allowSeac: allowSeac,
	}
	if _, err := in.run(code, 0); err != nil {
		return Type1Glyph{}, fmt.Errorf("%w: glyph %q", err, name)
	}
	return Type1Glyph{Segments: in.segments, Advance: in.advance(), HasWidth: in.hasWidth}, nil
}

// advance applies the FontMatrix to the width vector and returns 1/1000 em.
func (in *t1Interp) advance() float64 {
	if !in.hasWidth {
		return 0
	}
	matrix := in.font.FontMatrix
	return (matrix[0]*in.widthX + matrix[2]*in.widthY) * type1EmScale
}

// run interprets one charstring. The depth counts nested callsubr frames.
func (in *t1Interp) run(code []byte, depth int) (t1Result, error) {
	for pos := 0; pos < len(code); {
		next, result, err := in.step(code, pos, depth)
		if err != nil {
			return t1Continue, err
		}
		if result == t1Ended {
			return t1Ended, nil
		}
		pos = next
	}
	return t1Continue, nil
}

// step consumes one number or operator and returns the next offset.
func (in *t1Interp) step(code []byte, pos, depth int) (int, t1Result, error) {
	value := code[pos]
	if value >= type1NumberMin {
		number, size, err := t1DecodeNumber(code, pos)
		if err != nil {
			return 0, t1Continue, err
		}
		return pos + size, t1Continue, in.push(number)
	}
	if value == t1OpEscape {
		if pos+1 >= len(code) {
			return 0, t1Continue, fmt.Errorf("%w: escape at end", errType1Truncated)
		}
		err := in.escape(code[pos+1])
		return pos + type1EscapeLen, t1Continue, err
	}
	result, err := in.execute(value, depth)
	return pos + 1, result, err
}

// t1DecodeNumber decodes one Type 1 number and returns its byte length.
func t1DecodeNumber(code []byte, pos int) (float64, int, error) {
	value := code[pos]
	switch {
	case value <= type1NumberMax1:
		return float64(int(value) - type1NumberBias), 1, nil
	case value <= type1NumberMax2:
		if pos+1 >= len(code) {
			return 0, 0, fmt.Errorf("%w: number at end", errType1Truncated)
		}
		offset := (int(value)-type1NumberBase2)*type1NumberWeight + int(code[pos+1])
		return float64(offset + type1NumberOffset), type1NumberPair, nil
	case value <= type1NumberMax3:
		if pos+1 >= len(code) {
			return 0, 0, fmt.Errorf("%w: number at end", errType1Truncated)
		}
		offset := (int(value)-type1NumberBase3)*type1NumberWeight + int(code[pos+1])
		return float64(-offset - type1NumberOffset), type1NumberPair, nil
	default:
		if pos+type1NumberLong > len(code) {
			return 0, 0, fmt.Errorf("%w: long number at end", errType1Truncated)
		}
		long := code[pos+1 : pos+type1NumberLong]
		number := int32(binary.BigEndian.Uint32(long)) //nolint:gosec // the four bytes are the encoded int32
		return float64(number), type1NumberLong, nil
	}
}

// push adds one operand and enforces the operand stack cap.
func (in *t1Interp) push(value float64) error {
	if len(in.stack) >= type1MaxOperands {
		return fmt.Errorf("%w: operand stack", errType1Limit)
	}
	in.stack = append(in.stack, value)
	return nil
}

// pop removes the top operand.
func (in *t1Interp) pop() (float64, error) {
	if len(in.stack) == 0 {
		return 0, fmt.Errorf("%w: operand stack underflow", errType1Syntax)
	}
	value := in.stack[len(in.stack)-1]
	in.stack = in.stack[:len(in.stack)-1]
	return value, nil
}

// popN removes count operands and returns them bottom-most first.
func (in *t1Interp) popN(count int) ([]float64, error) {
	if count < 0 || len(in.stack) < count {
		return nil, fmt.Errorf("%w: operand stack underflow", errType1Syntax)
	}
	values := make([]float64, count)
	copy(values, in.stack[len(in.stack)-count:])
	in.stack = in.stack[:len(in.stack)-count]
	return values, nil
}

// takeLeadingOperand is the width fallback for a charstring that never ran
// hsbw or sbw: the extra operand of the first stack-clearing operator is the
// advance.
func (in *t1Interp) takeLeadingOperand() {
	in.widthX, in.widthY = in.stack[0], 0
	in.hasWidth = true
	in.stack = in.stack[1:]
}

// execute runs one single-byte operator.
func (in *t1Interp) execute(opcode byte, depth int) (t1Result, error) {
	if opcode != t1OpCallSubr && opcode != t1OpReturn {
		in.known = 0
	}
	if handled, err := in.executeMove(opcode); handled {
		return t1Continue, err
	}
	if handled, err := in.executeDraw(opcode); handled {
		return t1Continue, err
	}
	if result, handled, err := in.executeFlow(opcode, depth); handled {
		return result, err
	}
	return t1Continue, fmt.Errorf("%w: operator %d", errType1Syntax, opcode)
}

// executeMove runs the moveto family, including the width fallback.
func (in *t1Interp) executeMove(opcode byte) (bool, error) {
	switch opcode {
	case t1OpVMoveTo, t1OpHMoveTo:
		if !in.hasWidth && len(in.stack) == type1Operands2 {
			in.takeLeadingOperand()
		}
		return true, in.moveOne(opcode)
	case t1OpRMoveTo:
		if !in.hasWidth && len(in.stack) == type1Operands2+1 {
			in.takeLeadingOperand()
		}
		return true, in.rmoveTo()
	default:
		return false, nil
	}
}

// moveOne runs vmoveto or hmoveto.
func (in *t1Interp) moveOne(opcode byte) error {
	args, err := in.popN(1)
	if err != nil {
		return err
	}
	if opcode == t1OpHMoveTo {
		return in.move(args[0], 0)
	}
	return in.move(0, args[0])
}

// executeDraw runs the line, curve, and close operators.
func (in *t1Interp) executeDraw(opcode byte) (bool, error) {
	switch opcode {
	case t1OpRLineTo:
		return true, in.rlineTo()
	case t1OpHLineTo:
		return true, in.alternateLine(true)
	case t1OpVLineTo:
		return true, in.alternateLine(false)
	case t1OpRRCurveTo:
		return true, in.rrcurveto()
	case t1OpClosePath:
		return true, in.closepath()
	case t1OpVHCurveTo:
		return true, in.vhcurveto(false)
	case t1OpHVCurveTo:
		return true, in.vhcurveto(true)
	default:
		return false, nil
	}
}

// executeFlow runs the hint, subroutine, and character end operators.
func (in *t1Interp) executeFlow(opcode byte, depth int) (t1Result, bool, error) {
	switch opcode {
	case t1OpHStem, t1OpVStem:
		in.stack = in.stack[:0]
		return t1Continue, true, nil
	case t1OpCallSubr:
		result, err := in.callsubr(depth)
		return result, true, err
	case t1OpReturn:
		return t1Returned, true, nil
	case t1OpHSBW:
		return t1Continue, true, in.hsbw()
	case t1OpEndChar:
		return in.endchar(), true, nil
	default:
		return t1Continue, false, nil
	}
}

// endchar finishes the glyph and records the charstring width.
func (in *t1Interp) endchar() t1Result {
	if !in.hasWidth && len(in.stack) == 1 {
		in.takeLeadingOperand()
	}
	in.stack = in.stack[:0]
	return t1Ended
}

// escape runs one two-byte operator.
func (in *t1Interp) escape(sub byte) error {
	if sub == t1EscPop {
		return in.popResult()
	}
	in.known = 0
	switch sub {
	case t1EscDotsection, t1EscVStem3, t1EscHStem3:
		in.stack = in.stack[:0]
		return nil
	case t1EscSeac:
		return in.seac()
	case t1EscSBW:
		return in.sbw()
	case t1EscDiv:
		return in.divide()
	case t1EscCallOtherSubr:
		return in.callothersubr()
	case t1EscSetCurrentPoint:
		return in.setcurrentpoint()
	default:
		return fmt.Errorf("%w: escape %d", errType1Syntax, sub)
	}
}

// move updates the current point. During a flex the moveto adds a flex pole
// and leaves the outline alone.
func (in *t1Interp) move(dx, dy float64) error {
	in.x += dx
	in.y += dy
	if in.flexing {
		in.flexPoles = append(in.flexPoles, Type1Point{X: in.x, Y: in.y})
		return nil
	}
	if err := in.emit(Type1MoveTo, Type1Point{X: in.x, Y: in.y}); err != nil {
		return err
	}
	in.startX, in.startY = in.x, in.y
	in.subOpen = true
	return nil
}

// beginSubpath emits the implicit moveto a line or curve needs when the
// previous command closed the subpath or a setcurrentpoint moved it.
func (in *t1Interp) beginSubpath() error {
	if in.subOpen {
		return nil
	}
	if err := in.emit(Type1MoveTo, Type1Point{X: in.x, Y: in.y}); err != nil {
		return err
	}
	in.startX, in.startY = in.x, in.y
	in.subOpen = true
	return nil
}

// emit appends one segment and enforces the total point cap.
func (in *t1Interp) emit(opcode Type1Op, points ...Type1Point) error {
	if in.points+len(points) > type1MaxPoints {
		return fmt.Errorf("%w: outline points", errType1Limit)
	}
	in.points += len(points)
	args := make([]Type1Point, len(points))
	copy(args, points)
	in.segments = append(in.segments, Type1Segment{Op: opcode, Args: args})
	return nil
}

// rmoveTo is the rmoveto operator.
func (in *t1Interp) rmoveTo() error {
	args, err := in.popN(type1Operands2)
	if err != nil {
		return err
	}
	return in.move(args[0], args[1])
}

// lineBy draws one relative line.
func (in *t1Interp) lineBy(dx, dy float64) error {
	if err := in.beginSubpath(); err != nil {
		return err
	}
	in.x += dx
	in.y += dy
	return in.emit(Type1LineTo, Type1Point{X: in.x, Y: in.y})
}

// rlineTo consumes the stack in dx dy pairs.
func (in *t1Interp) rlineTo() error {
	if len(in.stack) == 0 || len(in.stack)%type1Operands2 != 0 {
		return fmt.Errorf("%w: rlineto operands", errType1Syntax)
	}
	args, err := in.popN(len(in.stack))
	if err != nil {
		return err
	}
	for index := 0; index < len(args); index += type1Operands2 {
		if err := in.lineBy(args[index], args[index+1]); err != nil {
			return err
		}
	}
	return nil
}

// alternateLine implements hlineto and vlineto, which alternate direction
// over the whole operand stack.
func (in *t1Interp) alternateLine(horizontal bool) error {
	for len(in.stack) > 0 {
		value, err := in.pop()
		if err != nil {
			return err
		}
		if horizontal {
			err = in.lineBy(value, 0)
		} else {
			err = in.lineBy(0, value)
		}
		if err != nil {
			return err
		}
		horizontal = !horizontal
	}
	return nil
}

// curveBy draws one relative cubic curve.
func (in *t1Interp) curveBy(dx1, dy1, dx2, dy2, dx3, dy3 float64) error {
	if err := in.beginSubpath(); err != nil {
		return err
	}
	ctrl1X, ctrl1Y := in.x+dx1, in.y+dy1
	ctrl2X, ctrl2Y := ctrl1X+dx2, ctrl1Y+dy2
	endX, endY := ctrl2X+dx3, ctrl2Y+dy3
	err := in.emit(Type1CurveTo,
		Type1Point{X: ctrl1X, Y: ctrl1Y},
		Type1Point{X: ctrl2X, Y: ctrl2Y},
		Type1Point{X: endX, Y: endY})
	if err != nil {
		return err
	}
	in.x, in.y = endX, endY
	return nil
}

// rrcurveto consumes the stack in groups of six.
func (in *t1Interp) rrcurveto() error {
	if len(in.stack) == 0 || len(in.stack)%type1CurveGroup != 0 {
		return fmt.Errorf("%w: rrcurveto operands", errType1Syntax)
	}
	args, err := in.popN(len(in.stack))
	if err != nil {
		return err
	}
	for index := 0; index < len(args); index += type1CurveGroup {
		err := in.curveBy(args[index], args[index+1], args[index+2],
			args[index+3], args[index+4], args[index+5])
		if err != nil {
			return err
		}
	}
	return nil
}

// closepath closes the open subpath without moving the current point.
func (in *t1Interp) closepath() error {
	if !in.subOpen {
		return nil
	}
	err := in.emit(Type1LineTo, Type1Point{X: in.startX, Y: in.startY})
	in.subOpen = false
	return err
}

// vhcurveto implements vhcurveto and hvcurveto, which alternate curve
// direction and may end with one extra coordinate.
func (in *t1Interp) vhcurveto(horizontalFirst bool) error {
	if len(in.stack) == 0 {
		return fmt.Errorf("%w: vhcurveto operands", errType1Syntax)
	}
	args, err := in.popN(len(in.stack))
	if err != nil {
		return err
	}
	for len(args) > 0 {
		if horizontalFirst {
			args, err = in.hcurve(args)
		} else {
			args, err = in.vcurve(args)
		}
		if err != nil {
			return err
		}
		if len(args) == 0 {
			break
		}
		if horizontalFirst {
			args, err = in.vcurve(args)
		} else {
			args, err = in.hcurve(args)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// hcurve reads one horizontal-vertical curve group.
func (in *t1Interp) hcurve(args []float64) ([]float64, error) {
	if len(args) < type1Operands4 {
		return nil, fmt.Errorf("%w: hvcurveto operands", errType1Syntax)
	}
	rest := args[type1Operands4:]
	dxc := 0.0
	if len(rest) == 1 {
		dxc, rest = rest[0], nil
	}
	err := in.curveBy(args[0], 0, args[1], args[2], dxc, args[3])
	return rest, err
}

// vcurve reads one vertical-horizontal curve group.
func (in *t1Interp) vcurve(args []float64) ([]float64, error) {
	if len(args) < type1Operands4 {
		return nil, fmt.Errorf("%w: vhcurveto operands", errType1Syntax)
	}
	rest := args[type1Operands4:]
	dyc := 0.0
	if len(rest) == 1 {
		dyc, rest = rest[0], nil
	}
	err := in.curveBy(0, args[0], args[1], args[2], args[3], dyc)
	return rest, err
}

// hsbw sets the sidebearing and the horizontal width.
func (in *t1Interp) hsbw() error {
	args, err := in.popN(type1Operands2)
	if err != nil {
		return err
	}
	in.setWidth(args[1], 0)
	in.sbx = args[0]
	in.x = args[0]
	return nil
}

// sbw sets the sidebearing and the width vector.
func (in *t1Interp) sbw() error {
	args, err := in.popN(type1Operands4)
	if err != nil {
		return err
	}
	in.setWidth(args[2], args[3])
	in.sbx = args[0]
	in.x, in.y = args[0], args[1]
	return nil
}

// setWidth records the first width the charstring carries.
func (in *t1Interp) setWidth(x, y float64) {
	if in.hasWidth {
		return
	}
	in.widthX, in.widthY = x, y
	in.hasWidth = true
}

// callsubr runs one Subrs entry with the index or its negative form.
func (in *t1Interp) callsubr(depth int) (t1Result, error) {
	if depth >= type1MaxCallDepth {
		return t1Continue, fmt.Errorf("%w: call depth", errType1Limit)
	}
	index, err := in.pop()
	if err != nil {
		return t1Continue, err
	}
	code, err := in.font.subr(int(index))
	if err != nil {
		return t1Continue, err
	}
	return in.run(code, depth+1)
}

// subr resolves a Subrs index. A negative index counts from the end.
func (f *Type1Font) subr(index int) ([]byte, error) {
	if index < 0 {
		index += len(f.subrs)
	}
	if index < 0 || index >= len(f.subrs) || f.subrs[index] == nil {
		return nil, fmt.Errorf("%w: subr %d", errType1Syntax, index)
	}
	return f.subrs[index], nil
}

// divide implements the charstring div operator.
func (in *t1Interp) divide() error {
	args, err := in.popN(type1Operands2)
	if err != nil {
		return err
	}
	if args[1] == 0 {
		return fmt.Errorf("%w: div by zero", errType1Syntax)
	}
	return in.push(args[0] / args[1])
}

// setcurrentpoint moves the current point without touching the outline.
func (in *t1Interp) setcurrentpoint() error {
	args, err := in.popN(type1Operands2)
	if err != nil {
		return err
	}
	in.x, in.y = args[0], args[1]
	in.subOpen = false
	return nil
}

// popResult implements the charstring pop operator. A pop after a known
// OtherSubr leaves that result in place for the following command.
func (in *t1Interp) popResult() error {
	if in.known > 0 {
		in.known--
		return nil
	}
	_, err := in.pop()
	return err
}

// callothersubr handles the flex, hint replacement, and Multiple Master
// blend entries, and keeps the stack balanced for anything unknown.
func (in *t1Interp) callothersubr() error {
	subr, args, err := in.otherSubrArgs()
	if err != nil {
		return err
	}
	switch int(subr) {
	case t1OtherSubrFlexEnd:
		return in.flexEnd(args)
	case t1OtherSubrFlexBegin:
		return in.flexBegin(args)
	case t1OtherSubrFlexPoint:
		return in.flexPoint(args)
	case t1OtherSubrHints:
		return in.hintReplacement(args)
	default:
		return in.otherSubrExtra(int(subr), args)
	}
}

// otherSubrArgs pops the entry number, the count, and the count arguments.
func (in *t1Interp) otherSubrArgs() (float64, []float64, error) {
	subr, err := in.pop()
	if err != nil {
		return 0, nil, err
	}
	count, err := in.pop()
	if err != nil {
		return 0, nil, err
	}
	if count < 0 || count != float64(int(count)) || count > type1MaxOperands {
		return 0, nil, fmt.Errorf("%w: callothersubr count %v", errType1Syntax, count)
	}
	args, err := in.popN(int(count))
	if err != nil {
		return 0, nil, err
	}
	return subr, args, nil
}

// otherSubrExtra handles the Multiple Master entries and the unknown ones.
func (in *t1Interp) otherSubrExtra(subr int, args []float64) error {
	switch subr {
	case t1OtherSubrBlend14, t1OtherSubrBlend15, t1OtherSubrBlend16,
		t1OtherSubrBlend17, t1OtherSubrBlend18:
		return in.blend(subr, args)
	default:
		return in.pushAll(args)
	}
}

// pushAll returns the arguments to the operand stack, which is how the
// specification keeps unknown OtherSubrs balanced.
func (in *t1Interp) pushAll(args []float64) error {
	for _, value := range args {
		if err := in.push(value); err != nil {
			return err
		}
	}
	return nil
}

// flexBegin starts a flex and records the current point as its first pole.
func (in *t1Interp) flexBegin(args []float64) error {
	if len(args) != 0 {
		return fmt.Errorf("%w: flex begin arguments", errType1Syntax)
	}
	in.flexing = true
	in.flexPoles = append(in.flexPoles[:0], Type1Point{X: in.x, Y: in.y})
	return nil
}

// flexPoint records one flex point. The pole itself comes from the moveto
// that precedes it, matching the Flex mechanism in the Type 1 format.
func (in *t1Interp) flexPoint(args []float64) error {
	if len(args) != 0 || !in.flexing {
		return fmt.Errorf("%w: flex point", errType1Syntax)
	}
	return nil
}

// flexEnd emits the two flex curves from the last six poles, then leaves the
// final point on the stack for the following pop and setcurrentpoint.
func (in *t1Interp) flexEnd(args []float64) error {
	if len(args) != 3 || !in.flexing || len(in.flexPoles) < type1FlexPoles {
		return fmt.Errorf("%w: flex end", errType1Syntax)
	}
	poles := in.flexPoles[len(in.flexPoles)-type1FlexPoles:]
	for index := 0; index < type1FlexPoles; index += 3 {
		if err := in.emit(Type1CurveTo, poles[index], poles[index+1], poles[index+2]); err != nil {
			return err
		}
	}
	in.flexing = false
	in.flexPoles = nil
	in.x, in.y = args[1], args[2]
	in.subOpen = false
	if err := in.push(args[1]); err != nil {
		return err
	}
	if err := in.push(args[2]); err != nil {
		return err
	}
	in.known = 2
	return nil
}

// hintReplacement handles OtherSubr 3. The hint subroutine is executed by the
// callsubr that follows the sequence; this operator returns the index for it
// and drops the old hints, which this decoder never kept.
func (in *t1Interp) hintReplacement(args []float64) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: hint replacement", errType1Syntax)
	}
	if err := in.push(args[0]); err != nil {
		return err
	}
	in.known = 1
	return nil
}

// blend implements OtherSubrs 14 through 18, the Multiple Master weighted
// averages. A font without a /WeightVector leaves the arguments in place.
func (in *t1Interp) blend(subr int, args []float64) error {
	results := t1BlendResults(subr)
	weight := in.font.weight
	if len(weight) == 0 || results == 0 {
		return in.pushAll(args)
	}
	if len(args) != len(weight)*results {
		return fmt.Errorf("%w: blend %d operands", errType1Syntax, subr)
	}
	if err := in.pushAll(t1BlendValues(results, weight, args)); err != nil {
		return err
	}
	in.known = results
	return nil
}

// t1BlendResults returns the number of results one blend entry produces.
func t1BlendResults(subr int) int {
	switch subr {
	case t1OtherSubrBlend14:
		return blendResults1
	case t1OtherSubrBlend15:
		return blendResults2
	case t1OtherSubrBlend16:
		return blendResults3
	case t1OtherSubrBlend17:
		return blendResults4
	case t1OtherSubrBlend18:
		return blendResults6
	default:
		return 0
	}
}

// t1BlendValues applies the WeightVector deltas of one blend entry.
func t1BlendValues(results int, weight, args []float64) []float64 {
	values := make([]float64, results)
	for result := range results {
		value := args[result]
		offset := results + result*(len(weight)-1)
		for index := 1; index < len(weight); index++ {
			value += args[offset+index-1] * weight[index]
		}
		values[result] = value
	}
	return values
}

// seac composes the base and accent charstrings. The accent offset follows
// fontTools: the accent is translated by adx + sbx - asb, where sbx is the
// sidebearing of the accented charstring and asb the accent's own sidebearing.
func (in *t1Interp) seac() error {
	if !in.allowSeac {
		return fmt.Errorf("%w: nested seac", errType1Syntax)
	}
	args, err := in.popN(type1Operands5)
	if err != nil {
		return err
	}
	asb, adx, ady := args[0], args[1], args[2]
	baseName := t1StandardCode(int(args[3]))
	accentName := t1StandardCode(int(args[4]))
	if baseName == "" || accentName == "" {
		return fmt.Errorf("%w: seac code", errType1Syntax)
	}
	base, err := in.font.glyphNamed(baseName, false)
	if err != nil {
		return err
	}
	accent, err := in.font.glyphNamed(accentName, false)
	if err != nil {
		return err
	}
	offsetX := adx + in.sbx - asb
	in.segments = append(in.segments, base.Segments...)
	translated := make([]Type1Segment, 0, len(accent.Segments))
	for _, segment := range accent.Segments {
		translated = append(translated, translate1Segment(segment, offsetX, ady))
	}
	in.segments = append(in.segments, translated...)
	in.points += len(base.Segments) + len(translated)
	if base.HasWidth {
		in.widthX, in.widthY, in.hasWidth = base.Advance, 0, true
	}
	return nil
}

// t1StandardCode maps a seac character code through StandardEncoding.
func t1StandardCode(code int) string {
	if code < 0 || code > 255 {
		return ""
	}
	return EncodingStandard.GlyphName(byte(code))
}

// translate1Segment moves one segment by a fixed offset.
func translate1Segment(segment Type1Segment, dx, dy float64) Type1Segment {
	args := make([]Type1Point, len(segment.Args))
	for index, point := range segment.Args {
		args[index] = Type1Point{X: point.X + dx, Y: point.Y + dy}
	}
	return Type1Segment{Op: segment.Op, Args: args}
}
