package pdf

import (
	"bytes"
	"testing"
)

// TestValidationPageTree locks the page-tree error paths and both /Contents
// forms. A /Kids cycle is limitcheck, a catalog with no /Pages is undefined,
// a non-page kid is syntaxerror, and a direct and an array /Contents both
// read through Content.
func TestValidationPageTree(t *testing.T) {
	t.Parallel()
	t.Run("kids cycle", validationKidsCycle)
	t.Run("missing pages", validationMissingPages)
	t.Run("non-page kid", validationNonPageKid)
	t.Run("direct contents", validationDirectContents)
	t.Run("array contents", validationArrayContents)
}

func validationKidsCycle(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [2 0 R] /Count 1 >>")
	_, err := Open(t.Context(), doc.classic(""))
	wantJob(t, err, opPDF, errLimit)
}

func validationMissingPages(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog >>")
	_, err := Open(t.Context(), doc.classic(""))
	wantJob(t, err, opPDF, errUndefined)
}

func validationNonPageKid(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Font /BaseFont /Helvetica >>")
	_, err := Open(t.Context(), doc.classic(""))
	wantJob(t, err, opPDF, errSyntax)
}

func validationDirectContents(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody("", []byte(lineMarks)))
	file := mustOpen(t, doc.classic(""))
	if file.PageCount() != 1 {
		t.Fatalf("PageCount = %d, want 1", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(lineMarks)) {
		t.Fatalf("Content(0) = %q, want %q", got, lineMarks)
	}
}

func validationArrayContents(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents [4 0 R 5 0 R] >>")
	doc.object(streamBody("", []byte("q")))
	doc.object(streamBody("", []byte("Q")))
	file := mustOpen(t, doc.classic(""))
	if file.PageCount() != 1 {
		t.Fatalf("PageCount = %d, want 1", file.PageCount())
	}
	got, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("q\nQ")) {
		t.Fatalf("Content(0) = %q, want %q", got, "q\nQ")
	}
}
