package cli

// The three tests in this file are the corpus drivers for the v0.0.4 features
// that documentation/test.md had no section for: font subsetting, PDF/UA-2 tag
// generation, and the Type 1 program. Each reads its rows from
// sampledata/validation/manifest.tsv, so a row's recorded digest and expect
// value are the input contract and TestValidationManifest checks the bytes.
//
// The rows are named rather than discovered. A row earns a place here when it
// carries the structure the case needs: a TrueType /FontFile2 program for the
// subsetter, an untagged page for the tag generator, or a symbolic /FontFile
// Type 1 program for the Type 1 machine. The manifest records the verdict, and
// this file records why a row was chosen.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/validation"
)

// subsetCorpusRow is the committed row that carries no /ToUnicode, so it is the
// only one where a synthesized map is a claim about the subsetter rather than a
// /ToUnicode the pass-through writer copied.
const subsetCorpusRow = "text/subset-text.pdf"

// type1CorpusRow is the committed symbolic /FontFile Type 1 row. The program
// carries a .notdef, an A, a B drawn from a Subrs entry, a seac C, a flex D,
// and a hint-replacement E.
const type1CorpusRow = "text/type1-text.pdf"

// corpusBaseFontPattern matches a /BaseFont name in a PDF body. The name runs
// to the next delimiter, so a subsetter tag and the original name both read as
// one token: GHVCPM+ZCCVRA+font0000000013f5eeab.
var corpusBaseFontPattern = regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+._-]+)`)

// subsetCorpusRows are the committed text rows that carry a TrueType
// /FontFile2 program, which is the only program the subset writer rewrites. A
// /FontFile3 OpenType program and a Type 1 /FontFile program are copied whole,
// so text/Embedded_font.pdf and text/type1-text.pdf are not here.
func subsetCorpusRows() []string {
	return []string{
		"text/complex_ttf_font.pdf",
		"text/mixedfonts.pdf",
		subsetCorpusRow,
	}
}

// tagGenSuccessRows are the committed rows `rewrite -tags` accepts. Each is an
// untagged page with no clip and no image, which is what the generator needs.
func tagGenSuccessRows() []string {
	return []string{
		"gs-argv/gs-argv-input.pdf",
		"paths/path.pdf",
		"structural/object-stream.pdf",
	}
}

// tagGenRefusal is one committed row that `rewrite -tags` must refuse, with the
// exact JobError.Msg it has to report.
type tagGenRefusal struct {
	path string
	msg  string
}

// tagGenRefusalRows are the committed rows `rewrite -tags` refuses. A page with
// a clip refuses through the level 0 path recorder, an image with no /Alt
// source refuses the figure, and a tagged input refuses before any of that.
func tagGenRefusalRows() []tagGenRefusal {
	return []tagGenRefusal{
		{path: "paths/whatisthis.pdf", msg: "undefined in W"},
		{path: "paths/xobject-image.pdf", msg: "alt in Tag"},
		{path: "images/ccitt_EndOfBlock_false.pdf", msg: "alt in Tag"},
		{path: "text/repo-tagged-text.pdf", msg: "tagged in RewritePDF"},
	}
}

// TestValidationCorpusSubset proves rewrite -subset-fonts over the committed
// corpus: the flag changes the bytes, at least one /BaseFont gains a tag derived
// from the subset digest, a source with no /ToUnicode gains a synthesized one,
// the page count holds, two opted-in runs return equal bytes, and level 0 keeps
// refusing text.
func TestValidationCorpusSubset(t *testing.T) {
	manifest := corpusManifest(t)
	for _, path := range subsetCorpusRows() {
		t.Run(path, func(t *testing.T) {
			corpusRunSubsetRow(t, corpusNamedRow(t, manifest, path))
		})
	}
	t.Run("synthesizes ToUnicode", func(t *testing.T) {
		corpusRunSubsetToUnicode(t, corpusNamedRow(t, manifest, subsetCorpusRow))
	})
	t.Run("level 0 ignores the flag", func(t *testing.T) {
		row := corpusNamedRow(t, manifest, subsetCorpusRow)
		src := corpusRowPath(t, row)
		out := filepath.Join(t.TempDir(), "zero.pdf")
		code, _, stderr := callRun(t, "rewrite", "-level", "0", "-subset-fonts", "-o", out, src)
		if code != exitJob {
			t.Fatalf("-level 0 -subset-fonts: code = %d, want %d", code, exitJob)
		}
		if !strings.Contains(stderr, "Tj") {
			t.Fatalf("-level 0 -subset-fonts: stderr = %q, want the Tj refusal", stderr)
		}
		corpusAssertNoOutput(t, out)
	})
}

// corpusRunSubsetRow runs one row plain and opted in, and asserts the contract
// on both writes. The invariant is the retag rather than the /ToUnicode: these
// sources already carry a subsetter tag and, for two of them, a /ToUnicode, so
// only the tag the writer derives from the subset digest changes.
func corpusRunSubsetRow(t *testing.T, row validation.Row) {
	t.Helper()
	src := corpusRowPath(t, row)
	work := t.TempDir()
	plain := filepath.Join(work, "plain.pdf")
	subset := filepath.Join(work, "subset.pdf")
	corpusRewriteLevelOne(t, row, src, plain)
	corpusRewriteLevelOne(t, row, src, subset, "-subset-fonts")
	plainBytes := corpusReadFile(t, plain)
	subsetBytes := corpusReadFile(t, subset)
	if bytes.Equal(plainBytes, subsetBytes) {
		t.Fatal("-subset-fonts changed nothing")
	}
	// The writer prepends a tag to the original /BaseFont, so a subset name
	// reads TAG+original. The original producer tag also survives inside the
	// embedded program's name table, which is why this reads /BaseFont rather
	// than scanning the payload.
	corpusAssertRetagged(t, row.Path, corpusBaseFonts(subsetBytes), corpusBaseFonts(corpusReadFile(t, src)))
	if pages, want := corpusInfoPages(t, row.Path, subset), corpusInfoPages(t, row.Path, src); pages != want {
		t.Fatalf("%s: the subset write has %d pages, want the source's %d", row.Path, pages, want)
	}
	again := filepath.Join(work, "again.pdf")
	corpusRewriteLevelOne(t, row, src, again, "-subset-fonts")
	if !bytes.Equal(corpusReadFile(t, again), subsetBytes) {
		t.Fatal("two subset runs returned different bytes")
	}
}

// corpusRunSubsetToUnicode proves the writer synthesizes a /ToUnicode for a
// source that carries none. Only text/subset-text.pdf qualifies: the other two
// rows already have a /ToUnicode that the pass-through writer copies, so
// asserting it there would prove nothing about the subsetter.
func corpusRunSubsetToUnicode(t *testing.T, row validation.Row) {
	t.Helper()
	src := corpusRowPath(t, row)
	work := t.TempDir()
	plain := filepath.Join(work, "plain.pdf")
	subset := filepath.Join(work, "subset.pdf")
	corpusRewriteLevelOne(t, row, src, plain)
	corpusRewriteLevelOne(t, row, src, subset, "-subset-fonts")
	if bytes.Contains(corpusReadFile(t, plain), []byte("/ToUnicode")) {
		t.Skip("the source already carries a /ToUnicode, so this case proves nothing")
	}
	if !bytes.Contains(corpusReadFile(t, subset), []byte("/ToUnicode")) {
		t.Fatal("the subset write carries no synthesized /ToUnicode")
	}
}

// corpusBaseFonts returns every /BaseFont name in payload, in order and with
// duplicates kept, so a caller can compare a subset write against its source.
func corpusBaseFonts(payload []byte) []string {
	names := []string{}
	for _, match := range corpusBaseFontPattern.FindAllSubmatch(payload, -1) {
		names = append(names, string(match[1]))
	}
	return names
}

// corpusAssertRetagged proves at least one /BaseFont in the subset write is a
// fresh six-letter tag followed by a name the source carried. A name the source
// already had passes through unchanged, so the claim is that at least one name
// was rewritten, not that every name was.
func corpusAssertRetagged(t *testing.T, rowPath string, subsetNames, sourceNames []string) {
	t.Helper()
	known := make(map[string]bool, len(sourceNames))
	for _, name := range sourceNames {
		known[name] = true
	}
	for _, name := range subsetNames {
		tag, rest, found := strings.Cut(name, "+")
		if !found || len(tag) != 6 || !corpusUpperTag([]byte(tag)) {
			continue
		}
		if known[rest] {
			return
		}
	}
	t.Fatalf("%s: no /BaseFont is a fresh tag over a source name; subset %v, source %v",
		rowPath, subsetNames, sourceNames)
}

// corpusUpperTag reports whether tag is six uppercase ASCII letters.
func corpusUpperTag(tag []byte) bool {
	if len(tag) != 6 {
		return false
	}
	for _, letter := range tag {
		if letter < 'A' || letter > 'Z' {
			return false
		}
	}
	return true
}

// TestValidationCorpusTagGeneration proves rewrite -tags over the committed
// corpus: an untagged page gains a structure tree, the claim rides along with a
// title and a language, two runs return equal bytes, and a clip, an image with
// no /Alt, and a tagged input each refuse by name with no output.
func TestValidationCorpusTagGeneration(t *testing.T) {
	manifest := corpusManifest(t)
	for _, path := range tagGenSuccessRows() {
		t.Run(path, func(t *testing.T) {
			corpusRunTagGenSuccess(t, corpusNamedRow(t, manifest, path))
		})
	}
	for _, refusal := range tagGenRefusalRows() {
		t.Run(refusal.path, func(t *testing.T) {
			corpusRunTagGenRefusal(t, corpusNamedRow(t, manifest, refusal.path), refusal.msg)
		})
	}
	t.Run("-tags with -pdfa", func(t *testing.T) {
		row := corpusNamedRow(t, manifest, "paths/path.pdf")
		src := corpusRowPath(t, row)
		out := filepath.Join(t.TempDir(), "out.pdf")
		code, _, stderr := callRun(t, "rewrite", "-tags", "-pdfa", "4", "-o", out, src)
		if code != exitJob {
			t.Fatalf("-tags -pdfa 4: code = %d, want %d", code, exitJob)
		}
		if !strings.Contains(stderr, "unsupported in RewritePDF") {
			t.Fatalf("-tags -pdfa 4: stderr = %q, want the unsupported refusal", stderr)
		}
		corpusAssertNoOutput(t, out)
	})
}

// corpusRunTagGenSuccess proves one row gains a tagged file that reopens, keeps
// its page count, and is byte-stable.
func corpusRunTagGenSuccess(t *testing.T, row validation.Row) {
	t.Helper()
	src := corpusRowPath(t, row)
	work := t.TempDir()
	out := filepath.Join(work, "out.pdf")
	corpusTagGenOnce(t, row.Path, out, src)
	corpusAssertTagGenPayload(t, row.Path, corpusReadFile(t, out), out, src)
	again := filepath.Join(work, "again.pdf")
	corpusTagGenOnce(t, row.Path, again, src)
	if !bytes.Equal(corpusReadFile(t, again), corpusReadFile(t, out)) {
		t.Fatalf("%s: two tagged runs returned different bytes", row.Path)
	}
}

// corpusTagGenOnce runs the claim-carrying tag generation once and requires a
// clean exit.
func corpusTagGenOnce(t *testing.T, rowPath, out, src string) {
	t.Helper()
	args := []string{"rewrite", "-tags", "-claim",
		"-tag-title", "Validation corpus", "-tag-lang", "en-US", "-o", out, src}
	code, stdout, stderr := callRun(t, args...)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("%s: %q code=%d stdout=%q stderr=%q, want exit 0", rowPath, args, code, stdout, stderr)
	}
}

// corpusAssertTagGenPayload proves the written file carries a structure tree and
// the claim, reopens as tagged, and kept the source page count.
func corpusAssertTagGenPayload(t *testing.T, rowPath string, payload []byte, out, src string) {
	t.Helper()
	for _, marker := range [][]byte{
		[]byte("/StructTreeRoot"),
		[]byte("<pdfuaid:part>2</pdfuaid:part>"),
	} {
		if !bytes.Contains(payload, marker) {
			t.Fatalf("%s: the output lacks %s", rowPath, marker)
		}
	}
	if !corpusInfoTagged(t, rowPath, out) {
		t.Fatalf("%s: info reports the tagged output untagged", rowPath)
	}
	if pages, want := corpusInfoPages(t, rowPath, out), corpusInfoPages(t, rowPath, src); pages != want {
		t.Fatalf("%s: the tagged write has %d pages, want the source's %d", rowPath, pages, want)
	}
}

// corpusRunTagGenRefusal proves one row refuses with the named error and writes
// no output file.
func corpusRunTagGenRefusal(t *testing.T, row validation.Row, msg string) {
	t.Helper()
	src := corpusRowPath(t, row)
	out := filepath.Join(t.TempDir(), "out.pdf")
	code, _, stderr := callRun(t, "rewrite", "-tags", "-o", out, src)
	if code != exitJob {
		t.Fatalf("%s: rewrite -tags code = %d, want %d", row.Path, code, exitJob)
	}
	if !strings.Contains(stderr, msg) {
		t.Fatalf("%s: stderr = %q, want the refusal %q", row.Path, stderr, msg)
	}
	corpusAssertNoOutput(t, out)
}

// corpusInfoTagged runs info and reports whether it reads Tagged true.
func corpusInfoTagged(t *testing.T, rowPath, path string) bool {
	t.Helper()
	const prefix = "Tagged: "
	for _, line := range strings.Split(corpusInfoStdout(t, rowPath, path), "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)) == "true"
		}
	}
	t.Fatalf("%s: info printed no Tagged line for %s", rowPath, path)
	return false
}

// TestValidationCorpusType1 proves the committed symbolic /FontFile Type 1 row.
// The program loads and its built-in encoding resolves the codes it carries, so
// the extraction is the built-in AGL names plus the code-point fallback for the
// one code the program does not carry. That same missing glyph is why painting
// refuses with invalidfont rather than drawing a blank page.
func TestValidationCorpusType1(t *testing.T) {
	manifest := corpusManifest(t)
	row := corpusNamedRow(t, manifest, type1CorpusRow)
	src := corpusRowPath(t, row)
	t.Run("embedded program", func(t *testing.T) {
		if !strings.Contains(corpusInfoStdout(t, row.Path, src), "SynthType1 embedded=true") {
			t.Fatalf("%s: info does not report the embedded Type 1 program", row.Path)
		}
	})
	t.Run("extraction", func(t *testing.T) {
		code, stdout, stderr := callRun(t, "text", src)
		if code != exitOK || stderr != "" {
			t.Fatalf("%s: text code=%d stderr=%q, want exit 0", row.Path, code, stderr)
		}
		if want := corpusExpectedText(t, row, stdout); stdout != want {
			t.Fatalf("%s: the extraction differs from %s", row.Path, corpusGoldenPath(row.Path))
		}
	})
	t.Run("paint refuses the absent glyph", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.ppm")
		code, _, stderr := callRun(t, "raster", "-o", out, src)
		if code != exitJob {
			t.Fatalf("%s: raster code = %d, want %d", row.Path, code, exitJob)
		}
		if !strings.Contains(stderr, "invalidfont in Tj") {
			t.Fatalf("%s: stderr = %q, want the invalidfont refusal", row.Path, stderr)
		}
		corpusAssertNoOutput(t, out)
	})
}

// corpusInfoStdout runs info and returns its stdout.
func corpusInfoStdout(t *testing.T, rowPath, path string) string {
	t.Helper()
	code, stdout, stderr := callRun(t, "info", path)
	if code != exitOK || stderr != "" {
		t.Fatalf("%s: info %s code=%d stderr=%q, want exit 0", rowPath, path, code, stderr)
	}
	return stdout
}

// corpusRewriteLevelOne runs one level 1 rewrite and requires a clean exit. The
// extra args follow -level and precede the output path.
func corpusRewriteLevelOne(t *testing.T, row validation.Row, path, out string, extra ...string) {
	t.Helper()
	args := append([]string{"rewrite", "-level", "1"}, extra...)
	args = append(args, "-o", out, path)
	code, stdout, stderr := callRun(t, args...)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("%s: %q code=%d stdout=%q stderr=%q, want exit 0",
			row.Path, args, code, stdout, stderr)
	}
}

// corpusNamedRow returns one committed row by its manifest path.
func corpusNamedRow(t *testing.T, manifest []validation.Row, path string) validation.Row {
	t.Helper()
	for _, row := range manifest {
		if row.Path == path {
			return row
		}
	}
	t.Fatalf("%s is not in the manifest", path)
	return validation.Row{}
}

// corpusReadFile reads one file the run wrote.
func corpusReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// corpusAssertNoOutput fails when path exists, so a refusal wrote nothing.
func corpusAssertNoOutput(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s exists after a refusal: %v", path, err)
	}
}
