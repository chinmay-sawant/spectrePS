package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestPDFImageRefusesTagged proves pdfimage refuses a tagged PDF instead of
// writing an image PDF that dropped the tree.
func TestPDFImageRefusesTagged(t *testing.T) {
	src := writeTemp(t, "tagged.pdf", taggedCLIFixture(t))
	out := filepath.Join(t.TempDir(), "out.pdf")
	want(t, []string{"pdfimage", "-o", out, "-w", "20", "-h", "20", "-r", "72", src},
		1, "", "Error: /tagged in ImagePDF\n")
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file exists: %v", err)
	}
}

// taggedCLIFixture is one path-only page with a structure tree.
func taggedCLIFixture(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> /StructTreeRoot 5 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>"),
		flateStream(t, "0 0 m 10 0 l S"),
		[]byte("<< /Type /StructTreeRoot /K [6 0 R] >>"),
		[]byte("<< /Type /StructElem /S /Document /P 5 0 R >>"),
	}
	return classicXref(t, objects)
}
