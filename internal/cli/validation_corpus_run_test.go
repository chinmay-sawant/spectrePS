package cli

// The corpus-run tests read sampledata/validation/manifest.tsv through
// internal/validation, resolve every row, and run the command class the row's
// folder and expect column select. The classes follow the rows named in
// plans/v0.0.4/5-validation.md, 3.8, 4.6, 5.5, 6.5, 8.7, 9.6, and 10.6:
//
//   - paint: `raster -o out-%d.ppm`, or `text` for a text/ row, or the gs argv
//     mode for a gs-argv/ PDF. A `refuse:<error>` row runs the same paint
//     command, must exit non-zero, must name the error on stderr, and must
//     write no output. A refusal that exits 0 fails the test.
//   - struct: `info` for a PDF, `rewrite -level 2 -o out.pdf` for a rewrite/
//     row, and `text` for a text/ row.
//
// Every committed row is asserted; the external tier skips cleanly when absent.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/validation"
)

const (
	// corpusRasterPattern is the multi-page raster output form.
	corpusRasterPattern = "out-%d.ppm"
	// corpusRewriteName is the rewritten PDF name under the test work dir.
	corpusRewriteName = "out.pdf"
	// corpusTextGoldenDir holds the expected extractions, relative to the
	// corpus root. manifest.checkUnlisted skips this folder.
	corpusTextGoldenDir = "text/expected"
	// corpusUpdateEnv writes the expected text instead of comparing it.
	corpusUpdateEnv = "UPDATE_FIXTURES"
)

// corpusRows loads and checks the manifest once per test run and returns the
// committed rows under the named folders. Every returned row is sorted as the
// manifest is.
func corpusRows(t *testing.T, dirs ...string) []validation.Row {
	t.Helper()
	rows := corpusManifest(t)
	selected := make([]validation.Row, 0, len(rows))
	for _, row := range rows {
		if row.External() || !corpusUnder(row.Path, dirs) {
			continue
		}
		selected = append(selected, row)
	}
	if len(selected) == 0 {
		t.Fatalf("no manifest rows under %v", dirs)
	}
	return selected
}

// corpusExternalRows returns the named external rows. corpusRowPath skips a
// row whose fetched file is absent, because the external tier is optional.
func corpusExternalRows(t *testing.T, paths ...string) []validation.Row {
	t.Helper()
	rows := corpusManifest(t)
	byPath := make(map[string]validation.Row, len(rows))
	for _, row := range rows {
		byPath[row.Path] = row
	}
	selected := make([]validation.Row, 0, len(paths))
	for _, path := range paths {
		row, ok := byPath[path]
		if !ok {
			t.Fatalf("external row %s is not in the manifest", path)
		}
		selected = append(selected, row)
	}
	return selected
}

// corpusManifest loads the manifest and checks every digest, byte count, and
// unlisted file once for the whole package run.
func corpusManifest(t *testing.T) []validation.Row {
	t.Helper()
	corpusOnce.Do(func() {
		corpusAll, corpusLoadErr = validation.Load()
		if corpusLoadErr == nil {
			corpusLoadErr = validation.CheckFiles(corpusAll, validation.CorpusDir())
		}
	})
	if corpusLoadErr != nil {
		t.Fatalf("validation corpus: %v", corpusLoadErr)
	}
	return corpusAll
}

var (
	corpusOnce    sync.Once
	corpusAll     []validation.Row
	corpusLoadErr error
)

