package ps

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const limitCheckName = "limitcheck"

func TestDict(t *testing.T) {
	assertInts(t, "/a 1 def a", 1)
	assertErrName(t, "systemdict begin /a 1 def", "invalidaccess")
	//nolint:dupword // the program runs end twice, down to systemdict
	assertErrName(t, "end end", "dictstackunderflow")
	assertErrName(t, "]", "unmatchedmark")
	assertLiteralArray(t, "[ 1 2 3 ]", 1, 2, 3)
	assertInts(t, "<< /a 1 >> begin /a load end", 1)
	assertForallPairs(t, "<< /a 1 /b 2 >> { } forall")
	assertBool(t, "[ 1 ] [ 1 ] eq", false)
	//nolint:dupword // the program compares one array object with itself
	assertBool(t, "/a [ 1 2 ] def a a eq", true)
}

func TestControl(t *testing.T) {
	assertInts(t, "true { 1 } if", 1)
	assertInts(t, "false { 1 } { 2 } ifelse", 2)
	assertInts(t, "3 { 7 } repeat", 7, 7, 7)
	assertInts(t, "1 1 3 {} for", 1, 2, 3)
	assertErrName(t, "0 0 1 {} for", "rangecheck")
	assertInts(t, "1 1 3 { exit } for", 1)
	assertErrName(t, "exit", "invalidexit")
	assertInts(t, "{ 1 2 add } exec", 3)
}

func TestBanned(t *testing.T) {
	names := []string{"file", "run", "deletefile", "renamefile", "filenameforall"}
	for _, name := range names {
		assertErrName(t, name, "invalidaccess")
	}
	assertErrName(t, "save", "undefined")
}

func TestLimits(t *testing.T) {
	assertErrName(t, "8193 { 1 } repeat", "stackoverflow")
	assertErrName(t, "/r { r } def r", limitCheckName)
	assertErrName(t, "19 { 1 dict begin } repeat", limitCheckName)
	assertErrName(t, strings.Repeat("{", 129), limitCheckName)
	assertCanceled(t)
}

func TestGraphics(t *testing.T) {
	assertFloats(t, "0 0 moveto currentpoint", 0, 0)
	assertFloats(t, "10 0 translate 0 0 moveto currentpoint", 0, 0)
}

func mustRun(t *testing.T, src string) *Interp {
	t.Helper()
	interp := NewInterp()
	registerFlowOps(interp)
	if err := interp.Run(t.Context(), []byte(src)); err != nil {
		t.Fatalf("Run(%q) error = %v", src, err)
	}
	return interp
}

func assertInts(t *testing.T, src string, want ...int32) {
	t.Helper()
	got := mustRun(t, src).Operand()
	if len(got) != len(want) {
		t.Fatalf("Run(%q) stack len = %d, want %d", src, len(got), len(want))
	}
	for idx, obj := range got {
		if obj.Kind != KindInt || obj.Int != want[idx] {
			t.Fatalf("Run(%q) stack[%d] kind %v int %d, want int %d", src, idx, obj.Kind, obj.Int, want[idx])
		}
	}
}

func assertFloats(t *testing.T, src string, want ...float64) {
	t.Helper()
	got := mustRun(t, src).Operand()
	if len(got) != len(want) {
		t.Fatalf("Run(%q) stack len = %d, want %d", src, len(got), len(want))
	}
	for idx, obj := range got {
		val, ok := scalar(obj)
		if !ok || val != want[idx] {
			t.Fatalf("Run(%q) stack[%d] = %v, want %v", src, idx, obj.Kind, want[idx])
		}
	}
}

func assertBool(t *testing.T, src string, want bool) {
	t.Helper()
	got := mustRun(t, src).Operand()
	if len(got) != 1 || got[0].Kind != KindBool || got[0].Bool != want {
		t.Fatalf("Run(%q) stack = %d objects, want bool %v", src, len(got), want)
	}
}

func assertErrName(t *testing.T, src, want string) {
	t.Helper()
	interp := NewInterp()
	registerFlowOps(interp)
	err := interp.Run(t.Context(), []byte(src))
	got, ok := psErrorName(err)
	if !ok || got != want {
		t.Fatalf("Run(%q) error = %v, want %s", src, err, want)
	}
}

func assertCanceled(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	interp := NewInterp()
	registerFlowOps(interp)
	err := interp.Run(ctx, []byte("1 2 add"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func assertLiteralArray(t *testing.T, src string, want ...int32) {
	t.Helper()
	got := mustRun(t, src).Operand()
	if len(got) != 1 {
		t.Fatalf("Run(%q) stack len = %d, want 1", src, len(got))
	}
	obj := got[0]
	if obj.Kind != KindArray || obj.Arr == nil || obj.Exec || obj.Arr.Exec {
		t.Fatalf("Run(%q) top kind %v exec %v, want literal array", src, obj.Kind, obj.Exec)
	}
	if len(obj.Arr.Elems) != len(want) {
		t.Fatalf("Run(%q) array len = %d, want %d", src, len(obj.Arr.Elems), len(want))
	}
	for idx, elem := range obj.Arr.Elems {
		if elem.Kind != KindInt || elem.Int != want[idx] {
			t.Fatalf("Run(%q) elem[%d] kind %v int %d, want int %d", src, idx, elem.Kind, elem.Int, want[idx])
		}
	}
}

func assertForallPairs(t *testing.T, src string) {
	t.Helper()
	got := mustRun(t, src).Operand()
	wantNames := []string{"a", "b"}
	wantVals := []int32{1, 2}
	if len(got) != len(wantNames)*2 {
		t.Fatalf("Run(%q) stack len = %d, want %d", src, len(got), len(wantNames)*2)
	}
	for idx, name := range wantNames {
		key := got[idx*2]
		val := got[idx*2+1]
		if key.Kind != KindName || key.Exec || key.Name != name {
			t.Fatalf("Run(%q) key[%d] kind %v name %q exec %v", src, idx, key.Kind, key.Name, key.Exec)
		}
		if val.Kind != KindInt || val.Int != wantVals[idx] {
			t.Fatalf("Run(%q) val[%d] kind %v int %d", src, idx, val.Kind, val.Int)
		}
	}
}

func scalar(obj Object) (float64, bool) {
	switch obj.Kind {
	case KindInt:
		return float64(obj.Int), true
	case KindReal:
		return obj.Real, true
	case KindNull, KindBool, KindName, KindString, KindArray, KindDict, KindMark, KindOp:
		return 0, false
	default:
		return 0, false
	}
}

func psErrorName(err error) (string, bool) {
	var psErr *Error
	if !errors.As(err, &psErr) || psErr == nil {
		return "", false
	}
	return psErr.Name, true
}
