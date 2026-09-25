package pdf

import (
	"errors"
	"math"
	"sync"
)

// errCalcSyntax marks malformed type 4 calculator code. The caller maps it to
// undefined in the space operator.
var errCalcSyntax = errors.New("pdf: calculator syntax")

// Function dictionary keys and the function types this subset evaluates.
const (
	keyFunctionType = "FunctionType"
	keyDomain       = "Domain"
	keyRange        = "Range"
	keyC0           = "C0"
	keyC1           = "C1"

	funcTypeExponential = 2
	funcTypeCalculator  = 4

	// calcStackLimit is the type 4 calculator stack cap.
	calcStackLimit = 100
	// calcDepthLimit caps nested procedure execution.
	calcDepthLimit = 32
	// calcShiftLimit caps a bitshift count at 31 either way.
	calcShiftLimit = 31
	// domainPair is the two numbers one domain or range pair holds.
	domainPair = 2

	degreesPerRadian = 180 / math.Pi
)

// tintFunction evaluates one tint transform: one input value per space
// component, one output value per alternate component. A false result marks
// values the function cannot represent.
type tintFunction func(dst, in []float64) bool

// loadTintFunction resolves one tint transform for a space with inputs
// components whose alternate space has outputs components. Type 2 exponential
// interpolation and type 4 PostScript calculator functions load. A type 0
// sampled function, a type 3 stitching function, and any other value are
// undefined in opName. A type 4 code stream decodes through the shared 32 MiB
// cap.
func (file *File) loadTintFunction(
	val Value,
	inputs, outputs int,
	opName string,
) (tintFunction, error) {
	node, err := file.colorNode(val)
	if err != nil {
		return nil, err
	}
	if node.Kind != KindStream && node.Kind != KindDict {
		return nil, NewError(opName, errUndefined)
	}
	functionType, ok := node.IntEntry(keyFunctionType)
	if !ok {
		return nil, NewError(opName, errUndefined)
	}
	switch functionType {
	case funcTypeExponential:
		return exponentialTint(node, inputs, outputs, opName)
	case funcTypeCalculator:
		return file.calculatorTint(node, inputs, outputs, opName)
	default:
		return nil, NewError(opName, errUndefined)
	}
}

// exponentialTintParams holds one validated type 2 function.
type exponentialTintParams struct {
	exponent float64
	low      float64
	high     float64
	start    []float64
	end      []float64
	limits   []float64
	hasRange bool
}

// exponentialTint builds a type 2 function. Domain holds one pair per input, N
// is the exponent, and C0 and C1 are the output endpoints. The first input
// drives every output, which matches the common one-input tint transform.
func exponentialTint(node Value, inputs, outputs int, opName string) (tintFunction, error) {
	params, err := readExponentialTint(node, inputs, outputs, opName)
	if err != nil {
		return nil, err
	}
	return params.eval, nil
}

func readExponentialTint(
	node Value,
	inputs, outputs int,
	opName string,
) (exponentialTintParams, error) {
	exponent, ok := numberValue(node, keyN)
	if !ok {
		return exponentialTintParams{}, NewError(opName, errUndefined)
	}
	params := exponentialTintParams{
		exponent: exponent,
		low:      0,
		high:     1,
		start:    nil,
		end:      nil,
		limits:   nil,
		hasRange: false,
	}
	if err := readExponentialDomain(node, inputs, opName, &params); err != nil {
		return exponentialTintParams{}, err
	}
	if err := readExponentialOutputs(node, outputs, opName, &params); err != nil {
		return exponentialTintParams{}, err
	}
	if err := readExponentialRange(node, outputs, opName, &params); err != nil {
		return exponentialTintParams{}, err
	}
	return params, nil
}

// readExponentialDomain reads /Domain into the input clip range.
func readExponentialDomain(node Value, inputs int, opName string, params *exponentialTintParams) error {
	domain, hasDomain, valid := numberEntry(node, keyDomain)
	if !valid || (hasDomain && len(domain) != domainPair*inputs) {
		return NewError(opName, errUndefined)
	}
	if hasDomain && len(domain) >= domainPair {
		params.low, params.high = domain[0], domain[1]
	}
	return nil
}

