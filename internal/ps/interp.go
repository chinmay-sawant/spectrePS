package ps

import (
	"context"
	"errors"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// Caps from the language file. Graphics caps live here so later files share one set.
const (
	maxOperand    = 8192
	maxExec       = 500
	maxDict       = 20
	maxProcNest   = 128
	maxGSave      = 32
	maxPathPoints = 100000
	// maxArrayElems and maxStringBytes bound one allocation. Without them a
	// program asks for 2147483647 elements and the runtime runs out of memory
	// instead of reporting limitcheck. An Object is about 80 bytes, so the
	// array budget is the larger of the two.
	maxArrayElems  = 1 << 20
	maxStringBytes = 1 << 25
	// maxRetainedPageBytes bounds the pages one run keeps. Every showpage
	// copies the whole page buffer, so an unbounded page count is an unbounded
	// allocation. The budget is in bytes, not pages, because the page count
	// that fills it depends on the caller's page geometry.
	maxRetainedPageBytes = 1 << 30
	// maxSteps bounds the objects one run executes, so a loop with no exit
	// stops with limitcheck instead of running until the machine gives up.
	// The interpreter retires about 22 million steps a second, so this is
	// roughly three seconds of runaway execution. The heaviest file in the
	// validation corpus uses 53,000 steps, so the headroom is over a thousand.
	maxSteps = 1 << 26
)

// bytesPerPixel is the RGB8 pixmap stride factor the page budget multiplies.
const bytesPerPixel = 3

// execFrame is one called procedure on the execution stack.
type execFrame struct {
	proc *Arr
}

// Interp executes PostScript.
// system is systemdict. dicts starts as systemdict under userdict.
// frames is the execution stack. The operand stack is stack, bottom to top.
// gfx is the graphics state, loopDepth counts the loops the program is inside,
// and steps counts the objects the run has executed. All of them live on the
// interpreter, so two interpreters share nothing and nothing outlives the
// interpreter that owns it.
type Interp struct {
	stack     []Object
	dicts     []*Dict
	system    *Dict
	frames    []execFrame
	gfx       *gstate
	loopDepth int
	steps     int
}

// NewInterp returns an interpreter with systemdict under userdict.
// Operators are written into systemdict by Install, not by def.
func NewInterp() *Interp {
	assertCaps()
	system := newDict(true)
	user := newDict(false)
	interp := &Interp{
		stack:     nil,
		dicts:     []*Dict{system, user},
		system:    system,
		frames:    nil,
		gfx:       newGState(),
		loopDepth: 0,
		steps:     0,
	}
	registerValueOps(interp)
	registerFlowOps(interp)
	system.putNew("systemdict", DictObj(system))
	system.putNew("userdict", DictObj(user))
	return interp
}

// gs returns the graphics state. It is created on first use so a hand-built
// Interp with the zero graphics field still paints.
func (ip *Interp) gs() *gstate {
	if ip.gfx == nil {
		ip.gfx = newGState()
	}
	return ip.gfx
}

// step counts one executed object or one procedure entry. Crossing maxSteps is
// limitcheck, so a loop with no exit stops on its own.
// ExecStream counts every object and callProc counts every procedure entry,
// because an empty procedure body has no object to count. A body of `{ }`
// would otherwise loop forever without the counter moving.
func (ip *Interp) step() error {
	ip.steps++
	if ip.steps > maxSteps {
		return errOf(errLimitCheck, "exec")
	}
	return nil
}

// Run scans src and executes the top-level objects. The operand stack is kept.
func (ip *Interp) Run(ctx context.Context, src []byte) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	toks, err := Scan(src)
	if err != nil {
		return err
	}
	objs, err := objectsFrom(toks)
	if err != nil {
		return err
	}
	if err := ip.ExecStream(ctx, objs); err != nil {
		return err
	}
	return ip.finishPage()
}

// UsePixmap paints stroke, fill, and showpage into pm.
// scale converts CTM points into device pixels. 72 dpi uses 1.
func (ip *Interp) UsePixmap(pm *graphics.Pixmap, scale float64) {
	state := ip.gs()
	state.pix = pm
	state.scale = scale
	if pm != nil {
		width, height := pm.PageSize()
		state.pageW = float64(width)
		state.pageH = float64(height)
	}
}

func (ip *Interp) finishPage() error {
	state := ip.gs()
	if state.pix == nil || state.pages > 0 {
		return nil
	}
	if err := state.roomForPage(); err != nil {
		return err
	}
	state.pix.ShowPage()
	state.pages++
	return nil
}

// Operand returns a copy of the operand stack from bottom to top.
func (ip *Interp) Operand() []Object {
	out := make([]Object, len(ip.stack))
	copy(out, ip.stack)
	return out
}

