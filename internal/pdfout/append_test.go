package pdfout

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

func TestCopyAppend(t *testing.T) {
	checkCopyAppend(t)
	checkCopyAppendStable(t)
}

func checkCopyAppend(t *testing.T) {
	t.Helper()
	file := mustOpenPDF(t, copyFixture(t))
	base := file.ObjectCount()
	metadataNum := base + 1
	intentNum := base + 2
	metadata := []byte("<< /Type /Metadata /Subtype /XML /Length 3 >>\nstream\nabc\nendstream")
	intent := []byte(fmt.Sprintf(
		"<< /Type /OutputIntent /S /GTS_PDFA1 /DestOutputProfile %d 0 R >>", metadataNum))
	catalog := []byte(fmt.Sprintf(
		"<< /Type /Catalog /Pages 2 0 R /Metadata %d 0 R /OutputIntents [%d 0 R] >>",
		metadataNum, intentNum))
	opt := CopyOptions{
		AppendObjects:   [][]byte{metadata, intent},
		CatalogOverride: catalog,
	}
	out := mustCopy(t, file, opt)
	reopened := mustOpenPDF(t, out)
	if reopened.PageCount() != 1 {
		t.Fatalf("pages %d", reopened.PageCount())
	}
	checkCopiedValue(t, reopened, copyFontNum, "BaseFont", "Helvetica")
	checkAppendedCatalog(t, reopened, metadataNum, intentNum)
	checkAppendedStream(t, reopened, metadataNum)
	checkAppendedValue(t, reopened, intentNum, "S", "GTS_PDFA1")
	if !bytes.Contains(out, []byte(fmt.Sprintf("/Size %d", base+3))) {
		t.Fatalf("trailer size missing, want %d", base+3)
	}
}

// checkAppendedCatalog checks the override replaced the root body and kept the
// source /Pages entry.
func checkAppendedCatalog(t *testing.T, file *pdf.File, metadataNum, intentNum int) {
	t.Helper()
	catalog, ok, err := file.ObjectValue(file.RootNum())
	if err != nil || !ok {
		t.Fatalf("catalog ok %v err %v", ok, err)
	}
	if pages, found := catalog.ValueEntry("Pages"); !found || pages.Kind != pdf.KindRef {
		t.Fatalf("catalog /Pages = %#v", pages)
	}
	metadata, found := catalog.ValueEntry("Metadata")
	if !found || metadata.Kind != pdf.KindRef || metadata.RefNum != metadataNum {
		t.Fatalf("catalog /Metadata = %#v", metadata)
	}
	intents, found := catalog.ArrayEntry("OutputIntents")
	if !found || len(intents) != 1 || intents[0].Kind != pdf.KindRef || intents[0].RefNum != intentNum {
		t.Fatalf("catalog /OutputIntents = %#v", intents)
	}
}

func checkAppendedStream(t *testing.T, file *pdf.File, num int) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	if val.Kind != pdf.KindStream {
		t.Fatalf("object %d kind %v, want stream", num, val.Kind)
	}
}

func checkAppendedValue(t *testing.T, file *pdf.File, num int, key, want string) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	got, found := val.NameEntry(key)
	if !found || got != want {
		t.Fatalf("object %d %s = %q found %v, want %q", num, key, got, found, want)
	}
}

func checkCopyAppendStable(t *testing.T) {
	t.Helper()
	file := mustOpenPDF(t, copyFixture(t))
	base := file.ObjectCount()
	opt := CopyOptions{
		AppendObjects:   [][]byte{[]byte("<< /Type /Metadata /Subtype /XML >>")},
		CatalogOverride: []byte(fmt.Sprintf("<< /Type /Catalog /Pages 2 0 R /Metadata %d 0 R >>", base+1)),
	}
	first := mustCopy(t, file, opt)
	second := mustCopy(t, file, opt)
	if !bytes.Equal(first, second) {
		t.Fatal("two calls differ")
	}
	rejectDates(t, first)
}
