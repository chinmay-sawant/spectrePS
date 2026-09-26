package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/validation"
)

// TestValidationPDFVersionCorpus keeps header parsing separate from the
// effective version reported by info. A catalog /Version can increase the
// header version, including in an incremental update.
func TestValidationPDFVersionCorpus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path      string
		header    string
		effective string
	}{
		{path: "compatibility/versions/asciihexdecode.pdf", header: "1.0", effective: "1.0"},
		{path: "paths/xobject-image.pdf", header: "1.1", effective: "1.1"},
		{path: "compatibility/versions/poppler-67295-0.pdf", header: "1.2", effective: "1.2"},
		{path: "structural/page-with-no-resources.pdf", header: "1.3", effective: "1.3"},
		{path: "paths/path.pdf", header: "1.4", effective: "1.4"},
		{path: "structural/object-stream.pdf", header: "1.5", effective: "1.5"},
		{path: "images/bug_jpx.pdf", header: "1.6", effective: "1.6"},
		{path: "paths/whatisthis.pdf", header: "1.7", effective: "1.7"},
		{path: "pdfa/4f-6-7-3-t01-pass-a.pdf", header: "2.0", effective: "2.0"},
		{path: "compatibility/versions/PDF-versions1.pdf", header: "1.4", effective: "1.6"},
		{path: "compatibility/versions/PDF-versions2.pdf", header: "1.4", effective: "1.6"},
		{path: "compatibility/versions/PDF-versions3.pdf", header: "1.4", effective: "1.6"},
	}
	for _, item := range cases {
		t.Run(item.path, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(validation.CorpusDir(), filepath.FromSlash(item.path))
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(data), "%PDF-"+item.header) {
				t.Fatalf("%s: header is not PDF %s", item.path, item.header)
			}
			code, stdout, stderr := callRun(t, "info", path)
			want := "PDF version: " + item.effective + "\n"
			if code != exitOK || stderr != "" || !strings.HasPrefix(stdout, want) {
				t.Fatalf("%s: code=%d stdout=%q stderr=%q, want prefix %q",
					item.path, code, stdout, stderr, want)
			}
		})
	}
}