// readExponentialOutputs reads /C0 and /C1, defaulting to zeros and ones.
func readExponentialOutputs(node Value, outputs int, opName string, params *exponentialTintParams) error {
	start, hasStart, valid := numberEntry(node, keyC0)
	if !valid || (hasStart && len(start) != outputs) {
		return NewError(opName, errUndefined)
	}
	end, hasEnd, valid := numberEntry(node, keyC1)
	if !valid || (hasEnd && len(end) != outputs) {
		return NewError(opName, errUndefined)
	}
	params.start = start
	if !hasStart {
		params.start = make([]float64, outputs)
	}
	params.end = end
	if !hasEnd {
		params.end = ones(outputs)
	}
	return nil
}

// readExponentialRange reads the optional output clip /Range.
func readExponentialRange(node Value, outputs int, opName string, params *exponentialTintParams) error {
	limits, hasRange, valid := numberEntry(node, keyRange)
	if !valid || (hasRange && len(limits) != domainPair*outputs) {
		return NewError(opName, errUndefined)
	}
	params.limits = limits
	params.hasRange = hasRange
	return nil
}

// eval runs the interpolation and clamps to Range when one is present.
func (params exponentialTintParams) eval(dst, in []float64) bool {
	if len(in) == 0 {
		return false
	}
	tint := clampNumber(in[0], params.low, params.high)
	weighted := tint
	if params.exponent != 1 {
		weighted = math.Pow(tint, params.exponent)
	}
	if math.IsNaN(weighted) {
		return false
	}
	for index := range dst {
		value := params.start[index] + weighted*(params.end[index]-params.start[index])
		if params.hasRange {
			value = clampNumber(value, params.limits[domainPair*index], params.limits[domainPair*index+1])
		}
		dst[index] = value
	}
	return true
}

// ones returns n ones for a missing C1 array.
func ones(count int) []float64 {
	out := make([]float64, count)
	for index := range out {
		out[index] = 1
	}
	return out
}

// calculatorTint builds a type 4 function. Domain and Range are required, and
// the Range length fixes the output count.
func (file *File) calculatorTint(
	node Value,
	inputs, outputs int,
	opName string,
) (tintFunction, error) {
	if node.Kind != KindStream {
		return nil, NewError(opName, errUndefined)
	}
	domain, hasDomain, valid := numberEntry(node, keyDomain)
	if !valid || !hasDomain || len(domain) != domainPair*inputs {
		return nil, NewError(opName, errUndefined)
	}
	limits, hasRange, valid := numberEntry(node, keyRange)
	if !valid || !hasRange || len(limits) != domainPair*outputs {
		return nil, NewError(opName, errUndefined)
	}
	code, err := decodeStream(node)
	if err != nil {
		return nil, err
	}
	program, err := parseCalculator(code)
	if err != nil {
		return nil, NewError(opName, errUndefined)
	}
	return func(dst, in []float64) bool {
		return evalCalculator(program, in, domain, dst, limits)
	}, nil
}

// calcKind is one calculator item or stack value kind.
type calcKind uint8

const (
	calcNumber calcKind = iota
	calcBool
	calcOperator
	calcProc
)

// calcItem is one parsed calculator token.
type calcItem struct {
	kind calcKind
	num  float64
	flag bool
	op   string
	proc []calcItem
}

// calcValue is one runtime calculator stack value.
type calcValue struct {
	kind calcKind
	num  float64
	flag bool
	proc []calcItem
}

func calcNumberItem(number float64) calcItem {
	return calcItem{kind: calcNumber, num: number, flag: false, op: "", proc: nil}
}

func calcBoolItem(flag bool) calcItem {
	return calcItem{kind: calcBool, num: 0, flag: flag, op: "", proc: nil}
}

func calcOperatorItem(opName string) calcItem {
	return calcItem{kind: calcOperator, num: 0, flag: false, op: opName, proc: nil}
}

func calcProcItem(proc []calcItem) calcItem {
	return calcItem{kind: calcProc, num: 0, flag: false, op: "", proc: proc}
}

func calcNumberValue(number float64) calcValue {
	return calcValue{kind: calcNumber, num: number, flag: false, proc: nil}
}

func calcBoolValue(flag bool) calcValue {
	return calcValue{kind: calcBool, num: 0, flag: flag, proc: nil}
}

