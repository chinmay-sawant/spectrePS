package pdfout

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

func TestTrailerID(t *testing.T) {
	line := []Page{{Content: []byte("0 0 m 10 0 l S")}}
	compressed := stableWrite(t, line, true)
	plain := stableWrite(t, line, false)
	if bytes.Equal(compressed, plain) {
		t.Fatal("uncompressed bytes matched compressed bytes")
	}
	changed := stableWrite(t, []Page{{Content: []byte("0 0 m 20 0 l S")}}, true)
	if bytes.Equal(compressed, changed) {
		t.Fatal("different content matched")
	}
	wantTrailer(t, compressed)
	wantTrailer(t, plain)
	wantZeroPages(t, nil)
	wantZeroPages(t, []Page{})
	wantCanceled(t, line)
}

func stableWrite(t *testing.T, pages []Page, compress bool) []byte {
	t.Helper()
	first := writeNow(t, pages, compress)
	second := writeNow(t, pages, compress)
	if !bytes.Equal(first, second) {
		t.Fatalf("compress=%v bytes differ", compress)
	}
	return first
}

func writeNow(t *testing.T, pages []Page, compress bool) []byte {
	t.Helper()
	got, err := Write(t.Context(), pages, compress)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func wantTrailer(t *testing.T, src []byte) {
	t.Helper()
	if !bytes.Contains(src, []byte("/ID [")) {
		t.Fatal("missing /ID [")
	}
	for _, word := range []string{"CreationDate", "ModDate", "/Info"} {
		if bytes.Contains(src, []byte(word)) {
			t.Fatalf("contains %s", word)
		}
	}
}

func wantZeroPages(t *testing.T, pages []Page) {
	t.Helper()
	got := stableWrite(t, pages, true)
	file, err := pdf.Open(t.Context(), got)
	if err != nil {
		t.Fatal(err)
	}
	if file.PageCount() != 0 {
		t.Fatalf("PageCount=%d", file.PageCount())
	}
}

func wantCanceled(t *testing.T, pages []Page) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := Write(ctx, pages, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Fatalf("bytes = %#v, want nil", got)
	}
}
