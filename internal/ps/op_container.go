// Package ps executes the PostScript subset from documentation/language.md.
package ps

import (
	"context"
)

const (
	dictPair    = 2
	opEndDictOp = ">>"
)

// registerFlowOps installs container, control, banned, and graphics operators.
func registerFlowOps(interp *Interp) {
	registerContainerOps(interp)
	registerControlOps(interp)
	registerBannedOps(interp)
	registerGraphicsOps(interp)
}

func registerContainerOps(interp *Interp) {
	interp.Install("array", opArray)
	interp.Install("string", opString)
	interp.Install("dict", opDict)
	interp.Install("def", opDef)
	interp.Install("load", opLoad)
	interp.Install("store", opStore)
	interp.Install("where", opWhere)
	interp.Install("known", opKnown)
	interp.Install("begin", opBegin)
	interp.Install("end", opEnd)
	interp.Install("bind", opBind)
	interp.Install("[", opMark)
	interp.Install("]", opEndArray)
	interp.Install("<<", opMark)
	interp.Install(opEndDictOp, opEndDict)
}

// opBind accepts a procedure and pushes it back unchanged. Name lookup happens
// at execution time in this interpreter, so there is nothing to bind: a name
// inside a procedure still sees the definition current when it runs.
func opBind(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if _, ok := procOf(obj); !ok {
		return errOf(errTypeCheck, "bind")
	}
	return interp.Push(obj)
}

// opArray allocates a null-filled array of the popped element count.
// A negative count and a count past maxArrayElems are both rangecheck, so a
// program asking for 2147483647 elements gets a PostScript error rather than
// an allocation failure from the Go runtime.
func opArray(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 || int64(count) > maxArrayElems {
		return errOf(errRangeCheck, "array")
	}
	return interp.Push(ArrayObj(nullArray(count), false))
}

func nullArray(count int32) []Object {
	elems := make([]Object, int(count))
	for i := range elems {
		elems[i] = NullObj()
	}
	return elems
}

// opString allocates a string of the popped byte count, filled with zero
// bytes. A negative count and a count past maxStringBytes are both rangecheck,
// so the allocation is bounded before the runtime is asked for it.
func opString(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 || int64(count) > maxStringBytes {
		return errOf(errRangeCheck, "string")
	}
	return interp.Push(StringObj(make([]byte, count), false))
}

func opDict(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
		return errOf(errRangeCheck, "dict")
	}
	return interp.Push(DictObj(newDict(false)))
}

func opDef(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	val, name, err := popValueName(interp, "def")
	if err != nil {
		return err
	}
	return interp.Define(name, val)
}

func opLoad(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := popName(interp, "load")
	if err != nil {
		return err
	}
	val, ok := interp.Lookup(name)
	if !ok {
		return errOf(errUndefined, "load")
	}
	return interp.Push(val)
}

func opStore(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	val, name, err := popValueName(interp, "store")
	if err != nil {
		return err
	}
	return interp.Store(name, val)
}

// opWhere searches the interpreter's own dictionary stack from the top down.
// It reads the same stack begin and end maintain, so it cannot drift from it.
func opWhere(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := popName(interp, "where")
	if err != nil {
		return err
	}
	for i := len(interp.dicts) - 1; i >= 0; i-- {
		if dict := interp.dicts[i]; dict != nil && dict.has(name) {
			return pushWhereHit(interp, dict)
		}
	}
	return interp.Push(BoolObj(false))
}

func pushWhereHit(interp *Interp, dict *Dict) error {
	if err := interp.Push(DictObj(dict)); err != nil {
		return err
	}
	return interp.Push(BoolObj(true))
}

func opKnown(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := popName(interp, "known")
	if err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if obj.Kind != KindDict || obj.Dict == nil {
		return errOf(errTypeCheck, "known")
	}
	return interp.Push(BoolObj(obj.Dict.has(name)))
}

func opBegin(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if obj.Kind != KindDict || obj.Dict == nil {
		return errOf(errTypeCheck, "begin")
	}
	return interp.Begin(obj.Dict)
}

func opEnd(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.End()
}

func opEndArray(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	items, err := popUntilMark(interp, "]")
	if err != nil {
		return err
	}
	return interp.Push(ArrayObj(items, false))
}

func opEndDict(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	items, err := popUntilMark(interp, opEndDictOp)
	if err != nil {
		return err
	}
	obj, err := dictFromPairs(items)
	if err != nil {
		return err
	}
	return interp.Push(obj)
}

func popValueName(interp *Interp, opName string) (Object, string, error) {
	val, err := interp.Pop()
	if err != nil {
		return zeroObj(), "", err
	}
	name, err := popName(interp, opName)
	if err != nil {
		return zeroObj(), "", err
	}
	return val, name, nil
}

func popName(interp *Interp, opName string) (string, error) {
	obj, err := interp.Pop()
	if err != nil {
		return "", err
	}
	if obj.Kind != KindName {
		return "", errOf(errTypeCheck, opName)
	}
	return obj.Name, nil
}

func popUntilMark(interp *Interp, opName string) ([]Object, error) {
	var items []Object
	for {
		obj, err := interp.Pop()
		if err != nil {
			if isErrName(err, errStackUnder) {
				return nil, errOf(errUnmatchedMark, opName)
			}
			return nil, err
		}
		if obj.Kind == KindMark {
			reverseObjs(items)
			return items, nil
		}
		items = append(items, obj)
	}
}

func reverseObjs(items []Object) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func dictFromPairs(items []Object) (Object, error) {
	if len(items)%dictPair != 0 {
		return zeroObj(), errOf(errTypeCheck, opEndDictOp)
	}
	dict := newDict(false)
	for i := 0; i < len(items); i += dictPair {
		key := items[i]
		if key.Kind != KindName {
			return zeroObj(), errOf(errTypeCheck, opEndDictOp)
		}
		dict.putNew(key.Name, items[i+1])
	}
	return DictObj(dict), nil
}