func calcProcValue(proc []calcItem) calcValue {
	return calcValue{kind: calcProc, num: 0, flag: false, proc: proc}
}

func zeroCalcValue() calcValue {
	return calcValue{kind: calcNumber, num: 0, flag: false, proc: nil}
}

// parseCalculator reads a type 4 code stream. Numbers, booleans, operator
// names, and brace procedures parse. An unknown operator or a stray brace is
// an error.
func parseCalculator(src []byte) ([]calcItem, error) {
	pos := 0
	items, err := calcItems(src, &pos, false, 0)
	if err != nil {
		return nil, err
	}
	skipCalcSpace(src, &pos)
	if pos != len(src) {
		return nil, errCalcSyntax
	}
	return items, nil
}

func calcItems(src []byte, pos *int, braced bool, depth int) ([]calcItem, error) {
	if depth > calcDepthLimit {
		return nil, errCalcSyntax
	}
	items := make([]calcItem, 0)
	for {
		skipCalcSpace(src, pos)
		if *pos >= len(src) {
			if braced {
				return nil, errCalcSyntax
			}
			return items, nil
		}
		cur := src[*pos]
		if cur == '}' {
			if !braced {
				return nil, errCalcSyntax
			}
			*pos++
			return items, nil
		}
		if cur == '{' {
			sub, err := calcSubProcedure(src, pos, depth)
			if err != nil {
				return nil, err
			}
			items = append(items, calcProcItem(sub))
			continue
		}
		item, err := calcAtom(src, pos)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
}

func calcSubProcedure(src []byte, pos *int, depth int) ([]calcItem, error) {
	*pos++
	return calcItems(src, pos, true, depth+1)
}

// calcAtom reads one number, boolean, or operator name.
func calcAtom(src []byte, pos *int) (calcItem, error) {
	start := *pos
	for *pos < len(src) && !calcBreak(src[*pos]) {
		*pos++
	}
	if start == *pos {
		return calcItem{}, errCalcSyntax
	}
	text := string(src[start:*pos])
	switch text {
	case "true":
		return calcBoolItem(true), nil
	case "false":
		return calcBoolItem(false), nil
	}
	if numberSyntax(text) {
		number, ok := finiteNumber(text)
		if !ok {
			return calcItem{}, errCalcSyntax
		}
		return calcNumberItem(number), nil
	}
	if _, ok := calcOperatorFor(text); !ok {
		return calcItem{}, errCalcSyntax
	}
	return calcOperatorItem(text), nil
}

func calcBreak(cur byte) bool {
	switch cur {
	case 0, '\t', '\n', '\f', '\r', ' ', '{', '}', '%':
		return true
	default:
		return false
	}
}

// skipCalcSpace skips whitespace and percent comments.
func skipCalcSpace(src []byte, pos *int) {
	for *pos < len(src) {
		switch src[*pos] {
		case 0, '\t', '\n', '\f', '\r', ' ':
			*pos++
		case '%':
			for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
				*pos++
			}
		default:
			return
		}
	}
}

// calcOperator runs one calculator operator against the stack. depth is the
// procedure nesting depth.
type calcOpFunc func(stack *[]calcValue, depth int) bool

// calcOperatorTable holds the ISO 32000-1 table 42 operator set. It is built
// on first use, because the runners call back into the executor, and a package
// initializer would close that loop.
//
//nolint:gochecknoglobals // one lazy operator table replaces a cycle-prone initializer
var (
	calcOperatorOnce  sync.Once
	calcOperatorTable map[string]calcOpFunc
)

// calcOperatorFor returns the runner for one operator name. The bool is false
// for a name outside the table.
func calcOperatorFor(opName string) (calcOpFunc, bool) {
	calcOperatorOnce.Do(func() {
		calcOperatorTable = buildCalcOperators()
	})
	run, ok := calcOperatorTable[opName]
	return run, ok
}

