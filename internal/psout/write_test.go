package psout

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestWriteOptionsBox(t *testing.T) {
	pages := []Page{{Content: []byte("S\n")}}
	cases := []struct {
		name string
		opts WriteOptions
		want string
	}{
		{"zero falls back to the default", WriteOptions{}, "%%BoundingBox: 0 0 612 792\n"},
		{"explicit size", WriteOptions{WidthPt: 200, HeightPt: 100}, "%%BoundingBox: 0 0 200 100\n"},
		{"a4 portrait", WriteOptions{WidthPt: 595.28, HeightPt: 841.89}, "%%BoundingBox: 0 0 595 842\n"},
		{"negative width only", WriteOptions{WidthPt: -5, HeightPt: 100}, "%%BoundingBox: 0 0 612 100\n"},
		{"negative height only", WriteOptions{WidthPt: 200, HeightPt: -5}, "%%BoundingBox: 0 0 200 792\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Write(t.Context(), pages, testCase.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(got, []byte(testCase.want)) {
				t.Fatalf("box = %q, want %q in %q", boxOf(got), testCase.want, got)
			}
		})
	}
}

// boxOf returns the %%BoundingBox line so a failure names the real value.
func boxOf(program []byte) string {
	_, after, ok := bytes.Cut(program, []byte("%%BoundingBox: "))
	if !ok {
		return ""
	}
	line, _, _ := bytes.Cut(after, []byte("\n"))
	return "%%BoundingBox: " + string(line)
}

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
	// The zero options fall back to the 612 by 792 reader default.
	if !bytes.Contains(got, []byte("%%BoundingBox: 0 0 612 792\n")) {
		t.Fatalf("missing the default box in %q", got)
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
