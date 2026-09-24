package ps

import (
	"context"
	"errors"
	"math"
)

const (
	errTypecheck       = "typecheck"
	errRangecheck      = "rangecheck"
	errStackUnderflow  = "stackunderflow"
	errUndefinedResult = "undefinedresult"
	errUnmatchedMark   = "unmatchedmark"
	errUndefined       = "undefined"
	errSyntax          = "syntaxerror"

	noStringVal  = "--nostringval--"
	bitSize64    = 64
	maxByteValue = 255
	roundHalf    = 0.5
	shortestPrec = -1
	psWhitespace = " \t\r\n\f\x00"
	minusOne     = int32(-1)
)

func registerValueOps(interp *Interp) {
	registerStackOps(interp)
	registerMathOps(interp)
	registerCmpOps(interp)
	registerTypeOps(interp)
}

func registerStackOps(interp *Interp) {
	interp.Install("pop", opPop)
	interp.Install("dup", opDup)
	interp.Install("exch", opExch)
	interp.Install("index", opIndex)
	interp.Install("roll", opRoll)
	interp.Install("clear", opClear)
	interp.Install("count", opCount)
	interp.Install("mark", opMark)
	interp.Install("cleartomark", opClearToMark)
	interp.Install("counttomark", opCountToMark)
	interp.Install("copy", opCopy)
}

func isUnderflow(err error) bool {
	var psErr *Error
	if !errors.As(err, &psErr) {
		return false
	}
	return psErr.Name == errStackUnderflow
}

func asInt32(value int, opName string) (int32, error) {
	if value > math.MaxInt32 || value < math.MinInt32 {
		return 0, errOf(errRangecheck, opName)
	}
	return int32(value), nil
}

func opPop(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := interp.Pop()
	return err
}

func opDup(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if err = interp.Push(obj); err != nil {
		return err
	}
	return interp.Push(obj)
}

func opExch(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	top, err := interp.Pop()
	if err != nil {
		return err
	}
	next, err := interp.Pop()
	if err != nil {
		return err
	}
	if err = interp.Push(top); err != nil {
		return err
	}
	return interp.Push(next)
}

func opIndex(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	depth, err := interp.PopInt()
	if err != nil {
		return err
	}
	if depth < 0 {
		return errOf(errRangecheck, "index")
	}
	obj, err := interp.Peek(int(depth))
	if err != nil {
		return err
	}
	return interp.Push(obj)
}

func opRoll(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	shift, err := interp.PopInt()
	if err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
		return errOf(errRangecheck, "roll")
	}
	if count == 0 {
		return nil
	}
	return rollTop(interp, count, shift)
}

func rollTop(interp *Interp, count, shift int32) error {
	if _, err := interp.Peek(int(count - 1)); err != nil {
		return err
	}
	items, err := popBottomToTop(interp, count)
	if err != nil {
		return err
	}
	// A positive shift moves the top object down in the rolled group.
	return pushAll(interp, rotateGroup(items, shift))
}

func popBottomToTop(interp *Interp, count int32) ([]Object, error) {
	popped := make([]Object, count)
	for i := range popped {
		obj, err := interp.Pop()
		if err != nil {
			return nil, err
		}
		popped[i] = obj
	}
	ordered := make([]Object, count)
	last := int(count) - 1
	for i := range ordered {
		ordered[i] = popped[last-i]
	}
	return ordered, nil
}

func rotateGroup(items []Object, shift int32) []Object {
	size := len(items)
	if size == 0 {
		return items
	}
	step := positiveMod(shift, size)
	rotated := make([]Object, size)
	for i := range rotated {
		rotated[i] = items[(i-step+size)%size]
	}
	return rotated
}

func positiveMod(shift int32, size int) int {
	if size == 0 {
		return 0
	}
	mod := int(shift) % size
	if mod < 0 {
		return mod + size
	}
	return mod
}

func pushAll(interp *Interp, items []Object) error {
	for _, obj := range items {
		if err := interp.Push(obj); err != nil {
			return err
		}
	}
	return nil
}

func opClear(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for {
		_, err := interp.Pop()
		if err == nil {
			continue
		}
		if isUnderflow(err) {
			return nil
		}
		return err
	}
}

func opCount(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	depth := 0
	for {
		_, err := interp.Peek(depth)
		if err == nil {
			depth++
			continue
		}
		if isUnderflow(err) {
			counted, convErr := asInt32(depth, "count")
			if convErr != nil {
				return convErr
			}
			return interp.Push(IntObj(counted))
		}
		return err
	}
}

func opMark(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(MarkObj())
}

func opClearToMark(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for {
		obj, err := interp.Pop()
		if err != nil {
			if isUnderflow(err) {
				return errOf(errUnmatchedMark, "cleartomark")
			}
			return err
		}
		if obj.Kind == KindMark {
			return nil
		}
	}
}

func opCountToMark(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	depth := 0
	for {
		obj, err := interp.Peek(depth)
		if err != nil {
			if isUnderflow(err) {
				return errOf(errUnmatchedMark, "counttomark")
			}
			return err
		}
		if obj.Kind == KindMark {
			counted, convErr := asInt32(depth, "counttomark")
			if convErr != nil {
				return convErr
			}
			return interp.Push(IntObj(counted))
		}
		depth++
	}
}

func opCopy(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
		return errOf(errRangecheck, "copy")
	}
	if count == 0 {
		return nil
	}
	return copyTop(interp, count)
}

func copyTop(interp *Interp, count int32) error {
	if _, err := interp.Peek(int(count - 1)); err != nil {
		return err
	}
	items := make([]Object, count)
	last := int(count) - 1
	for i := range items {
		obj, err := interp.Peek(last - i)
		if err != nil {
			return err
		}
		items[i] = obj
	}
	return pushAll(interp, items)
}