// Install writes an operator into systemdict. It does not call def.
func (ip *Interp) Install(name string, fn Operator) {
	obj := OpObj(fn)
	obj.Name = name
	ip.system.putNew(name, obj)
}

// Push adds obj to the operand stack. One past 8192 is stackoverflow.
func (ip *Interp) Push(obj Object) error {
	if len(ip.stack) >= maxOperand {
		return errOf("stackoverflow", "push")
	}
	ip.stack = append(ip.stack, obj)
	return nil
}

// Pop removes the top operand. An empty stack is stackunderflow.
func (ip *Interp) Pop() (Object, error) {
	n := len(ip.stack)
	if n == 0 {
		return NullObj(), errOf("stackunderflow", "pop")
	}
	last := n - 1
	obj := ip.stack[last]
	ip.stack[last] = NullObj()
	ip.stack = ip.stack[:last]
	return obj, nil
}

// Peek returns the operand depth places under the top. Depth 0 is the top.
// A negative depth is rangecheck. A missing operand is stackunderflow.
func (ip *Interp) Peek(depth int) (Object, error) {
	if depth < 0 {
		return NullObj(), errOf("rangecheck", "index")
	}
	n := len(ip.stack)
	if depth >= n {
		return NullObj(), errOf("stackunderflow", "index")
	}
	return ip.stack[n-1-depth], nil
}

// PopInt pops an integer. Any other kind is typecheck, and the operand is consumed.
func (ip *Interp) PopInt() (int32, error) {
	obj, err := ip.Pop()
	if err != nil {
		return 0, err
	}
	if obj.Kind != KindInt {
		return 0, errOf("typecheck", "pop")
	}
	return obj.Int, nil
}

// PopNum pops an int or a real.
// The bool is true when the operand was a real. Any other kind is typecheck, and the operand is consumed.
func (ip *Interp) PopNum() (float64, bool, error) {
	obj, err := ip.Pop()
	if err != nil {
		return 0, false, err
	}
	if obj.Kind == KindInt {
		return float64(obj.Int), false, nil
	}
	if obj.Kind == KindReal {
		return obj.Real, true, nil
	}
	return 0, false, errOf("typecheck", "pop")
}

// Lookup searches the dictionary stack from the top down.
func (ip *Interp) Lookup(name string) (Object, bool) {
	for i := len(ip.dicts) - 1; i >= 0; i-- {
		obj, ok := ip.dicts[i].get(name)
		if ok {
			return obj, true
		}
	}
	return NullObj(), false
}

// CurrentDict returns the top dictionary.
func (ip *Interp) CurrentDict() *Dict {
	return ip.dicts[len(ip.dicts)-1]
}

// Define puts a name in the current dictionary. systemdict returns invalidaccess.
func (ip *Interp) Define(name string, val Object) error {
	dict := ip.CurrentDict()
	if dict.systemDict() {
		return errOf("invalidaccess", "def")
	}
	dict.putNew(name, val)
	return nil
}

// Store replaces the first existing definition from the top.
// A missing name is defined in the current dictionary. Writing systemdict is invalidaccess.
func (ip *Interp) Store(name string, val Object) error {
	for i := len(ip.dicts) - 1; i >= 0; i-- {
		dict := ip.dicts[i]
		if !dict.has(name) {
			continue
		}
		if dict.systemDict() {
			return errOf("invalidaccess", "store")
		}
		dict.putNew(name, val)
		return nil
	}
	return ip.Define(name, val)
}

// Begin pushes a dictionary. More than 20 dictionaries is limitcheck.
func (ip *Interp) Begin(dict *Dict) error {
	if dict == nil {
		return errOf("typecheck", "begin")
	}
	if len(ip.dicts) >= maxDict {
		return errOf("limitcheck", "begin")
	}
	ip.dicts = append(ip.dicts, dict)
	return nil
}

// End pops a dictionary. end with only systemdict left is dictstackunderflow.
func (ip *Interp) End() error {
	if len(ip.dicts) <= 1 {
		return errOf("dictstackunderflow", "end")
	}
	last := len(ip.dicts) - 1
	ip.dicts[last] = nil
	ip.dicts = ip.dicts[:last]
	return nil
}

// Call runs an operator or an executable array. Any other object is pushed.
// The 501st nested procedure call is limitcheck.
func (ip *Interp) Call(ctx context.Context, obj Object) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if proc, ok := procOf(obj); ok {
		return ip.callProc(ctx, proc)
	}
	if obj.Kind == KindOp {
		return ip.runOp(ctx, obj)
	}
	return ip.Push(obj)
}

// ExecStream walks objects.
// An executable name is looked up and the value is executed.
// An executable array in the stream is pushed, not called.
// An operator runs. Every other object is pushed.
func (ip *Interp) ExecStream(ctx context.Context, elems []Object) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	for i := range elems {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ip.step(); err != nil {
			return err
		}
		if err := ip.execOne(ctx, elems[i]); err != nil {
			return err
		}
	}
	return nil
}

