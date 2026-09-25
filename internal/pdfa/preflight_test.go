package pdfa

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	plainCatalog    = "<< /Type /Catalog /Pages 2 0 R >>"
	embeddedCatalog = "<< /Type /Catalog /Pages 2 0 R /Names << /EmbeddedFiles << /Names [] >> >> >>"
	emptyStream     = "<< /Length 0 >>\nstream\n\nendstream"
)

func TestPreflightFonts(t *testing.T) {
	checkFontMissing(t)
	checkFontEmbedded(t)
	checkType3Font(t)
	checkType0Embedded(t)
	checkType0Missing(t)
}

func checkFontMissing(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/Font << /F1 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleFontNotEmbedded)
}

func checkFontEmbedded(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/Font << /F1 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Helvetica /FontFile 7 0 R >>",
		emptyStream)
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkType3Font(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/Font << /F1 5 0 R >>",
		"<< /Type /Font /Subtype /Type3 /FontBBox [0 0 1 1] /CharProcs << >> /Encoding << >> >>")
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkType0Embedded(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/Font << /F1 5 0 R >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /MSGothic /Encoding /Identity-H /DescendantFonts [6 0 R] >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /MSGothic /FontDescriptor 7 0 R >>",
		"<< /Type /FontDescriptor /FontName /MSGothic /FontFile2 8 0 R >>",
		emptyStream)
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkType0Missing(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/Font << /F1 5 0 R >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /MSGothic /Encoding /Identity-H /DescendantFonts [6 0 R] >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /MSGothic >>")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleFontNotEmbedded)
}

func TestPreflightFilters(t *testing.T) {
	checkLZW(t)
	checkUnknownFilter(t)
	checkCryptFilter(t)
	checkFilterChain(t)
	checkFlateFilter(t)
	checkUnfilteredStream(t)
}

func checkLZW(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "", "<< /Filter /LZWDecode /Length 0 >>\nstream\n\nendstream")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleLZWDecode)
}

func checkUnknownFilter(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "", "<< /Filter /FooDecode /Length 0 >>\nstream\n\nendstream")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleFilterNotAllowed)
}

func checkCryptFilter(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "", "<< /Filter /Crypt /Length 0 >>\nstream\n\nendstream")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleFilterNotAllowed)
}

func checkFilterChain(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "",
		"<< /Filter [/ASCII85Decode /FlateDecode] /Length 0 >>\nstream\n\nendstream")
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkFlateFilter(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "", "<< /Filter /FlateDecode /Length 0 >>\nstream\n\nendstream")
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkUnfilteredStream(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "", emptyStream)
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func TestPreflightColors(t *testing.T) {
	checkCMYKImage(t)
	checkRGBImage(t)
	checkAlternates(t)
	checkOPI(t)
	checkBlendMode(t)
	checkNormalBlend(t)
}

const cmykImage = "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
	"/BitsPerComponent 8 /ColorSpace /DeviceCMYK >>"

func checkCMYKImage(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/XObject << /Im0 5 0 R >>", cmykImage)
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleCMYKWithoutProfile)
}

func checkRGBImage(t *testing.T) {
	t.Helper()
	image := "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
		"/BitsPerComponent 8 /ColorSpace /DeviceRGB >>"
	file := preflightFile(t, plainCatalog, "/XObject << /Im0 5 0 R >>", image)
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func checkAlternates(t *testing.T) {
	t.Helper()
	image := "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
		"/BitsPerComponent 8 /ColorSpace /DeviceRGB /Alternates [] >>"
	file := preflightFile(t, plainCatalog, "/XObject << /Im0 5 0 R >>", image)
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleAlternatesNotAllowed)
}

func checkOPI(t *testing.T) {
	t.Helper()
	image := "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 " +
		"/BitsPerComponent 8 /ColorSpace /DeviceRGB /OPI << /Version 1.3 >> >>"
	file := preflightFile(t, plainCatalog, "/XObject << /Im0 5 0 R >>", image)
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleOPINotAllowed)
}

func checkBlendMode(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/ExtGState << /GS0 5 0 R >>",
		"<< /Type /ExtGState /BM /Multiply >>")
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleBlendModeNotAllowed)
}

func checkNormalBlend(t *testing.T) {
	t.Helper()
	file := preflightFile(t, plainCatalog, "/ExtGState << /GS0 5 0 R >>",
		"<< /Type /ExtGState /BM /Normal >>")
	wantNoRule(t, Preflight(t.Context(), file, Mode4))
}

func TestPreflightEmbeddedFiles(t *testing.T) {
	withFiles := preflightFile(t, embeddedCatalog, "")
	wantRule(t, Preflight(t.Context(), withFiles, Mode4), ruleEmbeddedFilesNeed4F)
	wantNoRule(t, Preflight(t.Context(), withFiles, Mode4F))
	plain := preflightFile(t, plainCatalog, "")
	wantNoRule(t, Preflight(t.Context(), plain, Mode4))
	wantRule(t, Preflight(t.Context(), plain, Mode4F), rule4FNeedsEmbeddedFiles)
	wantNoRule(t, Preflight(t.Context(), plain, ModeNone))
	wantRule(t, Preflight(t.Context(), plain, Mode(7)), errType)
}

func TestPreflightNilContext(t *testing.T) {
	file := preflightFile(t, plainCatalog, "")
	defer func() {
		if recovered := recover(); recovered != pdfaNilContextPanic {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_ = Preflight(nil, file, Mode4) //nolint:staticcheck // nil context is the case under test
}

// preflightFile builds a one-page PDF from a fixed skeleton and the extra
// object bodies. The first extra object is object 5.
func preflightFile(t *testing.T, catalog, resources string, extra ...string) *pdf.File {
	t.Helper()
	objects := []string{
		catalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << " + resources + " >> >>",
		emptyStream,
	}
	objects = append(objects, extra...)
	src := buildClassicPDF(t, objects)
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func buildClassicPDF(t *testing.T, objects []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 1, len(objects)+1)
	for i, body := range objects {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xrefAt := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets), xrefAt)
	return buf.Bytes()
}

func wantRule(t *testing.T, err error, rule string) {
	t.Helper()
	var job *pdf.Error
	if !errors.As(err, &job) {
		t.Fatalf("error %v, want *pdf.Error", err)
	}
	if job.Op != opPDFA || job.Name != rule {
		t.Fatalf("error %s in %s, want %s in %s", job.Name, job.Op, rule, opPDFA)
	}
}

func wantNoRule(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("error %v, want nil", err)
	}
}
