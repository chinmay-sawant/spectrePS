package spectreps_test

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// taggedMsg is the JobError message for a tagged document at level 0.
const taggedMsg = "tagged"

// TestRewriteRefusesTagged proves level 0 refuses a tagged document instead of
// stripping the tree, and that levels 1 through 5 keep it.
func TestRewriteRefusesTagged(t *testing.T) {
	in := newInst(t)
	doc, err := in.OpenPDF(t.Context(), taggedPDF(t))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Tagged() {
		t.Fatal("Tagged() = false")
	}
	out, err := in.RewritePDF(t.Context(), doc, spectreps.DefaultRewriteOptions())
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != taggedMsg || job.Op != "RewritePDF" {
		t.Fatalf("RewritePDF() error = %v, want tagged in RewritePDF", err)
	}
	if out != nil {
		t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
	}
	kept, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: 1})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := in.OpenPDF(t.Context(), kept)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Tagged() {
		t.Fatal("level 1 dropped the tags")
	}
}

// taggedPDF is one path-only page with a structure tree and no MCIDs, so
// level 0 would otherwise succeed and strip the tags.
func taggedPDF(t *testing.T) []byte {
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
