package pdf

import (
	"testing"
)

// TestPDFInfo reads the version, page count and inherited sizes, the tagged
// flag, fonts with the embedded flag, and the image count. An encrypted
// trailer is refused at open time with the Encrypt op.
func TestPDFInfo(t *testing.T) {
	t.Parallel()
	checkInfoReport(t)
	checkInfoType0(t)
	checkInfoDefaultBox(t)
	checkInfoBadBox(t)
	checkInfoEncrypt(t)
	checkInfoNil(t)
}

func checkInfoReport(t *testing.T) {
	t.Helper()
	report, err := mustOpen(t, infoPDF(t)).Info()
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != "1.4" {
		t.Fatalf("version %q", report.Version)
	}
	if report.Pages != 2 || len(report.PageSizes) != 2 {
		t.Fatalf("pages %d sizes %v", report.Pages, report.PageSizes)
	}
	if report.PageSizes[0] != (PageSize{Width: 612, Height: 792}) {
		t.Fatalf("page 1 size %v", report.PageSizes[0])
	}
	if report.PageSizes[1] != (PageSize{Width: 100, Height: 50}) {
		t.Fatalf("page 2 size %v", report.PageSizes[1])
	}
	if !report.Tagged {
		t.Fatal("tagged false")
	}
	if report.Images != 1 {
		t.Fatalf("images %d", report.Images)
	}
	wantFonts := []FontInfo{
		{Name: "Alpha", Embedded: false},
		{Name: "Beta", Embedded: true},
	}
	if !equalFonts(report.Fonts, wantFonts) {
		t.Fatalf("fonts %v", report.Fonts)
	}
}

// checkInfoType0 proves a Type0 font is one row and its descendant is not.
func checkInfoType0(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] >>")
	doc.object("<< /Type /Font /Subtype /Type0 /BaseFont /Gamma /DescendantFonts [5 0 R] >>")
	doc.object("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Gamma /FontDescriptor 6 0 R >>")
	doc.object("<< /Type /FontDescriptor /FontName /Gamma /FontFile2 7 0 R >>")
	doc.object(streamBody("", []byte("program")))
	report, err := mustOpen(t, doc.classic("")).Info()
	if err != nil {
		t.Fatal(err)
	}
	want := []FontInfo{{Name: "Gamma", Embedded: true}}
	if !equalFonts(report.Fonts, want) {
		t.Fatalf("fonts %v", report.Fonts)
	}
}

func checkInfoDefaultBox(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R >>")
	report, err := mustOpen(t, doc.classic("")).Info()
	if err != nil {
		t.Fatal(err)
	}
	if report.PageSizes[0] != (PageSize{Width: 612, Height: 792}) {
		t.Fatalf("default size %v", report.PageSizes[0])
	}
	if len(report.Fonts) != 0 || report.Images != 0 || report.Tagged {
		t.Fatalf("empty report %+v", report)
	}
}

func checkInfoBadBox(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10] >>")
	_, err := mustOpen(t, doc.classic("")).Info()
	wantJob(t, err, opInfo, errSyntax)
}

func checkInfoEncrypt(t *testing.T) {
	t.Helper()
	src := textPageTrailer(t, "q", " /Encrypt << /Filter /Standard >>")
	_, err := Open(t.Context(), src)
	wantJob(t, err, opEncrypt, errAccess)
}

func checkInfoNil(t *testing.T) {
	t.Helper()
	var file *File
	_, err := file.Info()
	wantJob(t, err, opInfo, errType)
}

// infoPDF is a two-page file: page 1 inherits the tree box, page 2 overrides
// it. The catalog is marked, object 6 is an image, and /Beta has a program
// while /Alpha does not.
func infoPDF(t *testing.T) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 /MediaBox [0 0 612 792] >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 5 0 R " +
		"/Resources << /XObject << /Im0 6 0 R >> /Font << /F1 7 0 R /F2 8 0 R >> >> >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [10 20 110 70] >>")
	doc.object(streamBody("", []byte("q Q")))
	doc.object(streamBody(
		"/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray "+
			"/BitsPerComponent 8 /Filter /FlateDecode", flateRaw(t, []byte{0x80})))
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Beta /FontDescriptor 9 0 R >>")
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Alpha >>")
	doc.object("<< /Type /FontDescriptor /FontName /Beta /FontFile2 10 0 R >>")
	doc.object(streamBody("", []byte("program")))
	return doc.classic("")
}

func equalFonts(got, want []FontInfo) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
