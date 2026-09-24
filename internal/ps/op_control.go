package ps

import (
	"context"
	"errors"
	"math"
	"sync"
)

const (
	errTypeCheck     = "typecheck"
	errRangeCheck    = "rangecheck"
	errLimitCheck    = "limitcheck"
	errInvalidAccess = "invalidaccess"
	errInvalidExit   = "invalidexit"
	errNoCurrentPt   = "nocurrentpoint"
	errUndefinedRes  = "undefinedresult"
	errStackUnder    = "stackunderflow"
	exitErrName      = "exit"
	pathLimitOp      = "path"
	opForName        = "for"
	opForallName     = "forall"
)

// errExit is the private signal caught by loop, repeat, for, and forall.
// It does not escape those operators. exit outside them returns invalidexit.
var errExit = errOf(exitErrName, exitErrName)

//nolint:gochecknoglobals // loop depth is not an Interp field
var (
	loopMu     sync.Mutex
	loopDepths = map[*Interp]int{}
)

func registerControlOps(interp *Interp) {
	interp.Install("exec", opExec)
	interp.Install("if", opIf)
	interp.Install("ifelse", opIfElse)
	interp.Install("repeat", opRepeat)
	interp.Install("for", opFor)
	interp.Install("loop", opLoop)
	interp.Install("forall", opForall)
	interp.Install("exit", opExit)
}

func opExec(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return interp.Call(ctx, obj)
}

func opIf(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, cond, err := popProcBool(interp, "if")
	if err != nil {
		return err
	}
	if !cond {
		return nil
	}
	return interp.Call(ctx, proc)
}

func opIfElse(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	procFalse, err := interp.Pop()
	if err != nil {
		return err
	}
	procTrue, cond, err := popProcBool(interp, "ifelse")
	if err != nil {
		return err
	}
	if cond {
		return interp.Call(ctx, procTrue)
	}
	return interp.Call(ctx, procFalse)
}

func opRepeat(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, err := interp.Pop()
	if err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
		return errOf(errRangeCheck, "repeat")
	}
	return callN(ctx, interp, proc, int(count))
}

func opFor(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, start, inc, limit, allInt, err := popForArgs(interp)
	if err != nil {
		return err
	}
	if inc == 0 {
		return errOf(errRangeCheck, opForName)
	}
	enterLoop(interp)
	defer leaveLoop(interp)
	if allInt {
		return forInts(ctx, interp, proc, int32(start), int32(inc), int32(limit))
	}
	return forReals(ctx, interp, proc, start, inc, limit)
}

func opLoop(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, err := interp.Pop()
	if err != nil {
		return err
	}
	enterLoop(interp)
	defer leaveLoop(interp)
	for {
		stopped, callErr := callProc(ctx, interp, proc)
		if stopped || callErr != nil {
			return callErr
		}
	}
}

func opForall(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, err := interp.Pop()
	if err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	switch obj.Kind {
	case KindArray:
		return forallArray(ctx, interp, obj, proc)
	case KindString:
		return forallString(ctx, interp, obj, proc)
	case KindDict:
		return forallDict(ctx, interp, obj, proc)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindMark, KindOp:
		return errOf(errTypeCheck, opForallName)
	default:
		return errOf(errTypeCheck, opForallName)
	}
}

func opExit(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !insideLoop(interp) {
		return errOf(errInvalidExit, exitErrName)
	}
	return errExit
}

func popProcBool(interp *Interp, opName string) (Object, bool, error) {
	proc, err := interp.Pop()
	if err != nil {
		return zeroObj(), false, err
	}
	cond, err := interp.Pop()
	if err != nil {
		return zeroObj(), false, err
	}
	if cond.Kind != KindBool {
		return zeroObj(), false, errOf(errTypeCheck, opName)
	}
	return proc, cond.Bool, nil
}

func popForArgs(interp *Interp) (Object, float64, float64, float64, bool, error) {
	proc, err := interp.Pop()
	if err != nil {
		return zeroObj(), 0, 0, 0, false, err
	}
	limit, limitInt, err := popNumber(interp, opForName)
	if err != nil {
		return zeroObj(), 0, 0, 0, false, err
	}
	inc, incInt, err := popNumber(interp, opForName)
	if err != nil {
		return zeroObj(), 0, 0, 0, false, err
	}
	start, startInt, err := popNumber(interp, opForName)
	if err != nil {
		return zeroObj(), 0, 0, 0, false, err
	}
	allInt := startInt && incInt && limitInt
	return proc, start, inc, limit, allInt, nil
}

