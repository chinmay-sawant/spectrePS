package pdfout

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	emitSide = 20
	lineSrc  = "0 0 m 10 0 l S"
)

func TestEmit(t *testing.T) {
	t.Run("line", checkEmitLine)
	for _, src := range emitSources() {
		t.Run(src, func(t *testing.T) {
			matchEmit(t, src, mustEmit(t, src))
		})
	}
	t.Run("undefined", checkEmitUndefined)
	t.Run("empty", checkEmitEmpty)
	t.Run("stable", checkEmitStable)
	t.Run("canceled", checkEmitCanceled)
	t.Run("nil context", checkEmitNilContext)
}

func emitSources() []string {
	return []string{
		"1 0 0 rg 0 0 10 10 re f",
		"0 0 m 0 10 10 10 10 0 c S",
		"q 0 1 0 rg 0 0 10 10 re f Q",
		"1 0 0 rg q 0 1 0 rg Q 0 0 10 10 re f",
		"0 0 m 10 0 l n S",
	}
}

func checkEmitLine(t *testing.T) {
	t.Helper()
	const spaced = "0  0 m  10 0 l S"
	got := mustEmit(t, spaced)
	if bytes.Equal(got, []byte(spaced)) {
		t.Fatal("emit copied the content bytes")
	}
	matchEmit(t, lineSrc, got)
}

func checkEmitUndefined(t *testing.T) {
	t.Helper()
	_, err := Emit(t.Context(), []byte("Tj"))
	var job *pdf.Error
	if !errors.As(err, &job) || job.Error() != "undefined" {
		t.Fatalf("error = %v, want undefined", err)
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
		if !ok || text != "pdfout: nil context" {
			t.Fatalf("panic = %v, want pdfout: nil context", got)
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

func matchEmit(t *testing.T, src string, emitted []byte) {
	t.Helper()
	want := shownImage(t, []byte(src))
	got := shownImage(t, emitted)
	sameSize := want.Width == got.Width && want.Height == got.Height
	if !sameSize || !bytes.Equal(want.Pixels, got.Pixels) {
		t.Fatalf("pixels differ for %q", src)
	}
}

func shownImage(t *testing.T, content []byte) graphics.Image {
	t.Helper()
	pixmap := graphics.NewPixmap(emitSide, emitSide)
	if err := pdf.Paint(t.Context(), content, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	if len(pixmap.Pages()) != 0 {
		t.Fatal("ShowPage ran before the snapshot")
	}
	pixmap.ShowPage()
	pages := pixmap.Pages()
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	return pages[0]
}
