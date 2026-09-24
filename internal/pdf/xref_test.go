package pdf

import (
	"bytes"
	"testing"
)

func TestXRefClassic(t *testing.T) {
	t.Parallel()
	if len([]byte("0000000000 65535 f \n")) != 20 {
		t.Fatal("fixed entry is not 20 bytes")
	}
	fixed := []byte("xref\n0 2\n0000000000 65535 f \n0000000185 00000 n \ntrailer\n")
	wantClassic(t, fixed, 0)
	prefixed := append([]byte("junk\n"), fixed...)
	wantClassic(t, prefixed, len("junk\n"))
	loose := []byte("xref\n0 2\n0000000000 65535 f\n0000000185 00000 n\ntrailer\n")
	wantClassic(t, loose, 0)
	crlf := []byte("xref\r\n0 2\r\n0000000000 65535 f\r\n0000000185 00000 n\r\ntrailer\r\n")
	wantClassic(t, crlf, 0)
}

func TestXRefClassicBad(t *testing.T) {
	t.Parallel()
	_, _, err := ParseClassic([]byte("xref\n0 1\nnot-an-entry\ntrailer\n"), 0)
	wantErr(t, err, opXRef, errSyntax)
}

func TestXRefStream(t *testing.T) {
	t.Parallel()
	body := []byte{1, 0x01, 0x02, 0, 2, 0x00, 0x07, 4}
	entries, err := ParseStreamRows(body, []int{1, 2, 1}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len %d", len(entries))
	}
	wantRow(t, entries[0], 0x0102, 0, 0, false)
	wantRow(t, entries[1], 0, 7, 4, true)
	if !entries[0].InUse || !entries[1].InUse || entries[0].Gen != 0 {
		t.Fatalf("use %+v %+v", entries[0], entries[1])
	}
}

func TestXRefStreamIndex(t *testing.T) {
	t.Parallel()
	body := []byte{1, 0x00, 0x0B, 2, 2, 0x00, 0x04, 1}
	entries, err := ParseStreamRows(body, []int{1, 2, 1}, 50, []int{3, 1, 8, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len %d", len(entries))
	}
	wantRow(t, entries[3], 11, 0, 0, false)
	if entries[3].Gen != 2 {
		t.Fatalf("gen %+v", entries[3])
	}
	wantRow(t, entries[8], 0, 4, 1, true)
}

func TestXRefStreamDefaults(t *testing.T) {
	t.Parallel()
	entries, err := ParseStreamRows([]byte{9}, []int{0, 1, 0}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := entries[0]
	if got.Offset != 9 || !got.InUse || got.Compressed || got.Gen != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestXRefStreamShort(t *testing.T) {
	t.Parallel()
	_, err := ParseStreamRows([]byte{1, 0, 0}, []int{1, 2, 1}, 1, nil)
	wantErr(t, err, opXRef, errSyntax)
	_, err = ParseStreamRows([]byte{1}, []int{1}, 1, nil)
	wantErr(t, err, opXRef, errSyntax)
	_, err = ParseStreamRows(nil, []int{1, -1, 1}, 1, nil)
	wantErr(t, err, opXRef, errSyntax)
}

func wantClassic(t *testing.T, src []byte, offset int) {
	t.Helper()
	entries, next, err := ParseClassic(src, offset)
	if err != nil {
		t.Fatal(err)
	}
	if next < 0 || next > len(src) || !bytes.HasPrefix(src[next:], []byte("trailer")) {
		t.Fatalf("next %d", next)
	}
	if len(entries) != 2 {
		t.Fatalf("len %d", len(entries))
	}
	free := entries[0]
	if free.InUse || free.Compressed || free.Offset != 0 || free.Gen != 65535 {
		t.Fatalf("free %+v", free)
	}
	used := entries[1]
	if !used.InUse || used.Compressed || used.Offset != 185 || used.Gen != 0 {
		t.Fatalf("used %+v", used)
	}
}

func wantRow(t *testing.T, got XEntry, offset, streamNum, streamIdx int, compressed bool) {
	t.Helper()
	if got.Offset != offset || got.StreamNum != streamNum || got.StreamIdx != streamIdx {
		t.Fatalf("row %+v", got)
	}
	if got.Compressed != compressed {
		t.Fatalf("compressed %+v", got)
	}
}
