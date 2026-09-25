package psout

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestWritePS(t *testing.T) {
	line := mustEmit(t, lineSrc)
	fill := mustEmit(t, fillSrc)
	pages := []Page{{Content: line}, {Content: fill}}
	got, err := Write(t.Context(), pages, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkHeader(t, got, len(pages))
	if bytes.Count(got, []byte(showpageLine)) != len(pages) {
		t.Fatalf("showpage count = %d, want %d", bytes.Count(got, []byte(showpageLine)), len(pages))
	}
	if !bytes.Contains(got, []byte("%%Page: 1 1\n")) || !bytes.Contains(got, []byte("%%Page: 2 2\n")) {
		t.Fatalf("missing page comments in %q", got)
	}
	if !bytes.Contains(got, line) || !bytes.Contains(got, fill) {
		t.Fatal("program lost a page body")
	}
	if !bytes.Equal(line, mustEmit(t, lineSrc)) || !bytes.Equal(fill, mustEmit(t, fillSrc)) {
		t.Fatal("write mutated the page content")
	}
	checkEmptyProgram(t)
}

func checkHeader(t *testing.T, got []byte, pageCount int) {
	t.Helper()
	if !bytes.HasPrefix(got, []byte(headerLine)) {
		t.Fatalf("prefix %q, want %q", got, headerLine)
	}
	if !bytes.Contains(got, []byte(boxLine)) {
		t.Fatalf("missing %q in %q", boxLine, got)
	}
	if !bytes.Contains(got, []byte("%%Pages: "+strconv.Itoa(pageCount)+"\n")) {
		t.Fatalf("missing page count %d in %q", pageCount, got)
	}
	if !bytes.HasSuffix(got, []byte(trailerLine)) {
		t.Fatalf("suffix %q, want %q", got, trailerLine)
	}
	if bytes.Contains(got, []byte("CreationDate")) || bytes.Contains(got, []byte("ModDate")) {
		t.Fatal("program contains a date")
	}
	checkProlog(t, got)
}

// checkProlog checks the definitions that make the emitted short names run.
func checkProlog(t *testing.T, got []byte) {
	t.Helper()
	defs := []string{
		"/m /moveto load def\n",
		"/l /lineto load def\n",
		"/S /stroke load def\n",
		"/f /fill load def\n",
		"/f* /eofill load def\n",
	}
	for _, def := range defs {
		if !bytes.Contains(got, []byte(def)) {
			t.Fatalf("program lacks prolog definition %q", def)
		}
	}
}

func checkEmptyProgram(t *testing.T) {
	t.Helper()
	got, err := Write(t.Context(), nil, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("empty program is nil")
	}
	checkHeader(t, got, 0)
	if bytes.Contains(got, []byte(showpageLine)) {
		t.Fatal("empty program has a showpage")
	}
}

func TestWritePSStable(t *testing.T) {
	pages := []Page{{Content: mustEmit(t, lineSrc)}, {Content: mustEmit(t, fillSrc)}}
	first := mustWrite(t, pages)
	second := mustWrite(t, pages)
	if !bytes.Equal(first, second) {
		t.Fatal("two writes returned different bytes")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := Write(ctx, pages, WriteOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Write() error = %v, want context.Canceled", err)
	}
	if out != nil {
		t.Fatalf("Write() bytes = %#v, want nil", out)
	}
	checkWriteNilContext(t, pages)
}

func checkWriteNilContext(t *testing.T, pages []Page) {
	t.Helper()
	defer func() {
		got := recover()
		text, ok := got.(string)
		if !ok || text != panicNilContext {
			t.Fatalf("panic = %v, want %s", got, panicNilContext)
		}
	}()
	_, _ = Write(nil, pages, WriteOptions{}) //nolint:staticcheck // nil context is the case under test
	t.Fatal("Write returned")
}

func mustWrite(t *testing.T, pages []Page) []byte {
	t.Helper()
	got, err := Write(t.Context(), pages, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("write returned a nil slice")
	}
	return got
}
