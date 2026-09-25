package pdfout

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// taggedHeader20 is a PDF 2.0 header with the binary marker on the next line.
const taggedHeader20 = "%PDF-2.0\n%\xe2\xe3\xcf\xd3\n"

// taggedContent is the marked-content stream of the tagged fixture.
const taggedContent = "/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC"

// taggedDoc builds a classic-xref PDF with a PDF 2.0 header.
type taggedDoc struct {
	buf     bytes.Buffer
	offsets []int
}

func newTaggedDoc() *taggedDoc {
	doc := &taggedDoc{offsets: []int{0}}
	doc.buf.WriteString(taggedHeader20)
	return doc
}

func (doc *taggedDoc) object(body []byte) {
	num := len(doc.offsets)
	doc.offsets = append(doc.offsets, doc.buf.Len())
	fmt.Fprintf(&doc.buf, "%d 0 obj\n", num)
	doc.buf.Write(body)
	doc.buf.WriteString("\nendobj\n")
}

func (doc *taggedDoc) classic() []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString(xrefRow(0, freeGen, 'f'))
	for num := 1; num < size; num++ {
		doc.buf.WriteString(xrefRow(doc.offsets[num], 0, 'n'))
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", size)
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

// taggedFixture is one page with a Document tree, a Figure with /Alt, and one
// MCID in a marked-content stream.
func taggedFixture(t *testing.T) []byte {
	t.Helper()
	doc := newTaggedDoc()
	doc.object([]byte("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> " +
		"/StructTreeRoot 5 0 R /Lang (en-US) >>"))
	doc.object([]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"))
	doc.object([]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
		"/Resources << >> /StructParents 0 >>"))
	doc.object(streamBody([]byte(taggedContent), false))
	doc.object([]byte("<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 8 0 R " +
		"/ParentTreeNextKey 1 >>"))
	doc.object([]byte("<< /Type /StructElem /S /Document /P 5 0 R /K [7 0 R] >>"))
	doc.object([]byte("<< /Type /StructElem /S /Figure /Alt (A figure) /P 6 0 R " +
		"/Pg 3 0 R /K 0 >>"))
	doc.object([]byte("<< /Nums [0 [7 0 R]] >>"))
	return doc.classic()
}

// tagShape is the structure shape one round trip has to keep.
type tagShape struct {
	Root string
	Kid  string
	Role string
	Alt  string
	Page int
	MCID int
}

// TestLevelsPreserveTags proves levels 1 through 5 copy the structure tree,
// the MCIDs, and /Alt.
func TestLevelsPreserveTags(t *testing.T) {
	t.Parallel()
	file := mustOpenPDF(t, taggedFixture(t))
	want := tagShapeOf(t, file)
	for level := MinCompressionLevel; level <= MaxCompressionLevel; level++ {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
		if got := tagShapeOf(t, reopened); got != want {
			t.Fatalf("level %d: shape %+v, want %+v", level, got, want)
		}
		content, err := reopened.Content(0)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if !bytes.Contains(content, []byte("/MCID 0")) {
			t.Fatalf("level %d: content %q", level, content)
		}
	}
}

// TestTaggedHeader proves a tagged PDF 2.0 source keeps its header, and an
// untagged source keeps the classic 1.4 header.
func TestTaggedHeader(t *testing.T) {
	t.Parallel()
	tagged := mustOpenPDF(t, taggedFixture(t))
	out := mustCopy(t, tagged, CopyOptions{})
	if !bytes.HasPrefix(out, []byte(taggedHeader20)) {
		t.Fatalf("tagged header = %q", headerText(out))
	}
	if _, err := mustOpenPDF(t, out).StructTree(); err != nil {
		t.Fatal(err)
	}
	plain := mustOpenPDF(t, copyFixture(t))
	if !bytes.HasPrefix(mustCopy(t, plain, CopyOptions{}), []byte(headerLine)) {
		t.Fatal("untagged source lost the 1.4 header")
	}
}

// headerText returns the first line of a file for a failure message.
func headerText(src []byte) string {
	if end := bytes.IndexByte(src, '\n'); end >= 0 {
		return string(src[:end])
	}
	return string(src)
}

// tagShapeOf reads the tree shape and one MCID lookup both ways.
func tagShapeOf(t *testing.T, file *pdf.File) tagShape {
	t.Helper()
	tree, err := file.StructTree()
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil {
		t.Fatal("StructTree() = nil")
	}
	root := tree.Root()
	if root == nil || len(root.Kids) != 1 {
		t.Fatalf("root = %+v", root)
	}
	kid := root.Kids[0]
	key, ok := tree.Key(kid)
	if !ok {
		t.Fatal("Key() = false")
	}
	elem, err := tree.Lookup(key)
	if err != nil {
		t.Fatal(err)
	}
	if elem != kid {
		t.Fatalf("Lookup() = %+v, want %+v", elem, kid)
	}
	return tagShape{
		Root: root.Type,
		Kid:  kid.Type,
		Role: kid.Role,
		Alt:  kid.Alt,
		Page: key.Page,
		MCID: key.MCID,
	}
}