func buildCalcOperators() map[string]calcOpFunc {
	return map[string]calcOpFunc{
		"abs":      func(stack *[]calcValue, _ int) bool { return calcUnary("abs", stack) },
		"add":      func(stack *[]calcValue, _ int) bool { return calcArithmetic("add", stack) },
		"atan":     func(stack *[]calcValue, _ int) bool { return calcAtan(stack) },
		"ceiling":  func(stack *[]calcValue, _ int) bool { return calcUnary("ceiling", stack) },
		"cos":      func(stack *[]calcValue, _ int) bool { return calcUnary("cos", stack) },
		"div":      func(stack *[]calcValue, _ int) bool { return calcArithmetic("div", stack) },
		"exp":      func(stack *[]calcValue, _ int) bool { return calcExp(stack) },
		"floor":    func(stack *[]calcValue, _ int) bool { return calcUnary("floor", stack) },
		"idiv":     func(stack *[]calcValue, _ int) bool { return calcInteger("idiv", stack) },
		"ln":       func(stack *[]calcValue, _ int) bool { return calcUnary("ln", stack) },
		"log":      func(stack *[]calcValue, _ int) bool { return calcUnary("log", stack) },
		"mod":      func(stack *[]calcValue, _ int) bool { return calcInteger("mod", stack) },
		"mul":      func(stack *[]calcValue, _ int) bool { return calcArithmetic("mul", stack) },
		"neg":      func(stack *[]calcValue, _ int) bool { return calcUnary("neg", stack) },
		"round":    func(stack *[]calcValue, _ int) bool { return calcUnary("round", stack) },
		"sin":      func(stack *[]calcValue, _ int) bool { return calcUnary("sin", stack) },
		"sqrt":     func(stack *[]calcValue, _ int) bool { return calcUnary("sqrt", stack) },
		"sub":      func(stack *[]calcValue, _ int) bool { return calcArithmetic("sub", stack) },
		"truncate": func(stack *[]calcValue, _ int) bool { return calcUnary("truncate", stack) },

		"and":      func(stack *[]calcValue, _ int) bool { return calcBoolean("and", stack) },
		"bitshift": func(stack *[]calcValue, _ int) bool { return calcInteger("bitshift", stack) },
		"eq":       func(stack *[]calcValue, _ int) bool { return calcRelation("eq", stack) },
		"ge":       func(stack *[]calcValue, _ int) bool { return calcRelation("ge", stack) },
		"gt":       func(stack *[]calcValue, _ int) bool { return calcRelation("gt", stack) },
		"le":       func(stack *[]calcValue, _ int) bool { return calcRelation("le", stack) },
		"lt":       func(stack *[]calcValue, _ int) bool { return calcRelation("lt", stack) },
		"ne":       func(stack *[]calcValue, _ int) bool { return calcRelation("ne", stack) },
		"not":      func(stack *[]calcValue, _ int) bool { return calcNot(stack) },
		"or":       func(stack *[]calcValue, _ int) bool { return calcBoolean("or", stack) },
		"xor":      func(stack *[]calcValue, _ int) bool { return calcBoolean("xor", stack) },

		"copy":  func(stack *[]calcValue, _ int) bool { return calcStackOp("copy", stack) },
		"dup":   func(stack *[]calcValue, _ int) bool { return calcStackOp("dup", stack) },
		"exch":  func(stack *[]calcValue, _ int) bool { return calcStackOp("exch", stack) },
		"index": func(stack *[]calcValue, _ int) bool { return calcStackOp("index", stack) },
		"pop":   func(stack *[]calcValue, _ int) bool { return calcStackOp("pop", stack) },
		"roll":  func(stack *[]calcValue, _ int) bool { return calcStackOp("roll", stack) },

		"if":     func(stack *[]calcValue, depth int) bool { return calcControl("if", stack, depth) },
		"ifelse": func(stack *[]calcValue, depth int) bool { return calcControl("ifelse", stack, depth) },
	}
}

// evalCalculator runs one parsed type 4 program. Inputs clip to Domain,
// outputs clip to Range. The stack must hold exactly one number per output.
func evalCalculator(program []calcItem, in, domain, dst, limits []float64) bool {
	inputs := len(domain) / domainPair
	if len(in) < inputs {
		return false
	}
	stack := make([]calcValue, 0, calcStackLimit)
	for index := range inputs {
		value := clampNumber(in[index], domain[domainPair*index], domain[domainPair*index+1])
		stack = append(stack, calcNumberValue(value))
	}
	if !calcExec(program, &stack, 0) {
		return false
	}
	if len(stack) != len(dst) {
		return false
	}
	for index := range dst {
		value := stack[index]
		if value.kind != calcNumber {
			return false
		}
		dst[index] = clampNumber(value.num, limits[domainPair*index], limits[domainPair*index+1])
	}
	return true
}

