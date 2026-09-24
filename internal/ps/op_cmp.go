package ps

import (
	"context"
	"reflect"
)

func registerCmpOps(interp *Interp) {
	interp.Install("eq", opEq)
	interp.Install("ne", opNe)
	interp.Install("gt", opGt)
	interp.Install("ge", opGe)
	interp.Install("lt", opLt)
	interp.Install("le", opLe)
	interp.Install("and", opAnd)
	interp.Install("or", opOr)
	interp.Install("not", opNot)
	interp.Install("xor", opXor)
	interp.Install("true", opTrue)
	interp.Install("false", opFalse)
}

func opEq(ctx context.Context, interp *Interp) error {
	return pushEqual(ctx, interp, true)
}

func opNe(ctx context.Context, interp *Interp) error {
	return pushEqual(ctx, interp, false)
}

func pushEqual(ctx context.Context, interp *Interp, wantEqual bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	right, err := interp.Pop()
	if err != nil {
		return err
	}
	left, err := interp.Pop()
	if err != nil {
		return err
	}
	same := objectsEqual(left, right)
	if !wantEqual {
		same = !same
	}
	return interp.Push(BoolObj(same))
}

func objectsEqual(left, right Object) bool {
	if isNumber(left) && isNumber(right) {
		return numberValue(left) == numberValue(right)
	}
	if left.Kind != right.Kind {
		return false
	}
	return sameKindEqual(left, right)
}

func isNumber(obj Object) bool {
	return obj.Kind == KindInt || obj.Kind == KindReal
}

func numberValue(obj Object) float64 {
	if obj.Kind == KindReal {
		return obj.Real
	}
	return float64(obj.Int)
}

func sameKindEqual(left, right Object) bool {
	if left.Kind == KindNull || left.Kind == KindMark {
		return true
	}
	if left.Kind == KindBool {
		return left.Bool == right.Bool
	}
	if left.Kind == KindName {
		return left.Name == right.Name
	}
	return compositeEqual(left, right)
}

func compositeEqual(left, right Object) bool {
	if left.Kind == KindString {
		return left.Str == right.Str
	}
	if left.Kind == KindArray {
		return left.Arr == right.Arr
	}
	if left.Kind == KindDict {
		return left.Dict == right.Dict
	}
	if left.Kind == KindOp {
		return sameOperator(left.Op, right.Op)
	}
	return false
}

func sameOperator(left, right Operator) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}

func opGt(ctx context.Context, interp *Interp) error {
	return relOp(ctx, interp, greater)
}

func opGe(ctx context.Context, interp *Interp) error {
	return relOp(ctx, interp, greaterEqual)
}

func opLt(ctx context.Context, interp *Interp) error {
	return relOp(ctx, interp, less)
}

func opLe(ctx context.Context, interp *Interp) error {
	return relOp(ctx, interp, lessEqual)
}

func greater(left, right float64) bool {
	return left > right
}

func greaterEqual(left, right float64) bool {
	return left >= right
}

func less(left, right float64) bool {
	return left < right
}

func lessEqual(left, right float64) bool {
	return left <= right
}

func relOp(ctx context.Context, interp *Interp, pred func(float64, float64) bool) error {
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
	return interp.Push(BoolObj(pred(left, right)))
}

func opAnd(ctx context.Context, interp *Interp) error {
	return logicOp(ctx, interp, "and", both, bitAnd)
}

func opOr(ctx context.Context, interp *Interp) error {
	return logicOp(ctx, interp, "or", either, bitOr)
}

func opXor(ctx context.Context, interp *Interp) error {
	return logicOp(ctx, interp, "xor", differ, bitXor)
}

func both(left, right bool) bool {
	return left && right
}

func either(left, right bool) bool {
	return left || right
}

func differ(left, right bool) bool {
	return left != right
}

func bitAnd(left, right int32) int32 {
	return left & right
}

func bitOr(left, right int32) int32 {
	return left | right
}

func bitXor(left, right int32) int32 {
	return left ^ right
}

func logicOp(
	ctx context.Context,
	interp *Interp,
	opName string,
	logic func(bool, bool) bool,
	bits func(int32, int32) int32,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	right, err := interp.Pop()
	if err != nil {
		return err
	}
	left, err := interp.Pop()
	if err != nil {
		return err
	}
	if left.Kind == KindBool && right.Kind == KindBool {
		return interp.Push(BoolObj(logic(left.Bool, right.Bool)))
	}
	if left.Kind == KindInt && right.Kind == KindInt {
		return interp.Push(IntObj(bits(left.Int, right.Int)))
	}
	return errOf(errTypecheck, opName)
}

func opNot(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return pushNot(interp, obj)
}

func pushNot(interp *Interp, obj Object) error {
	switch obj.Kind {
	case KindBool:
		return interp.Push(BoolObj(!obj.Bool))
	case KindInt:
		return interp.Push(IntObj(^obj.Int))
	case KindNull, KindReal, KindName, KindMark, KindString, KindArray, KindDict, KindOp:
		return errOf(errTypecheck, "not")
	default:
		return errOf(errTypecheck, "not")
	}
}

func opTrue(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(BoolObj(true))
}

func opFalse(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(BoolObj(false))
}