// corpusUnder reports whether path sits in one of the named folders.
func corpusUnder(path string, dirs []string) bool {
	for _, dir := range dirs {
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}

// corpusSubtest runs one row in its own subtest.
func corpusSubtest(t *testing.T, row validation.Row) {
	t.Helper()
	t.Run(row.Path, func(t *testing.T) {
		corpusRunRow(t, row)
	})
}

// corpusRunRow dispatches one manifest row to its command class.
func corpusRunRow(t *testing.T, row validation.Row) {
	t.Helper()
	path := corpusRowPath(t, row)
	work := t.TempDir()
	switch {
	case row.Refused():
		corpusRunRefusal(t, row, path, work)
	case strings.HasPrefix(row.Path, "text/"):
		corpusRunText(t, row, path)
	case strings.HasPrefix(row.Path, "rewrite/"):
		corpusRunRewrite(t, row, path, work)
	case row.Expect == validation.ExpectPaint:
		corpusRunPaint(t, row, path, work)
	default:
		corpusRunInfo(t, row, path)
	}
}

// corpusRowPath resolves one row and skips an absent external file.
func corpusRowPath(t *testing.T, row validation.Row) string {
	t.Helper()
	path := filepath.Join(validation.CorpusDir(), filepath.FromSlash(row.Path))
	if _, err := os.Stat(path); err != nil {
		if row.External() {
			t.Skipf("external corpus file absent: %v", err)
		}
		t.Fatalf("corpus file %s: %v", row.Path, err)
	}
	return path
}

// corpusRunRefusal runs the paint command of a refusal row and requires a
// non-zero exit, the recorded error on stderr, and no output file.
func corpusRunRefusal(t *testing.T, row validation.Row, path, work string) {
	t.Helper()
	args := corpusPaintArgs(row, path, work)
	code, _, stderr := callRun(t, args...)
	if code == exitOK {
		t.Fatalf("%s: %q exited 0, want the refusal %q", row.Path, args, row.RefuseError())
	}
	if want := row.RefuseError(); !strings.Contains(stderr, want) {
		t.Fatalf("%s: stderr %q does not contain %q", row.Path, stderr, want)
	}
	matches, err := filepath.Glob(filepath.Join(work, "out-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("%s refused but wrote %v", row.Path, matches)
	}
}

// corpusRunPaint runs a paint row and checks one P6 file per input page, plus
// no extra page.
func corpusRunPaint(t *testing.T, row validation.Row, path, work string) {
	t.Helper()
	args := corpusPaintArgs(row, path, work)
	code, stdout, stderr := callRun(t, args...)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("%s: %q code=%d stdout=%q stderr=%q, want exit 0",
			row.Path, args, code, stdout, stderr)
	}
	if strings.HasPrefix(row.Path, "text/") {
		// A paint-class text row with no extraction error is the claim.
		return
	}
	pages := corpusPaintPages(t, row.Path, path)
	for page := 1; page <= pages; page++ {
		corpusCheckPPM(t, row.Path, filepath.Join(work, fmt.Sprintf("out-%d.ppm", page)))
	}
	extra := filepath.Join(work, fmt.Sprintf("out-%d.ppm", pages+1))
	if _, err := os.Stat(extra); err == nil {
		t.Fatalf("%s wrote %s past the %d-page input", row.Path, extra, pages)
	}
}

// corpusRunInfo runs the struct verdict for a PDF and requires a live page
// count.
func corpusRunInfo(t *testing.T, row validation.Row, path string) {
	t.Helper()
	if pages := corpusInfoPages(t, row.Path, path); pages < 1 {
		t.Fatalf("%s: info reports %d pages", row.Path, pages)
	}
}

// corpusPaintPages returns the page count of a paint row: info for a PDF and
// bbox for a PostScript program.
func corpusPaintPages(t *testing.T, rowPath, path string) int {
	t.Helper()
	if strings.HasSuffix(path, ".pdf") {
		return corpusInfoPages(t, rowPath, path)
	}
	code, stdout, stderr := callRun(t, "bbox", path)
	if code != exitOK || stderr != "" {
		t.Fatalf("%s: bbox code=%d stderr=%q, want exit 0", rowPath, code, stderr)
	}
	pages := strings.Count(stdout, "%%BoundingBox: ")
	if pages < 1 {
		t.Fatalf("%s: bbox printed no page box: %q", rowPath, stdout)
	}
	return pages
}

// corpusRunRewrite rewrites a rewrite/ row at level 2, reopens the output with
// the same page count as the source, and proves two runs return equal bytes.
func corpusRunRewrite(t *testing.T, row validation.Row, path, work string) {
	t.Helper()
	sourcePages := corpusInfoPages(t, row.Path, path)
	first := filepath.Join(work, corpusRewriteName)
	corpusRewriteOnce(t, row, path, first)
	if pages := corpusInfoPages(t, row.Path, first); pages != sourcePages {
		t.Fatalf("%s: level 2 output has %d pages, want the source's %d", row.Path, pages, sourcePages)
	}
	second := filepath.Join(work, "again.pdf")
	corpusRewriteOnce(t, row, path, second)
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("%s: two level 2 rewrites differ", row.Path)
	}
}