// calcExec runs one procedure body.
func calcExec(program []calcItem, stack *[]calcValue, depth int) bool {
	if depth > calcDepthLimit {
		return false
	}
	for _, item := range program {
		switch item.kind {
		case calcNumber, calcBool:
			if !calcPush(stack, calcValue{kind: item.kind, num: item.num, flag: item.flag, proc: nil}) {
				return false
			}
		case calcProc:
			if !calcPush(stack, calcProcValue(item.proc)) {
				return false
			}
		case calcOperator:
			if !calcApply(item.op, stack, depth) {
				return false
			}
		}
	}
	return true
}

// calcApply runs one calculator operator. A false result is an invalid
// operand, an empty stack, or a full stack.
func calcApply(opName string, stack *[]calcValue, depth int) bool {
	run, ok := calcOperatorFor(opName)
	if !ok {
		return false
	}
	return run(stack, depth)
}

func calcArithmetic(opName string, stack *[]calcValue) bool {
	right, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	left, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	switch opName {
	case "add":
		return calcPushNumber(stack, left+right)
	case "sub":
		return calcPushNumber(stack, left-right)
	case "mul":
		return calcPushNumber(stack, left*right)
	case "div":
		if right == 0 {
			return false
		}
		return calcPushNumber(stack, left/right)
	default:
		return false
	}
}

func calcUnary(opName string, stack *[]calcValue) bool {
	value, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	result, ok := unaryResult(opName, value)
	if !ok {
		return false
	}
	return calcPushNumber(stack, result)
}

// unaryResult evaluates one unary operator. The bool is false for an unknown
// operator or an out-of-domain operand.
func unaryResult(opName string, value float64) (float64, bool) {
	if result, ok := unaryBasic(opName, value); ok {
		return result, true
	}
	return unaryTranscendental(opName, value)
}

func unaryBasic(opName string, value float64) (float64, bool) {
	switch opName {
	case "neg":
		return -value, true
	case "abs":
		return math.Abs(value), true
	case "ceiling":
		return math.Ceil(value), true
	case "floor":
		return math.Floor(value), true
	case "round":
		return math.Round(value), true
	case "truncate":
		return math.Trunc(value), true
	default:
		return 0, false
	}
}

func unaryTranscendental(opName string, value float64) (float64, bool) {
	switch opName {
	case "sqrt":
		if value < 0 {
			return 0, false
		}
		return math.Sqrt(value), true
	case "sin":
		return math.Sin(value / degreesPerRadian), true
	case "cos":
		return math.Cos(value / degreesPerRadian), true
	case "log":
		if value <= 0 {
			return 0, false
		}
		return math.Log10(value), true
	case "ln":
		if value <= 0 {
			return 0, false
		}
		return math.Log(value), true
	default:
		return 0, false
	}
}

// calcAtan pops the numerator then the denominator and pushes degrees.
func calcAtan(stack *[]calcValue) bool {
	denominator, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	numerator, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	return calcPushNumber(stack, math.Atan2(numerator, denominator)*degreesPerRadian)
}

func calcExp(stack *[]calcValue) bool {
	exponent, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	base, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	return calcPushNumber(stack, math.Pow(base, exponent))
}

// calcInteger implements idiv, mod, and bitshift. The operands are whole
// numbers.
func calcInteger(opName string, stack *[]calcValue) bool {
	right, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	left, ok := calcPopNumber(stack)
	if !ok {
		return false
	}
	leftWhole, okLeft := calcWholeInt(left)
	rightWhole, okRight := calcWholeInt(right)
	if !okLeft || !okRight {
		return false
	}
	switch opName {
	case "idiv", "mod":
		return calcDivide(opName, stack, leftWhole, rightWhole)
	case "bitshift":
		return calcBitshift(stack, leftWhole, rightWhole)
	default:
		return false
	}
}

// calcDivide implements idiv and mod. A zero divisor fails.
func calcDivide(opName string, stack *[]calcValue, left, right int32) bool {
	if right == 0 {
		return false
	}
	if opName == "idiv" {
		return calcPushNumber(stack, float64(left/right))
	}
	return calcPushNumber(stack, float64(left%right))
}

