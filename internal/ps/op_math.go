package ps

import (
	"context"
	"math"
)

func registerMathOps(interp *Interp) {
	interp.Install("add", opAdd)
	interp.Install("sub", opSub)
	interp.Install("mul", opMul)
	interp.Install("div", opDiv)
	interp.Install("idiv", opIdiv)
	interp.Install("mod", opMod)
	interp.Install("neg", opNeg)
	interp.Install("abs", opAbs)
	interp.Install("ceiling", opCeiling)
	interp.Install("floor", opFloor)
	interp.Install("round", opRound)
	interp.Install("sqrt", opSqrt)
	interp.Install("cos", opCos)
	interp.Install("sin", opSin)
}

func opAdd(ctx context.Context, interp *Interp) error {
	return binaryNum(ctx, interp, "add", sumNums)
}

func opSub(ctx context.Context, interp *Interp) error {
	return binaryNum(ctx, interp, "sub", diffNums)
}

func opMul(ctx context.Context, interp *Interp) error {
	return binaryNum(ctx, interp, "mul", prodNums)
}

func binaryNum(
	ctx context.Context,
	interp *Interp,
	opName string,
	calc func(float64, float64, bool, string) (Object, error),
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	right, rightReal, err := interp.PopNum()
	if err != nil {
		return err
	}
	left, leftReal, err := interp.PopNum()
	if err != nil {
		return err
	}
	obj, err := calc(left, right, leftReal || rightReal, opName)
	if err != nil {
		return err
	}
	return interp.Push(obj)
}

func sumNums(left, right float64, asReal bool, opName string) (Object, error) {
	if asReal {
		return RealObj(left + right), nil
	}
	return fitInt(int64(left)+int64(right), opName)
}

func diffNums(left, right float64, asReal bool, opName string) (Object, error) {
	if asReal {
		return RealObj(left - right), nil
	}
	return fitInt(int64(left)-int64(right), opName)
}

func prodNums(left, right float64, asReal bool, opName string) (Object, error) {
	if asReal {
		return RealObj(left * right), nil
	}
	return fitInt(int64(left)*int64(right), opName)
}

func fitInt(value int64, opName string) (Object, error) {
	var zero Object
	if value > math.MaxInt32 || value < math.MinInt32 {
		return zero, errOf(errRangecheck, opName)
	}
	return IntObj(int32(value)), nil
}

func opDiv(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	right, _, err := interp.PopNum()
	if err != nil {
		return err
	}
	left, _, err := interp.PopNum()
	if err != nil {
		return err
	}
	if right == 0 {
		return errOf(errUndefinedResult, "div")
	}
	return interp.Push(RealObj(left / right))
}

func opIdiv(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	left, right, err := popTwoInts(interp)
	if err != nil {
		return err
	}
	if right == 0 {
		return errOf(errUndefinedResult, "idiv")
	}
	if left == math.MinInt32 && right == minusOne {
		return errOf(errRangecheck, "idiv")
	}
	return interp.Push(IntObj(left / right))
}

func opMod(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	left, right, err := popTwoInts(interp)
	if err != nil {
		return err
	}
	if right == 0 {
		return errOf(errUndefinedResult, "mod")
	}
	if left == math.MinInt32 && right == minusOne {
		return interp.Push(IntObj(0))
	}
	return interp.Push(IntObj(left % right))
}

func popTwoInts(interp *Interp) (int32, int32, error) {
	var left int32
	right, err := interp.PopInt()
	if err != nil {
		return left, right, err
	}
	left, err = interp.PopInt()
	return left, right, err
}

func opNeg(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, isReal, err := interp.PopNum()
	if err != nil {
		return err
	}
	if isReal {
		return interp.Push(RealObj(-value))
	}
	asInt := int32(value)
	if asInt == math.MinInt32 {
		return errOf(errRangecheck, "neg")
	}
	return interp.Push(IntObj(-asInt))
}

func opAbs(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, isReal, err := interp.PopNum()
	if err != nil {
		return err
	}
	if isReal {
		return interp.Push(RealObj(math.Abs(value)))
	}
	asInt := int32(value)
	if asInt == math.MinInt32 {
		return errOf(errRangecheck, "abs")
	}
	if asInt < 0 {
		asInt = -asInt
	}
	return interp.Push(IntObj(asInt))
}

func opCeiling(ctx context.Context, interp *Interp) error {
	return keepIntReal(ctx, interp, math.Ceil)
}

func opFloor(ctx context.Context, interp *Interp) error {
	return keepIntReal(ctx, interp, math.Floor)
}

func opRound(ctx context.Context, interp *Interp) error {
	return keepIntReal(ctx, interp, psRound)
}

func keepIntReal(ctx context.Context, interp *Interp, fn func(float64) float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, isReal, err := interp.PopNum()
	if err != nil {
		return err
	}
	if !isReal {
		return interp.Push(IntObj(int32(value)))
	}
	return interp.Push(RealObj(fn(value)))
}

func psRound(value float64) float64 {
	return math.Floor(value + roundHalf)
}

func opSqrt(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, _, err := interp.PopNum()
	if err != nil {
		return err
	}
	if value < 0 {
		return errOf(errUndefinedResult, "sqrt")
	}
	return interp.Push(RealObj(math.Sqrt(value)))
}

// opCos and opSin take an angle in degrees and push the real sine or cosine.
func opCos(ctx context.Context, interp *Interp) error {
	return trigDegrees(ctx, interp, "cos", math.Cos)
}

func opSin(ctx context.Context, interp *Interp) error {
	return trigDegrees(ctx, interp, "sin", math.Sin)
}

func trigDegrees(ctx context.Context, interp *Interp, _ string, fn func(float64) float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	degrees, _, err := interp.PopNum()
	if err != nil {
		return err
	}
	return interp.Push(RealObj(fn(degrees * math.Pi / halfTurnDegrees)))
}
