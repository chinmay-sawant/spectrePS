package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestTagCommand proves rewrite -tags builds a tagged file, keeps the tree
// and the claim, and refuses a tagged input by name.
func TestTagCommand(t *testing.T) {
	src := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	t.Run("positive", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.pdf")
		want(t, []string{
			"rewrite", "-tags", "-claim", "-tag-title", "Annual report",
			"-tag-lang", "en-US", "-o", out, src,
		}, 0, "", "")
		payload, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(payload, []byte("/StructTreeRoot")) {
			t.Fatal("output lacks /StructTreeRoot")
		}
		if !bytes.Contains(payload, []byte("/Artifact BMC")) {
			t.Fatal("output lacks the artifact wrap")
		}
		if !bytes.Contains(payload, []byte("<pdfuaid:part>2</pdfuaid:part>")) {
			t.Fatal("output lacks pdfuaid:part 2")
		}
	})
	t.Run("tagged input", func(t *testing.T) {
		tagged := writeTemp(t, "tagged.pdf", taggedCLIFixture(t))
		out := filepath.Join(t.TempDir(), "out.pdf")
		want(t, []string{"rewrite", "-tags", "-o", out, tagged}, 1, "",
			"Error: /tagged in RewritePDF\n")
		if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("output file exists: %v", err)
		}
	})
	t.Run("claim needs tags", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.pdf")
		want(t, []string{"rewrite", "-claim", "-o", out, src}, 2, "",
			"spectreps: -claim, -tag-title, and -tag-lang need -tags\n")
	})
}

// TestValidateUA2 proves validate runs the PDF/UA-2 machine checks on a
// tagged input and leaves an untagged input alone.
func TestValidateUA2(t *testing.T) {
	tagged := writeTemp(t, "tagged.pdf", taggedCLIFixture(t))
	want(t, []string{"validate", tagged}, 1, "", "Error: /ua2-document in PDFUA\n")
	plain := writeTemp(t, "plain.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	want(t, []string{"validate", plain}, 0, "", "")
}
