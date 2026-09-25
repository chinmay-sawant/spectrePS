package pdfa

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
	"github.com/chinmay-sawant/spectrePS/internal/truetypesynth"
)

// TestEmbedSubsetPDFA proves the PDF/A preflight still refuses a font with no
// embedded program whether or not the subset option ran, and that a font with
// /FontFile2 still passes after a subsetting rewrite.
func TestEmbedSubsetPDFA(t *testing.T) {
	t.Parallel()
	checkPDFASubsetRefusal(t)
	checkPDFASubsetProgram(t)
}

// checkPDFASubsetRefusal runs the subsetting rewrite on a font with no
// program and preflights both outputs.
func checkPDFASubsetRefusal(t *testing.T) {
	t.Helper()
	src := buildClassicPDF(t, []string{
		plainCatalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>",
		plainTextStream(),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	})
	for _, subset := range []bool{false, true} {
		out := rewriteSubset(t, src, subset)
		wantRule(t, Preflight(t.Context(), out, Mode4), ruleFontNotEmbedded)
	}
}

// checkPDFASubsetProgram runs the subsetting rewrite on a /FontFile2 font and
// proves the output still carries a program and passes the preflight.
func checkPDFASubsetProgram(t *testing.T) {
	t.Helper()
	src := embeddedFontPDF(t)
	for _, subset := range []bool{false, true} {
		out := rewriteSubset(t, src, subset)
		wantNoRule(t, Preflight(t.Context(), out, Mode4))
	}
	subset := rewriteSubset(t, src, true)
	if len(subsetPrograms(t, subset)) == 0 {
		t.Fatal("the subsetting rewrite left no font program")
	}
}

// TestEmbedSubsetUA2 proves a tagged input keeps its tree, MCIDs, /Alt,
// /ActualText, and /Lang through a subsetting rewrite, and the synthesized
// /ToUnicode makes TaggedFontOK pass for a simple font with a nonstandard
// encoding.
func TestEmbedSubsetUA2(t *testing.T) {
	t.Parallel()
	src := taggedSubsetPDF(t)
	before, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	after := rewriteSubset(t, src, true)
	checkTagsKept(t, after)
	checkSubsetUnicode(t, before, after)
	checkSubsetTaggedFont(t, after, 8)
}

// checkTagsKept reads the tree and the content after the rewrite.
func checkTagsKept(t *testing.T, file *pdf.File) {
	t.Helper()
	if !file.HasStructTree() {
		t.Fatal("the structure tree was dropped")
	}
	tree, err := file.StructTree()
	if err != nil || tree == nil {
		t.Fatalf("StructTree = %v, %v", tree, err)
	}
	if tree.Lang != "en-US" {
		t.Fatalf("Lang = %q, want en-US", tree.Lang)
	}
	elem := checkTreeShape(t, tree)
	checkMCID(t, tree, elem)
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("/MCID 0")) {
		t.Fatalf("content = %q, want the MCID", content)
	}
}

// checkTreeShape proves the Document root and the one P kid kept /Alt,
// /ActualText, and /Lang.
func checkTreeShape(t *testing.T, tree *pdf.StructTree) *pdf.StructElem {
	t.Helper()
	root := tree.Root()
	if root == nil || root.Type != "Document" || len(root.Kids) != 1 {
		t.Fatalf("root = %+v", root)
	}
	elem := root.Kids[0]
	if elem.Alt != "Alpha" || elem.ActualText != "AB" || elem.Lang != "de-DE" {
		t.Fatalf("element = %+v", elem)
	}
	return elem
}

// checkMCID proves the parent tree still resolves the MCID both ways.
func checkMCID(t *testing.T, tree *pdf.StructTree, elem *pdf.StructElem) {
	t.Helper()
	key, ok := tree.Key(elem)
	if !ok || key.Page != 0 || key.MCID != 0 {
		t.Fatalf("key = %+v, %v", key, ok)
	}
	if got, err := tree.Lookup(key); err != nil || got != elem {
		t.Fatalf("Lookup = %+v, %v", got, err)
	}
}

// checkSubsetUnicode proves the synthesized /ToUnicode overrides the
// nonstandard /Differences encoding, so both codes keep their text.
func checkSubsetUnicode(t *testing.T, before, after *pdf.File) {
	t.Helper()
	wantFont, err := before.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	wantA, okA := wantFont.Unicode('A')
	wantB, okB := wantFont.Unicode('B')
	if !okA || !okB || wantA == wantB {
		t.Fatalf("source Unicode = %q, %q", wantA, wantB)
	}
	gotFont, err := after.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := gotFont.Unicode('A'); !ok || text != wantA {
		t.Fatalf("Unicode(A) = %q, %v, want %q", text, ok, wantA)
	}
	if text, ok := gotFont.Unicode('B'); !ok || text != wantB {
		t.Fatalf("Unicode(B) = %q, %v, want %q", text, ok, wantB)
	}
}

// checkSubsetTaggedFont proves TaggedFontOK sees the synthesized /ToUnicode on
// the rewritten font dictionary.
func checkSubsetTaggedFont(t *testing.T, file *pdf.File, fontNum int) {
	t.Helper()
	dict, ok, err := file.ObjectValue(fontNum)
	if err != nil || !ok || dict.Kind != pdf.KindDict {
		t.Fatalf("font object %d = %v, %v, %v", fontNum, dict.Kind, ok, err)
	}
	if _, found := dict.ValueEntry("ToUnicode"); !found {
		t.Fatal("the font dictionary carries no /ToUnicode")
	}
	if !pdf.TaggedFontOK(dict) {
		t.Fatal("TaggedFontOK = false")
	}
}

