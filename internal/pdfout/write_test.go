package pdfout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	lineMarks = "0 0 m 10 0 l S"
	fillMarks = "1 0 0 rg 0 0 10 10 re f"
	pageSide  = 20
)

func TestFlate(t *testing.T) {
	line := []byte(lineMarks)
	fill := []byte(fillMarks)
	pages := []Page{{Content: line}, {Content: fill}}
	compressed := mustWrite(t, pages, true)
	plain := mustWrite(t, pages, false)
	wantFlate(t, compressed, plain)
	wantStable(t, pages, compressed, plain)
	if !bytes.Equal(line, []byte(lineMarks)) || !bytes.Equal(fill, []byte(fillMarks)) {
		t.Fatal("content mutated")
	}
	checkRoundTrip(t, compressed, pages)
	checkRoundTrip(t, plain, pages)
	wantID(t, plain, line, fill)
	wantID(t, compressed, mustFlate(t, line), mustFlate(t, fill))
	checkEmpty(t)
	checkContext(t)
}

func wantFlate(t *testing.T, compressed, plain []byte) {
	t.Helper()
	if !bytes.Contains(compressed, []byte("/Filter /FlateDecode")) {
		t.Fatal("missing FlateDecode")
	}
	if bytes.Contains(plain, []byte("FlateDecode")) {
		t.Fatal("plain file contains FlateDecode")
	}
	rejectDates(t, compressed)
	rejectDates(t, plain)
}

func rejectDates(t *testing.T, src []byte) {
	t.Helper()
	if bytes.Contains(src, []byte("CreationDate")) || bytes.Contains(src, []byte("ModDate")) {
		t.Fatal("file contains a date")
	}
}

func wantStable(t *testing.T, pages []Page, compressed, plain []byte) {
	t.Helper()
	if !bytes.Equal(compressed, mustWrite(t, pages, true)) {
		t.Fatal("compressed bytes differ")
	}
	if !bytes.Equal(plain, mustWrite(t, pages, false)) {
		t.Fatal("plain bytes differ")
	}
}

func checkRoundTrip(t *testing.T, src []byte, pages []Page) {
	t.Helper()
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	if file.PageCount() != len(pages) {
		t.Fatalf("pages %d", file.PageCount())
	}
	for i, page := range pages {
		got, readErr := file.Content(i)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, page.Content) {
			t.Fatalf("page %d content %q", i, got)
		}
		matchPixels(t, page.Content, got)
	}
}

func matchPixels(t *testing.T, original, decoded []byte) {
	t.Helper()
	left := paintShown(t, original)
	right := paintShown(t, decoded)
	if !bytes.Equal(left, right) {
		t.Fatal("pixels differ")
	}
}

func paintShown(t *testing.T, content []byte) []byte {
	t.Helper()
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := pdf.Paint(t.Context(), content, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	pixmap.ShowPage()
	shown := pixmap.Pages()
	if len(shown) != 1 {
		t.Fatalf("shown %d", len(shown))
	}
	return shown[0].Pixels
}

func wantID(t *testing.T, src []byte, parts ...[]byte) {
	t.Helper()
	sum := sha256.New()
	for _, part := range parts {
		_, _ = sum.Write(part)
	}
	digest := hex.EncodeToString(sum.Sum(nil))
	needle := "<" + digest + "><" + digest + ">"
	if !bytes.Contains(src, []byte(needle)) {
		t.Fatalf("id %s missing", digest)
	}
}

func checkEmpty(t *testing.T) {
	t.Helper()
	compressed := mustWrite(t, nil, true)
	plain := mustWrite(t, nil, false)
	if !bytes.Equal(compressed, plain) {
		t.Fatal("empty files differ")
	}
	file, err := pdf.Open(t.Context(), plain)
	if err != nil {
		t.Fatal(err)
	}
	if file.PageCount() != 0 {
		t.Fatalf("pages %d", file.PageCount())
	}
	wantID(t, plain)
	rejectDates(t, plain)
}

func checkContext(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := Write(ctx, nil, true)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("canceled %v %#v", err, got)
	}
	defer func() {
		recovered := recover()
		if recovered != panicNilContext {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_, _ = Write(nil, nil, false) //nolint:staticcheck // nil context is the case under test
}

func mustWrite(t *testing.T, pages []Page, compress bool) []byte {
	t.Helper()
	got, err := Write(t.Context(), pages, compress)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-1.4\n")) {
		t.Fatal("bad header")
	}
	return got
}

func mustFlate(t *testing.T, content []byte) []byte {
	t.Helper()
	got, err := flateBytes(content)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