func (ip *Interp) execOne(ctx context.Context, obj Object) error {
	if obj.Kind == KindName && obj.Exec {
		return ip.execName(ctx, obj.Name)
	}
	if _, ok := procOf(obj); ok {
		return ip.Push(obj)
	}
	if obj.Kind == KindOp {
		return ip.runOp(ctx, obj)
	}
	return ip.Push(obj)
}

func (ip *Interp) execName(ctx context.Context, name string) error {
	obj, ok := ip.Lookup(name)
	if !ok {
		return errOf("undefined", name)
	}
	return ip.Call(ctx, obj)
}

func (ip *Interp) runOp(ctx context.Context, obj Object) error {
	if obj.Op == nil {
		return errOf("typecheck", "exec")
	}
	err := obj.Op(ctx, ip)
	return tagOp(err, obj.Name)
}

// tagOp reports the operator the program invoked.
// Pop and other helpers name themselves. The JobError should name add, not pop.
func tagOp(err error, opName string) error {
	if err == nil || opName == "" {
		return err
	}
	var psErr *Error
	if errors.As(err, &psErr) {
		psErr.Op = opName
	}
	return err
}

func (ip *Interp) callProc(ctx context.Context, proc *Arr) error {
	if len(ip.frames) >= maxExec {
		return errOf(errLimitCheck, "exec")
	}
	if err := ip.step(); err != nil {
		return err
	}
	ip.frames = append(ip.frames, execFrame{proc: proc})
	defer ip.popFrame()
	current := ip.frames[len(ip.frames)-1].proc
	return ip.ExecStream(ctx, current.Elems)
}

func (ip *Interp) popFrame() {
	last := len(ip.frames) - 1
	ip.frames[last] = execFrame{proc: nil}
	ip.frames = ip.frames[:last]
}

func objectsFrom(toks []Token) ([]Object, error) {
	objs, rest, err := readSeq(toks, 0, false)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, errOf("syntaxerror", "scan")
	}
	return objs, nil
}

// readSeq builds objects. depth is the number of braces already open.
// inProc is true when a closing brace ends the sequence.
func readSeq(toks []Token, depth int, inProc bool) ([]Object, []Token, error) {
	objs := make([]Object, 0, len(toks))
	for len(toks) > 0 {
		tok := toks[0]
		if tok.Kind == TokRBrace {
			if !inProc {
				return nil, nil, errOf("syntaxerror", "scan")
			}
			return objs, toks[1:], nil
		}
		if tok.Kind == TokLBrace {
			obj, rest, err := takeProc(toks[1:], depth)
			if err != nil {
				return nil, nil, err
			}
			objs = append(objs, obj)
			toks = rest
			continue
		}
		obj, err := tokenObject(tok)
		if err != nil {
			return nil, nil, err
		}
		objs = append(objs, obj)
		toks = toks[1:]
	}
	if inProc {
		return nil, nil, errOf("syntaxerror", "scan")
	}
	return objs, nil, nil
}

func takeProc(toks []Token, depth int) (Object, []Token, error) {
	if depth >= maxProcNest {
		return NullObj(), nil, errOf("limitcheck", "scan")
	}
	elems, rest, err := readSeq(toks, depth+1, true)
	if err != nil {
		return NullObj(), nil, err
	}
	return ArrayObj(elems, true), rest, nil
}

func tokenObject(tok Token) (Object, error) {
	switch tok.Kind {
	case TokInt:
		return IntObj(tok.Int), nil
	case TokReal:
		return RealObj(tok.Real), nil
	case TokName:
		return ExecName(tok.Text), nil
	case TokLiteral:
		return LiteralName(tok.Text), nil
	case TokString:
		return StringObj(tok.Bytes, false), nil
	case TokLBrace, TokRBrace:
		// readSeq owns braces. A brace here is a syntax error.
	}
	return NullObj(), errOf("syntaxerror", "scan")
}

func procOf(obj Object) (*Arr, bool) {
	if obj.Kind != KindArray || obj.Arr == nil {
		return nil, false
	}
	if obj.Arr.Exec || obj.Exec {
		return obj.Arr, true
	}
	return nil, false
}

func requireContext(ctx context.Context) error {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	return ctx.Err()
}

func assertCaps() {
	caps := [...]int{
		maxOperand,
		maxExec,
		maxDict,
		maxProcNest,
		maxGSave,
		maxPathPoints,
		maxArrayElems,
		maxStringBytes,
		maxRetainedPageBytes,
		maxSteps,
	}
	for _, lim := range caps {
		if lim <= 0 {
			panic("spectreps: invalid interpreter cap")
		}
	}
}
