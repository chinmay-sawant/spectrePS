// Package ps executes the PostScript subset from documentation/language.md.
package ps

import (
	"context"
	"sync"
)

const (
	dictPair    = 2
	opEndDictOp = ">>"
)

// dictFrames mirrors the dictionary stack for where.
// begin and end keep it aligned. Interp does not expose that stack.
//
//nolint:gochecknoglobals // where needs the stack and Interp has no dict-stack field
var (
	dictMu     sync.Mutex
	dictFrames = map[*Interp]*dictFrame{}
)

// dictFrame is the shadow dictionary stack for one interpreter.
type dictFrame struct {
	dicts  []*Dict
	seeded bool
}

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

func opArray(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
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
// bytes. A negative count is rangecheck.
func opString(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	count, err := interp.PopInt()
	if err != nil {
		return err
	}
	if count < 0 {
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

func opWhere(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := popName(interp, "where")
	if err != nil {
		return err
	}
	for _, dict := range dictsTopDown(interp) {
		if dict != nil && dict.has(name) {
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
	frame := ensureSeeded(interp)
	if err := interp.Begin(obj.Dict); err != nil {
		return err
	}
	dictMu.Lock()
	frame.dicts = append(frame.dicts, obj.Dict)
	dictMu.Unlock()
	return nil
}

func opEnd(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	frame := ensureSeeded(interp)
	if err := interp.End(); err != nil {
		return err
	}
	dictMu.Lock()
	if len(frame.dicts) > 0 {
		frame.dicts = frame.dicts[:len(frame.dicts)-1]
	}
	dictMu.Unlock()
	return nil
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

func ensureSeeded(interp *Interp) *dictFrame {
	dictMu.Lock()
	frame := dictFrames[interp]
	if frame == nil {
		frame = &dictFrame{dicts: nil, seeded: false}
		dictFrames[interp] = frame
	}
	if frame.seeded {
		dictMu.Unlock()
		return frame
	}
	dictMu.Unlock()

	sys, cur := lookupStartupDicts(interp)
	dictMu.Lock()
	defer dictMu.Unlock()
	if frame.seeded {
		return frame
	}
	frame.dicts = startupDicts(sys, cur)
	frame.seeded = true
	return frame
}

func lookupStartupDicts(interp *Interp) (*Dict, *Dict) {
	var sys *Dict
	if obj, ok := interp.Lookup("systemdict"); ok && obj.Kind == KindDict {
		sys = obj.Dict
	}
	return sys, interp.CurrentDict()
}

func startupDicts(sys, cur *Dict) []*Dict {
	var dicts []*Dict
	if sys != nil {
		dicts = append(dicts, sys)
	}
	if cur != nil && (len(dicts) == 0 || dicts[len(dicts)-1] != cur) {
		dicts = append(dicts, cur)
	}
	return dicts
}

func dictsTopDown(interp *Interp) []*Dict {
	frame := ensureSeeded(interp)
	dictMu.Lock()
	defer dictMu.Unlock()
	out := make([]*Dict, 0, len(frame.dicts))
	for i := len(frame.dicts) - 1; i >= 0; i-- {
		out = append(out, frame.dicts[i])
	}
	return out
}