func popNumber(interp *Interp, opName string) (float64, bool, error) {
	obj, err := interp.Pop()
	if err != nil {
		return 0, false, err
	}
	switch obj.Kind {
	case KindInt:
		return float64(obj.Int), true, nil
	case KindReal:
		return obj.Real, false, nil
	case KindNull, KindBool, KindName, KindString, KindArray, KindDict, KindMark, KindOp:
		return 0, false, errOf(errTypeCheck, opName)
	default:
		return 0, false, errOf(errTypeCheck, opName)
	}
}

func callN(ctx context.Context, interp *Interp, proc Object, count int) error {
	enterLoop(interp)
	defer leaveLoop(interp)
	for range count {
		stopped, err := callProc(ctx, interp, proc)
		if stopped || err != nil {
			return err
		}
	}
	return nil
}

func forInts(ctx context.Context, interp *Interp, proc Object, start, inc, limit int32) error {
	value := start
	for intKeepsGoing(value, limit, inc) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := interp.Push(IntObj(value)); err != nil {
			return err
		}
		stopped, err := callProc(ctx, interp, proc)
		if stopped || err != nil {
			return err
		}
		next, ok := stepInt(value, inc)
		if !ok {
			return nil
		}
		value = next
	}
	return nil
}

func forReals(ctx context.Context, interp *Interp, proc Object, start, inc, limit float64) error {
	value := start
	for realKeepsGoing(value, limit, inc) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := interp.Push(RealObj(value)); err != nil {
			return err
		}
		stopped, err := callProc(ctx, interp, proc)
		if stopped || err != nil {
			return err
		}
		value += inc
	}
	return nil
}

func intKeepsGoing(value, limit, inc int32) bool {
	if inc > 0 {
		return value <= limit
	}
	return value >= limit
}

func realKeepsGoing(value, limit, inc float64) bool {
	if inc > 0 {
		return value <= limit
	}
	return value >= limit
}

func stepInt(value, inc int32) (int32, bool) {
	sum := int64(value) + int64(inc)
	if sum > math.MaxInt32 || sum < math.MinInt32 {
		return 0, false
	}
	return int32(sum), true
}

func forallArray(ctx context.Context, interp *Interp, obj, proc Object) error {
	if obj.Arr == nil {
		return errOf(errTypeCheck, opForallName)
	}
	return callEach(ctx, interp, proc, func(index int) error {
		return interp.Push(obj.Arr.Elems[index])
	}, len(obj.Arr.Elems))
}

func forallString(ctx context.Context, interp *Interp, obj, proc Object) error {
	if obj.Str == nil {
		return errOf(errTypeCheck, opForallName)
	}
	return callEach(ctx, interp, proc, func(index int) error {
		return interp.Push(IntObj(int32(obj.Str.Bytes[index])))
	}, len(obj.Str.Bytes))
}

func forallDict(ctx context.Context, interp *Interp, obj, proc Object) error {
	if obj.Dict == nil {
		return errOf(errTypeCheck, opForallName)
	}
	keys := obj.Dict.Keys()
	return callEach(ctx, interp, proc, func(index int) error {
		return pushDictEntry(interp, obj.Dict, keys[index])
	}, len(keys))
}

func pushDictEntry(interp *Interp, dict *Dict, key string) error {
	val, ok := dict.get(key)
	if !ok {
		return errOf(errUndefined, opForallName)
	}
	if err := interp.Push(LiteralName(key)); err != nil {
		return err
	}
	return interp.Push(val)
}

func callEach(ctx context.Context, interp *Interp, proc Object, push func(int) error, count int) error {
	enterLoop(interp)
	defer leaveLoop(interp)
	for index := range count {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := push(index); err != nil {
			return err
		}
		stopped, err := callProc(ctx, interp, proc)
		if stopped || err != nil {
			return err
		}
	}
	return nil
}

func callProc(ctx context.Context, interp *Interp, proc Object) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	err := interp.Call(ctx, proc)
	if isExitErr(err) {
		return true, nil
	}
	return false, err
}

func enterLoop(interp *Interp) {
	loopMu.Lock()
	defer loopMu.Unlock()
	loopDepths[interp]++
}

func leaveLoop(interp *Interp) {
	loopMu.Lock()
	defer loopMu.Unlock()
	if loopDepths[interp] > 0 {
		loopDepths[interp]--
	}
}

func insideLoop(interp *Interp) bool {
	loopMu.Lock()
	defer loopMu.Unlock()
	return loopDepths[interp] > 0
}

func isExitErr(err error) bool {
	return isErrName(err, exitErrName)
}

func isErrName(err error, name string) bool {
	var psErr *Error
	if !errors.As(err, &psErr) {
		return false
	}
	return psErr != nil && psErr.Name == name
}