// corpusRewriteOnce runs one level 2 rewrite and requires a clean exit.
func corpusRewriteOnce(t *testing.T, row validation.Row, path, out string) {
	t.Helper()
	code, stdout, stderr := callRun(t, "rewrite", "-level", "2", "-o", out, path)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("%s: rewrite -level 2 code=%d stdout=%q stderr=%q, want exit 0",
			row.Path, code, stdout, stderr)
	}
}

// corpusRunText runs the text verdict and compares the extraction with the
// checked-in golden, or writes it when UPDATE_FIXTURES=1 is set.
func corpusRunText(t *testing.T, row validation.Row, path string) {
	t.Helper()
	code, stdout, stderr := callRun(t, "text", path)
	if code != exitOK || stderr != "" {
		t.Fatalf("%s: text code=%d stderr=%q, want exit 0", row.Path, code, stderr)
	}
	want := corpusExpectedText(t, row, stdout)
	if stdout != want {
		t.Fatalf("%s: extraction differs from %s\n got: %q\nwant: %q",
			row.Path, corpusGoldenPath(row.Path), stdout, want)
	}
}

// corpusExpectedText returns the golden extraction. With UPDATE_FIXTURES=1 it
// writes the measured text first, per documentation/folder-structure.md.
func corpusExpectedText(t *testing.T, row validation.Row, got string) string {
	t.Helper()
	golden := corpusGoldenPath(row.Path)
	if os.Getenv(corpusUpdateEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", golden)
		return got
	}
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%s: read %s: %v (run with %s=1 to write it)", row.Path, golden, err, corpusUpdateEnv)
	}
	return string(raw)
}

// corpusGoldenPath maps a text row onto its golden file name.
func corpusGoldenPath(rowPath string) string {
	base := filepath.Base(rowPath)
	name := strings.TrimSuffix(base, filepath.Ext(base)) + ".txt"
	return filepath.Join(validation.CorpusDir(), filepath.FromSlash(corpusTextGoldenDir), name)
}

// corpusPaintArgs builds the paint command for one row.
func corpusPaintArgs(row validation.Row, path, work string) []string {
	switch {
	case strings.HasPrefix(row.Path, "text/"):
		return []string{"text", path}
	case strings.HasPrefix(row.Path, "gs-argv/") && strings.HasSuffix(row.Path, ".pdf"):
		out := filepath.Join(work, corpusRasterPattern)
		return []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, path}
	default:
		out := filepath.Join(work, corpusRasterPattern)
		return []string{"raster", "-o", out, path}
	}
}

// corpusInfoPages runs info and returns the reported page count.
func corpusInfoPages(t *testing.T, rowPath, path string) int {
	t.Helper()
	code, stdout, stderr := callRun(t, "info", path)
	if code != exitOK || stderr != "" {
		t.Fatalf("%s: info %s code=%d stderr=%q, want exit 0", rowPath, path, code, stderr)
	}
	return corpusPagesFromInfo(t, rowPath, stdout)
}

