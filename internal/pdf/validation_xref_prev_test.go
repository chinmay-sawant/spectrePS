package pdf

import (
	"bytes"
	"fmt"
	"testing"
)

// TestValidationXRefPrev locks trailer /Prev chain walking: the newest section
// wins per object number, an older section fills the gaps, trailer keys inherit
// from older sections except /Prev and /Size, and a cycle, a malformed /Prev,
// a chain past the cap, and a broken older section each fail by name.
func TestValidationXRefPrev(t *testing.T) {
	t.Parallel()
	t.Run("newest wins and older fills", validationPrevMerge)
	t.Run("cycle", validationPrevCycle)
	t.Run("malformed prev", validationPrevMalformed)
	t.Run("chain cap", validationPrevChainCap)
	t.Run("broken older section", validationPrevBrokenOlder)
}

func validationPrevMerge(t *testing.T) {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	newest3 := body.Len()
	body.WriteString("3 0 obj\n<< /Type /Page /Marker /Newest >>\nendobj\n")
	older4 := body.Len()
	body.WriteString("4 0 obj\n<< /Type /Page /Marker /Older >>\nendobj\n")
	older3 := body.Len()
	body.WriteString("3 0 obj\n<< /Type /Page /Marker /OlderCopy >>\nendobj\n")

	olderAt := body.Len()
	body.Write(validationClassicSection(
		[]objPos{{num: 0}, {num: 3, offset: older3}, {num: 4, offset: older4}},
		"/Root 9 0 R /Info 7 0 R /Size 30"))
	newestAt := body.Len()
	body.Write(validationClassicSection(
		[]objPos{{num: 0}, {num: 3, offset: newest3}},
		fmt.Sprintf("/Root 1 0 R /Prev %d", olderAt)))

	entries, trailer, err := readCrossRef(body.Bytes(), newestAt)
	if err != nil {
		t.Fatal(err)
	}
	if got := entries[3].Offset; got != newest3 {
		t.Fatalf("object 3 offset %d, want the newest %d", got, newest3)
	}
	if got, ok := entries[4]; !ok || got.Offset != older4 {
		t.Fatalf("object 4 row %+v, want offset %d from the older section", got, older4)
	}
	if root, ok := trailer.ValueEntry(keyRoot); !ok || root.RefNum != 1 {
		t.Fatalf("trailer /Root %+v, want 1 0 R from the newest section", root)
	}
	if info, ok := trailer.ValueEntry("Info"); !ok || info.RefNum != 7 {
		t.Fatalf("trailer /Info %+v, want 7 0 R inherited from the older section", info)
	}
	if _, ok := trailer.ValueEntry(keySize); ok {
		t.Fatal("merged trailer inherited /Size from the older section")
	}
	if _, ok := trailer.ValueEntry(keyPrev); ok {
		t.Fatal("merged trailer kept /Prev")
	}
}

// validationClassicSection writes one classic xref section with one
// subsection per row, then a trailer dictionary with extra inside it.
func validationClassicSection(rows []objPos, extra string) []byte {
	var buf bytes.Buffer
	buf.WriteString("xref\n")
	for _, row := range rows {
		fmt.Fprintf(&buf, "%d 1\n", row.num)
		if row.num == 0 {
			buf.WriteString(xrefLine(0, freeGen, false))
			continue
		}
		buf.WriteString(xrefLine(row.offset, 0, true))
	}
	fmt.Fprintf(&buf, "trailer\n<< %s >>\n", extra)
	return buf.Bytes()
}

func validationPrevCycle(t *testing.T) {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	at := body.Len()
	body.Write(validationClassicSection(
		[]objPos{{num: 0}}, fmt.Sprintf("/Root 1 0 R /Prev %d", at)))
	_, _, err := readCrossRef(body.Bytes(), at)
	wantJob(t, err, opXRef, errSyntax)
}

func validationPrevMalformed(t *testing.T) {
	t.Helper()
	for name, extra := range map[string]string{
		"string":   "/Root 1 0 R /Prev (older)",
		"name":     "/Root 1 0 R /Prev /Older",
		"negative": "/Root 1 0 R /Prev -1",
		"past eof": "/Root 1 0 R /Prev 999999",
	} {
		t.Run(name, func(t *testing.T) {
			var body bytes.Buffer
			body.WriteString("%PDF-1.4\n")
			at := body.Len()
			body.Write(validationClassicSection([]objPos{{num: 0}}, extra))
			_, _, err := readCrossRef(body.Bytes(), at)
			wantJob(t, err, opXRef, errSyntax)
		})
	}
}

// validationPrevSection writes a fixed-width chained section. The ten-digit
// /Prev keeps every chained section the same length, so offsets are known
// before the chain is written.
func validationPrevSection(prev int) []byte {
	var buf bytes.Buffer
	buf.WriteString("xref\n0 1\n")
	buf.WriteString(xrefLine(0, freeGen, false))
	fmt.Fprintf(&buf, "trailer\n<< /Size 4 /Prev %010d >>\n", prev)
	return buf.Bytes()
}

// validationPrevLastSection writes the oldest section, which ends the chain.
func validationPrevLastSection() []byte {
	var buf bytes.Buffer
	buf.WriteString("xref\n0 1\n")
	buf.WriteString(xrefLine(0, freeGen, false))
	buf.WriteString("trailer\n<< /Size 4 >>\n")
	return buf.Bytes()
}

func validationPrevChainCap(t *testing.T) {
	t.Helper()
	atCap := validationPrevChain(64)
	if _, _, err := readCrossRef(atCap, len("%PDF-1.4\n")); err != nil {
		t.Fatalf("chain of 64 sections error = %v, want a clean walk", err)
	}
	overCap := validationPrevChain(65)
	_, _, err := readCrossRef(overCap, len("%PDF-1.4\n"))
	wantJob(t, err, opXRef, errSyntax)
}

// validationPrevChain builds a chain of count sections, newest first, where
// every section but the last points at the next one.
func validationPrevChain(count int) []byte {
	base := len("%PDF-1.4\n")
	width := len(validationPrevSection(0))
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	for i := range count - 1 {
		body.Write(validationPrevSection(base + (i+1)*width))
	}
	body.Write(validationPrevLastSection())
	fmt.Fprintf(&body, "startxref\n%d\n%%%%EOF\n", base)
	return body.Bytes()
}

func validationPrevBrokenOlder(t *testing.T) {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n")
	body.WriteString("1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	brokenAt := body.Len()
	body.WriteString("6 0 obj\n<< /Type /XRef /W [1 2 1] /Length 3 /Filter /FooDecode >>\nstream\nabc\nendstream\nendobj\n")
	newestAt := body.Len()
	body.Write(validationClassicSection(
		[]objPos{{num: 0}}, fmt.Sprintf("/Root 1 0 R /Prev %d", brokenAt)))
	_, _, err := readCrossRef(body.Bytes(), newestAt)
	wantJob(t, err, "FooDecode", errUndefined)
}
