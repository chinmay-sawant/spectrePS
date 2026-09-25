package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// subsetTextFixture is the checked-in one-page PDF with the synthetic
// TrueType /FontFile2 program. The generator is TestGenSubsetFixture in
// internal/pdf; the fixture is pinned so internal/cli keeps the import
// boundary that TestValidationNoProcess locks.
const subsetTextFixture = "subset-text.pdf"

// subsetTextSHA256 is the pinned digest of sampledata/fixtures/subset-text.pdf.
const subsetTextSHA256 = "14b0bc415bbdafcc4177deb34c74ca7e555c4a2e1de114134857f25709220ceb"

// TestRewriteSubsetCLI proves the -subset-fonts flag and the
// RewriteOptions.SubsetFonts option: levels 1 through 5 apply the subset,
// level 0 still refuses text, a bad flag exits 2, and bytes change only when
// the caller opts in.
func TestRewriteSubsetCLI(t *testing.T) {
	src := writeTemp(t, "subset.pdf", subsetTextPDF(t))
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.pdf")
	subset := filepath.Join(dir, "subset.pdf")
	want(t, []string{"rewrite", "-level", "1", "-o", plain, src}, 0, "", "")
	want(t, []string{"rewrite", "-level", "1", "-subset-fonts", "-o", subset, src}, 0, "", "")
	plainBytes := readPayload(t, plain)
	subsetBytes := readPayload(t, subset)
	checkSubsetBytes(t, plainBytes, subsetBytes)
	checkLibraryOption(t, src, plainBytes, subsetBytes)
	checkSubsetLevels(t, src, dir)
	checkSubsetLevelZero(t, src, dir)
	checkSubsetBadFlag(t, src, dir)
	checkSubsetPDFAClaim(t, src, dir)
}

// checkSubsetBytes proves the opted-in output carries a tagged subset program
// and a /ToUnicode map, and the plain output carries neither.
func checkSubsetBytes(t *testing.T, plain, subset []byte) {
	t.Helper()
	if bytes.Equal(plain, subset) {
		t.Fatal("-subset-fonts changed nothing")
	}
	for _, marker := range [][]byte{[]byte("/ToUnicode"), []byte("/Length1"), []byte("+Synth")} {
		if !bytes.Contains(subset, marker) {
			t.Fatalf("subset output has no %s", marker)
		}
		if bytes.Contains(plain, marker) {
			t.Fatalf("plain output carries %s", marker)
		}
	}
	checkTaggedBaseFont(t, subset)
	if got := openPDFBytes(t, subset).PageCount(); got != 1 {
		t.Fatalf("PageCount = %d, want 1", got)
	}
}

// checkTaggedBaseFont proves /BaseFont takes a six uppercase letter tag.
func checkTaggedBaseFont(t *testing.T, subset []byte) {
	t.Helper()
	at := bytes.Index(subset, []byte("+Synth"))
	if at < 6 {
		t.Fatal("no tagged BaseFont")
	}
	tag := subset[at-6 : at]
	for _, letter := range tag {
		if letter < 'A' || letter > 'Z' {
			t.Fatalf("tag %q is not six uppercase letters", tag)
		}
	}
}

// checkLibraryOption proves the RewriteOptions field alone turns subsetting
// on, off by default, and that two opted-in runs return equal bytes.
func checkLibraryOption(t *testing.T, src string, plain, subset []byte) {
	t.Helper()
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	doc, err := in.OpenPDF(t.Context(), readPayload(t, src))
	if err != nil {
		t.Fatal(err)
	}
	gotPlain, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPlain, plain) {
		t.Fatal("the zero SubsetFonts option changed the bytes")
	}
	gotSubset, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: 1, SubsetFonts: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotSubset, subset) {
		t.Fatal("the CLI and the library option disagree")
	}
	again, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: 1, SubsetFonts: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, gotSubset) {
		t.Fatal("two subset runs returned different bytes")
	}
}

// checkSubsetLevels proves every pass-through level accepts the flag.
func checkSubsetLevels(t *testing.T, src, dir string) {
	t.Helper()
	for level := 1; level <= maxRewriteLevel; level++ {
		out := filepath.Join(dir, "level.pdf")
		want(t, []string{"rewrite", "-level", strconv.Itoa(level), "-subset-fonts", "-o", out, src}, 0, "", "")
		if got := openPDFBytes(t, readPayload(t, out)).PageCount(); got != 1 {
			t.Fatalf("level %d: PageCount = %d, want 1", level, got)
		}
	}
}

// checkSubsetLevelZero proves level 0 keeps refusing text with the flag set.
func checkSubsetLevelZero(t *testing.T, src, dir string) {
	t.Helper()
	out := filepath.Join(dir, "zero.pdf")
	code, _, stderr := callRun(t, "rewrite", "-level", "0", "-subset-fonts", "-o", out, src)
	if code != exitJob {
		t.Fatalf("-level 0 -subset-fonts: code = %d, want %d", code, exitJob)
	}
	if !strings.Contains(stderr, "Tj") {
		t.Fatalf("-level 0 -subset-fonts: stderr = %q, want the Tj refusal", stderr)
	}
}

// checkSubsetBadFlag proves a bad -subset-fonts value exits 2.
func checkSubsetBadFlag(t *testing.T, src, dir string) {
	t.Helper()
	out := filepath.Join(dir, "bad.pdf")
	wantCode(t, []string{"rewrite", "-subset-fonts=maybe", "-o", out, src}, exitUsage)
}

// checkSubsetPDFAClaim proves the option combines with a PDF/A claim, whose
// appended objects come before the subset objects.
func checkSubsetPDFAClaim(t *testing.T, src, dir string) {
	t.Helper()
	out := filepath.Join(dir, "pdfa.pdf")
	wantCode(t, []string{"rewrite", "-level", "1", "-pdfa", "4", "-subset-fonts", "-o", out, src}, exitOK)
	payload := readPayload(t, out)
	if !bytes.Contains(payload, []byte("/ToUnicode")) {
		t.Fatal("the PDF/A claim lost the subset /ToUnicode")
	}
	if got := openPDFBytes(t, payload).PageCount(); got != 1 {
		t.Fatalf("PageCount = %d, want 1", got)
	}
}

// subsetTextPDF reads the checked-in fixture and proves its digest, the same
// way the Type 1 command proof does.
func subsetTextPDF(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "sampledata", "fixtures", subsetTextFixture))
	if err != nil {
		t.Fatalf("read %s: %v", subsetTextFixture, err)
	}
	if got := sha256Hex(raw); got != subsetTextSHA256 {
		t.Fatalf("%s sha256 = %s, want %s", subsetTextFixture, got, subsetTextSHA256)
	}
	return raw
}
