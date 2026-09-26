package pdf

import (
	"fmt"
	"strings"
	"testing"
)

// TestFontUsedCodes proves the reader-side collector returns sorted unique
// codes per page and font resource name, follows Form XObjects, caps one
// resource name, and reports an empty map for a page with no text.
func TestFontUsedCodes(t *testing.T) {
	t.Parallel()
	checkUsedCodesPage(t)
	checkUsedCodesEmpty(t)
	checkUsedCodesCap(t)
}

// checkUsedCodesPage proves the simple font, the Type0 font, TJ, ', and a
// form's own resources.
func checkUsedCodesPage(t *testing.T) {
	t.Helper()
	content := "BT /F1 12 Tf 5 5 Td (BBA) Tj ET\n" +
		"BT /F2 12 Tf 5 5 Td <00010002> Tj ET\n" +
		"q /Fm Do Q\n" +
		"BT /F1 12 Tf 5 5 Td [ (A) -50 <42> ] TJ ET\n" +
		"BT /F1 12 Tf 5 5 Td (C) ' ET"
	file := mustOpen(t, usedCodesDoc(t, content, true))
	used := file.FontUsedCodes()
	if len(used) != 1 {
		t.Fatalf("pages = %d, want 1", len(used))
	}
	wantSimple := []uint32{'A', 'B', 'C'}
	if got := used[0]["F1"]; !sameCodes(got, wantSimple) {
		t.Fatalf("F1 = %v, want %v", got, wantSimple)
	}
	wantType0 := []uint32{1, 2}
	if got := used[0]["F2"]; !sameCodes(got, wantType0) {
		t.Fatalf("F2 = %v, want %v", got, wantType0)
	}
	if len(used[0]) != 2 {
		t.Fatalf("names = %d, want 2", len(used[0]))
	}
}

// checkUsedCodesEmpty proves a page with no text has an empty map.
func checkUsedCodesEmpty(t *testing.T) {
	t.Helper()
	file := mustOpen(t, usedCodesDoc(t, "0 0 m 10 0 l S", false))
	used := file.FontUsedCodes()
	if len(used) != 1 || len(used[0]) != 0 {
		t.Fatalf("used = %v, want one empty map", used)
	}
}

// checkUsedCodesCap proves one resource name reports at most maxUsedCodes
// codes.
func checkUsedCodesCap(t *testing.T) {
	t.Helper()
	var body strings.Builder
	body.WriteString("BT /F1 12 Tf 0 0 Td <")
	for code := 1; code <= maxUsedCodes+16; code++ {
		fmt.Fprintf(&body, "%04X", code)
	}
	body.WriteString("> Tj ET")
	file := mustOpen(t, usedCapDoc(t, body.String()))
	used := file.FontUsedCodes()
	codes := used[0]["F1"]
	if len(codes) != maxUsedCodes {
		t.Fatalf("codes = %d, want the cap %d", len(codes), maxUsedCodes)
	}
	if codes[0] != 1 || codes[len(codes)-1] != maxUsedCodes {
		t.Fatalf("codes span %d through %d", codes[0], codes[len(codes)-1])
	}
}

// usedCapDoc binds /F1 to the Type0 font so the content can carry more than
// 256 distinct codes.
func usedCapDoc(t *testing.T, content string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(streamBody("", []byte(content)))
	doc.object("<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H " +
		"/DescendantFonts [6 0 R] >>")
	doc.object("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth " +
		"/FontDescriptor 7 0 R /CIDToGIDMap /Identity /DW 1000 >>")
	doc.object("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 8 0 R >>")
	doc.object(synthFontStream(t, ""))
	return doc.classic("")
}

// sameCodes compares two sorted code slices.
func sameCodes(got, want []uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range want {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}

// usedCodesDoc builds one page. The simple /F1 is the synthetic TrueType
// font, the Type0 /F2 is the identity descendant that shares its program, and
// /Fm is a form with its own resources that shows A through /F1.
func usedCodesDoc(t *testing.T, content string, withType0 bool) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R"
	if withType0 {
		page += " /F2 8 0 R"
	}
	page += " >> /XObject << /Fm 11 0 R >> >> >>"
	doc.object(page)
	doc.object(streamBody("", []byte(content)))
	doc.object("<< /Type /Font /Subtype /TrueType /BaseFont /Synth " +
		"/FirstChar 65 /Widths [600 400] /FontDescriptor 6 0 R >>")
	doc.object("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>")
	doc.object(synthFontStream(t, ""))
	doc.object("<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H " +
		"/DescendantFonts [9 0 R] >>")
	doc.object("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth " +
		"/FontDescriptor 10 0 R /CIDToGIDMap /Identity /DW 1000 >>")
	doc.object("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>")
	doc.object(streamBody(
		"/Type /XObject /Subtype /Form /BBox [0 0 20 20] "+
			"/Resources << /Font << /F1 5 0 R >> >>",
		[]byte("BT /F1 12 Tf 0 0 Td (A) Tj ET"),
	))
	return doc.classic("")
}
