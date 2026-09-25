package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPSCommand(t *testing.T) {
	src := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	wantCode(t, []string{"ps", src}, exitUsage)
	wantCode(t, []string{"ps", "-o", filepath.Join(t.TempDir(), "out.ps")}, exitUsage)

	out := filepath.Join(t.TempDir(), "out.ps")
	want(t, []string{"ps", "-o", out, src}, exitOK, "", "")
	payload := readPayload(t, out)
	if !bytes.HasPrefix(payload, []byte("%!PS-Adobe-3.0")) {
		t.Fatalf("output prefix = %q, want %%!PS-Adobe-3.0", payload)
	}
	if !bytes.Contains(payload, []byte("showpage")) {
		t.Fatalf("output has no showpage: %q", payload)
	}
	if !bytes.Contains(payload, []byte("0 0 m\n10 0 l\nS\n")) {
		t.Fatalf("output lost the path marks: %q", payload)
	}
	checkPSMode(t, out)

	text := writeTemp(t, "text.pdf", onePagePDF(t, "(Hi) Tj"))
	textOut := filepath.Join(t.TempDir(), "text.ps")
	want(t, []string{"ps", "-o", textOut, text}, exitJob, "", "Error: /undefined in Tj\n")
	if _, err := os.Stat(textOut); err == nil {
		t.Fatal("ps wrote output for a text page")
	}
}

func checkPSMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != rasterFileMode {
		t.Fatalf("mode = %#o, want %#o", perm, rasterFileMode)
	}
}