// rewriteSubset runs the subsetting pass over src and writes the copy with
// pdfout, the same options the library passes.
func rewriteSubset(t *testing.T, src []byte, subset bool) *pdf.File {
	t.Helper()
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[int][]byte{}
	var appended [][]byte
	if subset {
		overrides, appended, err = file.SubsetFontObjects(t.Context(), file.ObjectCount()+1)
		if err != nil {
			t.Fatal(err)
		}
	}
	copyOpt := pdfout.CopyOptions{
		Overrides:       overrides,
		PackObjects:     false,
		AppendObjects:   appended,
		CatalogOverride: nil,
		PDFA:            false,
	}
	out, err := pdfout.WriteCopy(t.Context(), file, copyOpt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := pdf.Open(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	return after
}

// subsetPrograms returns every /FontFile2 stream body of one file.
func subsetPrograms(t *testing.T, file *pdf.File) [][]byte {
	t.Helper()
	var out [][]byte
	for num := 1; num <= file.ObjectCount(); num++ {
		val, ok, err := file.ObjectValue(num)
		if err != nil || !ok || val.Kind != pdf.KindStream {
			continue
		}
		if _, found := val.ValueEntry("Length1"); found {
			out = append(out, val.Stream)
		}
	}
	return out
}

// taggedSubsetPDF is a tagged one-page PDF whose /F1 is a simple font with a
// nonstandard /Differences encoding and a synthetic /FontFile2 program.
func taggedSubsetPDF(t *testing.T) []byte {
	t.Helper()
	doc := newTaggedDoc()
	doc.object([]byte("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> " +
		"/StructTreeRoot 5 0 R /Lang (en-US) >>"))
	doc.object([]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"))
	doc.object([]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 72 72] " +
		"/Contents 4 0 R /Resources << /Font << /F1 8 0 R >> >> /StructParents 0 >>"))
	doc.object([]byte(taggedTextStream()))
	doc.object([]byte("<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 10 0 R " +
		"/ParentTreeNextKey 1 >>"))
	doc.object([]byte("<< /Type /StructElem /S /Document /P 5 0 R /K [7 0 R] >>"))
	doc.object([]byte("<< /Type /StructElem /S /P /P 6 0 R /Pg 3 0 R /K 0 " +
		"/Alt (Alpha) /ActualText (AB) /Lang (de-DE) >>"))
	doc.object([]byte("<< /Type /Font /Subtype /TrueType /BaseFont /Synth " +
		"/FontDescriptor 9 0 R " +
		"/Encoding << /BaseEncoding /WinAnsiEncoding /Differences [65 /B 66 /A] >> >>"))
	doc.object([]byte("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 11 0 R >>"))
	doc.object([]byte("<< /Nums [0 [7 0 R]] >>"))
	doc.object(programStreamBytes(truetypesynth.Program()))
	return doc.classic()
}

// taggedTextStream is the marked-content stream that shows "AB" through /F1.
func taggedTextStream() string {
	content := "/P <</MCID 0>> BDC BT /F1 12 Tf 5 5 Td (AB) Tj ET EMC"
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

// plainTextStream is an unmarked text stream.
func plainTextStream() string {
	content := "BT /F1 12 Tf 0 0 Td (AB) Tj ET"
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

// embeddedFontPDF is a one-page PDF whose /F1 carries the synthetic
// /FontFile2 program.
func embeddedFontPDF(t *testing.T) []byte {
	t.Helper()
	objects := []string{
		plainCatalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>",
		plainTextStream(),
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FirstChar 65 /Widths [600 400] " +
			"/FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
		string(programStreamBytes(truetypesynth.Program())),
	}
	return buildClassicPDF(t, objects)
}

// programStreamBytes wraps one font program in a raw stream.
func programStreamBytes(program []byte) []byte {
	head := fmt.Sprintf("<< /Length %d >>\nstream\n", len(program))
	out := make([]byte, 0, len(head)+len(program)+12)
	out = append(out, head...)
	out = append(out, program...)
	out = append(out, "\nendstream"...)
	return out
}

// newTaggedDoc starts a builder with the PDF 2.0 tagged header.
type taggedDocBuilder struct {
	buf     bytes.Buffer
	offsets []int
}

func newTaggedDoc() *taggedDocBuilder {
	doc := &taggedDocBuilder{offsets: []int{0}}
	doc.buf.WriteString("%PDF-2.0\n%\xe2\xe3\xcf\xd3\n")
	return doc
}

func (doc *taggedDocBuilder) object(body []byte) {
	num := len(doc.offsets)
	doc.offsets = append(doc.offsets, doc.buf.Len())
	fmt.Fprintf(&doc.buf, "%d 0 obj\n", num)
	doc.buf.Write(body)
	doc.buf.WriteString("\nendobj\n")
}

func (doc *taggedDocBuilder) classic() []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString("0000000000 65535 f \n")
	for num := 1; num < size; num++ {
		fmt.Fprintf(&doc.buf, "%010d 00000 n \n", doc.offsets[num])
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}
