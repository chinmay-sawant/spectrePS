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
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "kids cycle", run: validationKidsCycle},
		{name: "damaged shared subtree", run: validationDamagedSharedSubtree},
		{name: "missing pages", run: validationMissingPages},
		{name: "non-page kid", run: validationNonPageKid},
		{name: "direct contents", run: validationDirectContents},
		{name: "array contents", run: validationArrayContents},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.run(t)
		})
	}
}

func validationDamagedSharedSubtree(t *testing.T) {
	t.Helper()
	src := damagedSharedSubtreePDF()
	for _, test := range []struct {
		name      string
		src       []byte
		wantPages int
	}{
		{name: "readable table", src: src, wantPages: 5},
		{name: "rebuilt table", src: replaceStartxref(src, 3), wantPages: 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := mustOpen(t, test.src)
			if got := file.PageCount(); got != test.wantPages {
				t.Fatalf("PageCount = %d, want %d", got, test.wantPages)
			}
			report, err := file.Info()
			if err != nil {
				t.Fatal(err)
			}
			if report.Pages != test.wantPages || len(report.PageSizes) != test.wantPages {
				t.Fatalf("Info pages = %d, sizes = %d, want %d each", report.Pages, len(report.PageSizes), test.wantPages)
			}
			if nums, err := file.PageContentNums(2); err != nil || len(nums) != 0 {
				t.Fatalf("PageContentNums(2) = %v, %v; want no content refs", nums, err)
			}
		})
	}
}

func damagedSharedSubtreePDF() []byte {
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 6 >>")
	doc.object("<< /Type /Pages /Kids [4 0 R 5 0 R] /Count 2 >>")
	doc.object("<< /Type /Page /Parent 2 0 R >>")
	doc.object("<< /Type /Page /Parent 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 7 0 R 8 0 R 9 0 R 999 0 R] /Count 4 >>")
	doc.object("<< /Type /Font /BaseFont /Helvetica >>")
	doc.object(streamBody(
		"/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8",
		[]byte{0},
	))
	doc.object(streamBody("", nil))
	return doc.classic("")
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
