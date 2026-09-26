package spectreps_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestWritePostScript(t *testing.T) {
	in := newInst(t)
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, "0 0 m 10 0 l S"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("stable", func(t *testing.T) { checkWritePostScriptStable(t, in, doc) })
	t.Run("text", func(t *testing.T) { checkWritePostScriptText(t, in) })
	t.Run("nil document", func(t *testing.T) { checkWritePostScriptNilDoc(t, in) })
	t.Run("canceled", func(t *testing.T) { checkWritePostScriptCanceled(t, in, doc) })
	t.Run("nil context", func(t *testing.T) { checkWritePostScriptNilContext(t, in) })
}

// checkWritePostScriptStable checks the date-free header, the showpage count,
// and that two writes return equal bytes.
func checkWritePostScriptStable(t *testing.T, in *spectreps.Instance, doc *spectreps.Document) []byte {
	t.Helper()
	first, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareFiles(first, second); !compared.Equal {
		t.Fatalf("CompareFiles = %+v", compared)
	}
	if !bytes.HasPrefix(first, []byte("%!PS-Adobe-3.0")) {
		t.Fatalf("program prefix = %q, want %%!PS-Adobe-3.0", first)
	}
	// The box is the document's own /MediaBox, not a fixed letter box. The
	// fixture is 20 by 20 points.
	if !bytes.Contains(first, []byte("%%BoundingBox: 0 0 20 20")) {
		t.Fatalf("program has no 20 by 20 box: %q", first)
	}
	if bytes.Contains(first, []byte("CreationDate")) || bytes.Contains(first, []byte("ModDate")) {
		t.Fatal("program contains a date")
	}
	if got := bytes.Count(first, []byte("showpage")); got != doc.PageCount() {
		t.Fatalf("showpage count = %d, want %d", got, doc.PageCount())
	}
	return first
}

func checkWritePostScriptText(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, "(Hi) Tj"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != "Tj" || job.Error() != "Error: /undefined in Tj" {
		t.Fatalf("WritePostScript() error = %v, want undefined in Tj", err)
	}
	if out != nil {
		t.Fatalf("WritePostScript() bytes = %#v, want nil", out)
	}
}

func checkWritePostScriptNilDoc(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	out, err := in.WritePostScript(t.Context(), nil, spectreps.PostScriptOptions{})
	want := spectreps.JobError{Op: "WritePostScript", Msg: rangecheckMsg, Filename: "", Line: 0, Column: 0}
	var job spectreps.JobError
	if !errors.As(err, &job) || job != want {
		t.Fatalf("WritePostScript() error = %v, want %v", err, want)
	}
	if out != nil {
		t.Fatalf("WritePostScript() bytes = %#v, want nil", out)
	}
}

func checkWritePostScriptCanceled(t *testing.T, in *spectreps.Instance, doc *spectreps.Document) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := in.WritePostScript(ctx, doc, spectreps.PostScriptOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WritePostScript() error = %v, want context.Canceled", err)
	}
	if out != nil {
		t.Fatalf("WritePostScript() bytes = %#v, want nil", out)
	}
}

func checkWritePostScriptNilContext(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	opt := spectreps.PostScriptOptions{}
	requirePanic(t, func() {
		_, err := in.WritePostScript(nil, nil, opt) //nolint:staticcheck // nil context is the case under test
		if err != nil {
			t.Errorf("WritePostScript() error = %v", err)
		}
	})
}

func TestPostScriptRoundTrip(t *testing.T) {
	in := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	content := "q 1 0 0 1 2 1 cm 1 0 0 rg 0 0 8 8 re f Q " +
		"0 1 0 rg 0 0 m 10 0 l S " +
		"0 0 1 rg 0 0 m 0 5 5 5 5 0 c S"
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, content))
	if err != nil {
		t.Fatal(err)
	}
	before, err := in.RasterizePage(t.Context(), doc, 0, opt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pages, err := in.RunPostScript(t.Context(), program, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("RunPostScript() pages = %d, want 1", len(pages))
	}
	if compared := spectreps.CompareRaster(before, pages[0]); !compared.Equal {
		t.Fatalf("CompareRaster = %+v", compared)
	}
}
