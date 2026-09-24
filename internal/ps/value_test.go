package ps

import (
	"errors"
	"testing"
)

func TestStack(t *testing.T) {
	t.Run("pop underflow", func(t *testing.T) {
		assertPSError(t, "pop", "stackunderflow")
	})
	t.Run("dup", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 dup"), 1, 1)
	})
	t.Run("exch", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 exch"), 2, 1)
	})
	t.Run("copy", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 3 2 copy"), 1, 2, 3, 2, 3)
	})
	t.Run("copy typecheck", func(t *testing.T) {
		assertPSError(t, "1 2 3.5 copy", "typecheck")
	})
	t.Run("unmatched mark", func(t *testing.T) {
		assertPSError(t, "]", "unmatchedmark")
	})
	t.Run("cleartomark", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 mark 2 3 cleartomark"), 1)
	})
	t.Run("count", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 count"), 1, 2, 2)
	})
}

func TestMath(t *testing.T) {
	t.Run("div", func(t *testing.T) {
		const threeHalves = 1.5
		assertReal(t, assertRun(t, "3 2 div"), threeHalves)
	})
	t.Run("div zero", func(t *testing.T) {
		assertPSError(t, "1 0 div", "undefinedresult")
	})
	t.Run("add overflow", func(t *testing.T) {
		assertPSError(t, "2147483647 1 add", "rangecheck")
	})
	t.Run("add", func(t *testing.T) {
		assertIntValues(t, assertRun(t, "1 2 add"), 3)
	})
	t.Run("sqrt negative", func(t *testing.T) {
		assertPSError(t, "-4 sqrt", "undefinedresult")
	})
}

func assertRun(t *testing.T, src string) []Object {
	t.Helper()
	interp := NewInterp()
	err := interp.Run(t.Context(), []byte(src))
	if err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	return interp.Operand()
}

func assertPSError(t *testing.T, src, name string) {
	t.Helper()
	interp := NewInterp()
	err := interp.Run(t.Context(), []byte(src))
	var psErr *Error
	if !errors.As(err, &psErr) {
		t.Fatalf("run %q error = %v, want %s", src, err, name)
	}
	if psErr.Name != name {
		t.Fatalf("run %q name = %s, want %s", src, psErr.Name, name)
	}
}

func assertIntValues(t *testing.T, got []Object, want ...int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Kind != KindInt || got[i].Int != want[i] {
			t.Fatalf("stack[%d] = %+v, want int %d", i, got[i], want[i])
		}
	}
}

func assertReal(t *testing.T, got []Object, want float64) {
	t.Helper()
	if len(got) != 1 || got[0].Kind != KindReal || got[0].Real != want {
		t.Fatalf("got %+v, want real %v", got, want)
	}
}