// calcBitshift shifts a 32-bit value by a signed count.
func calcBitshift(stack *[]calcValue, value, count int32) bool {
	if count < -calcShiftLimit || count > calcShiftLimit {
		return false
	}
	var result int32
	if count >= 0 {
		result = value << uint(count)
	} else {
		result = value >> uint(-count)
	}
	return calcPushNumber(stack, float64(result))
}

func calcRelation(opName string, stack *[]calcValue) bool {
	right, ok := calcPop(stack)
	if !ok {
		return false
	}
	left, ok := calcPop(stack)
	if !ok {
		return false
	}
	result, ok := relationResult(opName, left, right)
	if !ok {
		return false
	}
	return calcPushBool(stack, result)
}

func relationResult(opName string, left, right calcValue) (bool, bool) {
	if opName == "eq" || opName == "ne" {
		equal := calcEqual(left, right)
		return equal == (opName == "eq"), true
	}
	if left.kind != calcNumber || right.kind != calcNumber {
		return false, false
	}
	switch opName {
	case "gt":
		return left.num > right.num, true
	case "ge":
		return left.num >= right.num, true
	case "lt":
		return left.num < right.num, true
	case "le":
		return left.num <= right.num, true
	default:
		return false, false
	}
}

func calcEqual(left, right calcValue) bool {
	if left.kind != right.kind {
		return false
	}
	switch left.kind {
	case calcNumber:
		return left.num == right.num
	case calcBool:
		return left.flag == right.flag
	case calcOperator, calcProc:
		return false
	}
	return false
}

// calcBoolean implements and, or, and xor. Boolean operands combine logically;
// whole-number operands combine bitwise.
func calcBoolean(opName string, stack *[]calcValue) bool {
	right, ok := calcPop(stack)
	if !ok {
		return false
	}
	left, ok := calcPop(stack)
	if !ok {
		return false
	}
	if left.kind == calcBool && right.kind == calcBool {
		result, ok := boolLogic(opName, left.flag, right.flag)
		if !ok {
			return false
		}
		return calcPushBool(stack, result)
	}
	return calcBitwise(opName, stack, left, right)
}

// calcBitwise implements and, or, and xor on whole numbers.
func calcBitwise(opName string, stack *[]calcValue, left, right calcValue) bool {
	if left.kind != calcNumber || right.kind != calcNumber {
		return false
	}
	leftWhole, okLeft := calcWholeInt(left.num)
	rightWhole, okRight := calcWholeInt(right.num)
	if !okLeft || !okRight {
		return false
	}
	result, ok := intLogic(opName, leftWhole, rightWhole)
	if !ok {
		return false
	}
	return calcPushNumber(stack, float64(result))
}

func boolLogic(opName string, left, right bool) (bool, bool) {
	switch opName {
	case "and":
		return left && right, true
	case "or":
		return left || right, true
	case "xor":
		return left != right, true
	default:
		return false, false
	}
}

func intLogic(opName string, left, right int32) (int32, bool) {
	switch opName {
	case "and":
		return left & right, true
	case "or":
		return left | right, true
	case "xor":
		return left ^ right, true
	default:
		return 0, false
	}
}

func calcNot(stack *[]calcValue) bool {
	value, ok := calcPop(stack)
	if !ok {
		return false
	}
	if value.kind == calcBool {
		return calcPushBool(stack, !value.flag)
	}
	if value.kind != calcNumber {
		return false
	}
	whole, ok := calcWholeInt(value.num)
	if !ok {
		return false
	}
	return calcPushNumber(stack, float64(^whole))
}

func calcStackOp(opName string, stack *[]calcValue) bool {
	switch opName {
	case "dup":
		value, ok := calcTop(stack)
		if !ok {
			return false
		}
		return calcPush(stack, value)
	case "exch":
		return calcExchange(stack)
	case "pop":
		_, ok := calcPop(stack)
		return ok
	case "copy":
		return calcCopy(stack)
	case "index":
		return calcIndex(stack)
	case "roll":
		return calcRoll(stack)
	default:
		return false
	}
}

// calcExchange swaps the top two values.
func calcExchange(stack *[]calcValue) bool {
	count := len(*stack)
	if count < domainPair {
		return false
	}
	(*stack)[count-1], (*stack)[count-2] = (*stack)[count-2], (*stack)[count-1]
	return true
}

