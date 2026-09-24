package ps

import (
	"errors"
	"testing"
)

func TestExecRule(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		wantOneInt(t, execSrc(t, "1 2 add"), 3)
	})
	t.Run("procedure", func(t *testing.T) {
		obj := wantExecArray(t, execSrc(t, "{ 1 2 add }"))
		wantAddBody(t, obj.Arr)
	})
	t.Run("exec", func(t *testing.T) {
		wantOneInt(t, execSrc(t, "{ 1 2 add } exec"), 3)
	})
	t.Run("nested exec", func(t *testing.T) {
		obj := wantExecArray(t, execSrc(t, "{ { 1 2 add } } exec"))
		wantAddBody(t, obj.Arr)
	})
}

func TestLateLookup(t *testing.T) {
	const src = "/test 1 def\n/proc { test } def\n/test 2 def\nproc\n"
	wantOneInt(t, execSrc(t, src), 2)
}

func execSrc(t *testing.T, src string) []Object {
	t.Helper()
	interp := NewInterp()
	err := interp.Run(t.Context(), []byte(src))
	if err != nil {
		if psErr, ok := psError(err); ok {
			t.Fatalf("Run(%q): %s in %s", src, psErr.Name, psErr.Op)
		}
		t.Fatal(err)
	}
	return interp.Operand()
}

func psError(err error) (*Error, bool) {
	var got *Error
	if !errors.As(err, &got) {
		return nil, false
	}
	return got, true
}

func wantOneInt(t *testing.T, stack []Object, want int32) {
	t.Helper()
	if len(stack) != 1 || stack[0].Kind != KindInt || stack[0].Int != want {
		t.Fatalf("stack = %#v, want int %d", stack, want)
	}
}

func wantExecArray(t *testing.T, stack []Object) Object {
	t.Helper()
	if len(stack) != 1 {
		t.Fatalf("len = %d, want 1", len(stack))
	}
	obj := stack[0]
	if obj.Kind != KindArray || obj.Arr == nil || !obj.Arr.Exec {
		t.Fatalf("got %#v, want executable array", obj)
	}
	return obj
}

func wantAddBody(t *testing.T, arr *Arr) {
	t.Helper()
	if arr == nil {
		t.Fatal("array is nil")
	}
	elems := arr.Elems
	if len(elems) != 3 {
		t.Fatalf("len(body) = %d, want 3", len(elems))
	}
	if elems[0].Kind != KindInt || elems[0].Int != 1 {
		t.Fatalf("body[0] = %#v, want 1", elems[0])
	}
	if elems[1].Kind != KindInt || elems[1].Int != 2 {
		t.Fatalf("body[1] = %#v, want 2", elems[1])
	}
	name := elems[2]
	if name.Kind != KindName || name.Name != "add" || !name.Exec {
		t.Fatalf("body[2] = %#v, want executable add", name)
	}
}
