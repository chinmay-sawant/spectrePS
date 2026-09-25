package psout

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	lineSrc = "0 0 m 10 0 l S"
	rectSrc = "2 0 0 2 1 1 cm 0 0 10 10 re S"
	fillSrc = "1 0 0 rg 0 0 10 10 re f"
	eoSrc   = "0.25 g 0 0 10 10 re f*"
)

func TestEmitPS(t *testing.T) {
	t.Run("re cm S", checkEmitRectStroke)
	t.Run("line", checkEmitLine)
	t.Run("fill", checkEmitFill)
	t.Run("eofill", checkEmitEOFill)
	t.Run("empty", checkEmitEmpty)
	t.Run("stable", checkEmitStable)
	t.Run("undefined", checkEmitUndefined)
	t.Run("canceled", checkEmitCanceled)
	t.Run("nil context", checkEmitNilContext)
}

// checkEmitRectStroke checks the operator text for a re, cm, and S input. The
// matrix doubles the rectangle and translates it by one point.
func checkEmitRectStroke(t *testing.T) {
	t.Helper()
	const want = "0 setgray\n" +
		"2 setlinewidth\n" +
		"1 1 m\n" +
		"21 1 l\n" +
		"21 21 l\n" +
		"1 21 l\n" +
		"1 1 l\n" +
		"S\n"
	if got := string(mustEmit(t, rectSrc)); got != want {
		t.Fatalf("emit %q = %q, want %q", rectSrc, got, want)
	}
}

func checkEmitLine(t *testing.T) {
	t.Helper()
	const want = "0 setgray\n" +
		"1 setlinewidth\n" +
		"0 0 m\n" +
		"10 0 l\n" +
		"S\n"
	if got := string(mustEmit(t, lineSrc)); got != want {
		t.Fatalf("emit %q = %q, want %q", lineSrc, got, want)
	}
}

func checkEmitFill(t *testing.T) {
	t.Helper()
	const want = "1 0 0 setrgbcolor\n" +
		"0 0 m\n" +
		"10 0 l\n" +
		"10 10 l\n" +
		"0 10 l\n" +
		"0 0 l\n" +
		"f\n"
	if got := string(mustEmit(t, fillSrc)); got != want {
		t.Fatalf("emit %q = %q, want %q", fillSrc, got, want)
	}
}

func checkEmitEOFill(t *testing.T) {
	t.Helper()
	const want = "0.25 setgray\n" +
		"0 0 m\n" +
		"10 0 l\n" +
		"10 10 l\n" +
		"0 10 l\n" +
		"0 0 l\n" +
		"f*\n"
	if got := string(mustEmit(t, eoSrc)); got != want {
		t.Fatalf("emit %q = %q, want %q", eoSrc, got, want)
	}
}

func checkEmitEmpty(t *testing.T) {
	t.Helper()
	checkOneEmpty(t, nil)
	checkOneEmpty(t, []byte{})
}

func checkOneEmpty(t *testing.T, src []byte) {
	t.Helper()
	got, err := Emit(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("emit = %q, want empty non-nil", got)
	}
}

func checkEmitStable(t *testing.T) {
	t.Helper()
	if !bytes.Equal(mustEmit(t, lineSrc), mustEmit(t, lineSrc)) {
		t.Fatal("two emits returned different bytes")
	}
}

func checkEmitUndefined(t *testing.T) {
	t.Helper()
	_, err := Emit(t.Context(), []byte("(Hi) Tj"))
	var job *pdf.Error
	if !errors.As(err, &job) || job.Error() != "undefined" {
		t.Fatalf("error = %v, want undefined", err)
	}
}

func checkEmitCanceled(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := Emit(ctx, []byte(lineSrc))
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("emit = %v, %v, want nil slice and context.Canceled", got, err)
	}
}

func checkEmitNilContext(t *testing.T) {
	t.Helper()
	defer func() {
		got := recover()
		text, ok := got.(string)
		if !ok || text != "psout: nil context" {
			t.Fatalf("panic = %v, want psout: nil context", got)
		}
	}()
	_, _ = Emit(nil, nil) //nolint:staticcheck // nil context is the case under test
	t.Fatal("Emit returned")
}

func mustEmit(t *testing.T, src string) []byte {
	t.Helper()
	got, err := Emit(t.Context(), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("emit returned a nil slice")
	}
	return got
}