// calcCopy pops a count and duplicates the top count values in order.
func calcCopy(stack *[]calcValue) bool {
	count, ok := calcPopWhole(stack)
	if !ok || count < 0 || count > len(*stack) {
		return false
	}
	if count == 0 {
		return true
	}
	group := (*stack)[len(*stack)-count:]
	for _, value := range group {
		if !calcPush(stack, value) {
			return false
		}
	}
	return true
}

// calcIndex pops n and pushes a copy of the value n below the top, with the
// top value at n equal to 0.
func calcIndex(stack *[]calcValue) bool {
	depth, ok := calcPopWhole(stack)
	if !ok || depth < 0 || depth >= len(*stack) {
		return false
	}
	value := (*stack)[len(*stack)-1-depth]
	return calcPush(stack, value)
}

// calcRoll pops n and j and rotates the top n values by j positions. A
// positive j moves values toward the top, as in "1 2 3 3 1 roll" = "3 1 2".
func calcRoll(stack *[]calcValue) bool {
	shift, ok := calcPopWhole(stack)
	if !ok {
		return false
	}
	count, ok := calcPopWhole(stack)
	if !ok || count < 0 || count > len(*stack) {
		return false
	}
	if count <= 1 {
		return true
	}
	shift %= count
	if shift < 0 {
		shift += count
	}
	if shift == 0 {
		return true
	}
	group := (*stack)[len(*stack)-count:]
	rotated := make([]calcValue, count)
	copy(rotated, group[count-shift:])
	copy(rotated[shift:], group[:count-shift])
	copy(group, rotated)
	return true
}

func calcControl(opName string, stack *[]calcValue, depth int) bool {
	if opName == "if" {
		body, ok := calcPopProc(stack)
		if !ok {
			return false
		}
		condition, ok := calcPopBool(stack)
		if !ok {
			return false
		}
		if !condition {
			return true
		}
		return calcExec(body, stack, depth+1)
	}
	elseBody, ok := calcPopProc(stack)
	if !ok {
		return false
	}
	thenBody, ok := calcPopProc(stack)
	if !ok {
		return false
	}
	condition, ok := calcPopBool(stack)
	if !ok {
		return false
	}
	if condition {
		return calcExec(thenBody, stack, depth+1)
	}
	return calcExec(elseBody, stack, depth+1)
}

// calcWholeInt returns one whole number as an int32.
func calcWholeInt(value float64) (int32, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int32(value), true
}

func calcTop(stack *[]calcValue) (calcValue, bool) {
	if len(*stack) == 0 {
		return zeroCalcValue(), false
	}
	return (*stack)[len(*stack)-1], true
}

func calcPop(stack *[]calcValue) (calcValue, bool) {
	if len(*stack) == 0 {
		return zeroCalcValue(), false
	}
	last := len(*stack) - 1
	value := (*stack)[last]
	*stack = (*stack)[:last]
	return value, true
}

func calcPopNumber(stack *[]calcValue) (float64, bool) {
	value, ok := calcPop(stack)
	if !ok || value.kind != calcNumber {
		return 0, false
	}
	return value.num, true
}

func calcPopWhole(stack *[]calcValue) (int, bool) {
	number, ok := calcPopNumber(stack)
	if !ok {
		return 0, false
	}
	whole, ok := calcWholeInt(number)
	if !ok {
		return 0, false
	}
	return int(whole), true
}

func calcPopBool(stack *[]calcValue) (bool, bool) {
	value, ok := calcPop(stack)
	if !ok || value.kind != calcBool {
		return false, false
	}
	return value.flag, true
}

func calcPopProc(stack *[]calcValue) ([]calcItem, bool) {
	value, ok := calcPop(stack)
	if !ok || value.kind != calcProc {
		return nil, false
	}
	return value.proc, true
}

func calcPush(stack *[]calcValue, value calcValue) bool {
	if len(*stack) >= calcStackLimit {
		return false
	}
	*stack = append(*stack, value)
	return true
}

func calcPushNumber(stack *[]calcValue, value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	return calcPush(stack, calcNumberValue(value))
}

func calcPushBool(stack *[]calcValue, flag bool) bool {
	return calcPush(stack, calcBoolValue(flag))
}
