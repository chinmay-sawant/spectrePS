package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/truetypesynth"
)

// TestGenSubsetFixture writes sampledata/fixtures/subset-text.pdf when
// UPDATE_FIXTURES=1. The file is the one-page /FontFile2 input the CLI subset
// proof reads, and internal/cli reads it from disk so it keeps its import
// boundary.
func TestGenSubsetFixture(t *testing.T) {
	if os.Getenv("UPDATE_FIXTURES") != "1" {
		t.Skip("set UPDATE_FIXTURES=1 to rewrite the fixture")
	}
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 72 72] /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(streamBody("", []byte("BT /F1 24 Tf 12 24 Td (AB) Tj ET")))
	doc.object("<< /Type /Font /Subtype /TrueType /BaseFont /Synth " +
		"/FirstChar 65 /Widths [600 400] /FontDescriptor 6 0 R >>")
	doc.object("<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>")
	doc.object(streamBody("/Filter /FlateDecode", flateRaw(t, truetypesynth.Program())))
	path := filepath.Join("..", "..", "sampledata", "fixtures", "subset-text.pdf")
	if err := os.WriteFile(path, doc.classic(""), 0o600); err != nil {
		t.Fatal(err)
	}
}
