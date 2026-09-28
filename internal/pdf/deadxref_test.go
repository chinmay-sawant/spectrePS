package pdf

import "testing"

// deadRowDoc builds a document whose xref carries an in-use row for an object
// the file does not carry. The row for deadNum is written with page 4's offset,
// so resolving it lands on the wrong object number. Nothing in the document
// references deadNum, which is what the real files look like: the producer
// wrote a row for an object it then dropped.
func deadRowDoc(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 5 0 R >>")
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	doc.object(streamBody("", []byte("q")))
	doc.deadRow(6, idPage)
	return doc.classic("")
}

// TestInfoSkipsDeadXrefRows requires the document summary to survive an in-use
// xref row whose object the file does not carry. The row is a dead object
// number, not a font and not an image, so it belongs in neither list and must
// not veto the rest of the summary.
func TestInfoSkipsDeadXrefRows(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, deadRowDoc(t))
	report, err := file.Info()
	if err != nil {
		t.Fatal(err)
	}
	if report.Pages != 1 {
		t.Fatalf("pages %d", report.Pages)
	}
	if len(report.Fonts) != 1 || report.Fonts[0].Name != "Helvetica" {
		t.Fatalf("fonts %v", report.Fonts)
	}
	if report.Images != 0 {
		t.Fatalf("images %d", report.Images)
	}
}

// TestReferencedDeadXrefRowStillFails requires the tolerance above to stay on the
// survey side. A page that references the dead number is a real dependency, and
// walking the page tree must still refuse it.
func TestReferencedDeadXrefRowStillFails(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 6 0 R >>")
	doc.object(streamBody("", []byte("q")))
	doc.deadRow(5, idPage)
	_, err := Open(t.Context(), doc.classic(""))
	wantJob(t, err, opXRef, errSyntax)
}

// TestImageNumsSkipDeadXrefRows requires the image survey to step over a dead
// row the same way the font survey does, and to keep the real image.
func TestImageNumsSkipDeadXrefRows(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [] /Count 0 >>")
	imageDict := "/Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8"
	imageNum := doc.object(streamBody(imageDict, []byte{0}))
	const deadNum = 4
	doc.deadRow(deadNum, idCatalog)
	file := mustOpen(t, doc.classic(""))
	nums, err := file.ImageObjectNums()
	if err != nil {
		t.Fatal(err)
	}
	if len(nums) != 1 || nums[0] != imageNum {
		t.Fatalf("nums %v want [%d]", nums, imageNum)
	}
	if _, _, err := file.ObjectValue(deadNum); err == nil {
		t.Fatal("the dead number still resolves for a caller that asks for it")
	}
}