// corpusPagesFromInfo reads the Pages line out of info output.
func corpusPagesFromInfo(t *testing.T, rowPath, stdout string) int {
	t.Helper()
	const prefix = "Pages: "
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		pages, err := strconv.Atoi(strings.TrimPrefix(line, prefix))
		if err != nil {
			t.Fatalf("%s: info Pages line %q: %v", rowPath, line, err)
		}
		return pages
	}
	t.Fatalf("%s: info printed no Pages line: %q", rowPath, stdout)
	return 0
}

// corpusCheckPPM requires one complete P6 body at the recorded geometry.
func corpusCheckPPM(t *testing.T, rowPath, path string) {
	t.Helper()
	width, height := validationPPMSize(t, path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header := len(fmt.Sprintf("P6\n%d %d\n255\n", width, height))
	if got, want := len(raw), header+width*height*rgbChannels; got < want {
		t.Fatalf("%s: %s body is %d bytes, want %d", rowPath, path, got, want)
	}
}

// TestValidationCorpusPostScript runs postscript/ and refs/.
func TestValidationCorpusPostScript(t *testing.T) {
	for _, row := range corpusRows(t, "postscript", "refs") {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusPDF runs structural/ and paths/, plus the external PDF
// 2.0 container when the fetched tier is present.
func TestValidationCorpusPDF(t *testing.T) {
	rows := corpusRows(t, "structural", "paths")
	rows = append(rows, corpusExternalRows(t, "external/pdf20examples/simple-pdf-2.0.pdf")...)
	for _, row := range rows {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusImages runs images/, plus the external CCITT file when
// the fetched tier is present.
func TestValidationCorpusImages(t *testing.T) {
	rows := corpusRows(t, "images")
	rows = append(rows, corpusExternalRows(t, "external/fax-decode-parms.pdf")...)
	for _, row := range rows {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusText runs text/ and compares each extraction with its
// golden file.
func TestValidationCorpusText(t *testing.T) {
	for _, row := range corpusRows(t, "text") {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusRewrite runs rewrite/ at level 2 and reopens the output.
func TestValidationCorpusRewrite(t *testing.T) {
	for _, row := range corpusRows(t, "rewrite") {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusPDFA runs pdfa/ and tagged/.
func TestValidationCorpusPDFA(t *testing.T) {
	for _, row := range corpusRows(t, "pdfa", "tagged") {
		corpusSubtest(t, row)
	}
}

// TestValidationCorpusGS runs gs-argv/ and proves the gs pdfwrite output
// reopens with the source page count.
func TestValidationCorpusGS(t *testing.T) {
	rows := corpusRows(t, "gs-argv")
	for _, row := range rows {
		corpusSubtest(t, row)
	}
	for _, row := range rows {
		if !strings.HasSuffix(row.Path, ".pdf") {
			continue
		}
		t.Run(row.Path+" pdfwrite", func(t *testing.T) {
			corpusRunGSRewrite(t, row)
		})
	}
}

// corpusRunGSRewrite runs gs -sDEVICE=pdfwrite and reopens the output.
func corpusRunGSRewrite(t *testing.T, row validation.Row) {
	t.Helper()
	path := corpusRowPath(t, row)
	out := filepath.Join(t.TempDir(), corpusRewriteName)
	code, stdout, stderr := callRun(t, "gs", "-sDEVICE=pdfwrite", "-sOutputFile="+out, path)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("%s: gs -sDEVICE=pdfwrite code=%d stdout=%q stderr=%q, want exit 0",
			row.Path, code, stdout, stderr)
	}
	sourcePages := corpusInfoPages(t, row.Path, path)
	writtenPages := corpusInfoPages(t, row.Path, out)
	if writtenPages != sourcePages {
		t.Fatalf("%s: gs pdfwrite output has %d pages, want the source's %d",
			row.Path, writtenPages, sourcePages)
	}
}
