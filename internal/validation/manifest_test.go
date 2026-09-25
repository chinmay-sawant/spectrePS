package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidationManifest parses the checked-in manifest, requires the
// provenance fields on every row, rejects an unlisted file, and checks every
// committed digest.
func TestValidationManifest(t *testing.T) {
	rows, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	committed := 0
	for _, row := range rows {
		if row.External() {
			continue
		}
		committed++
		if row.Source == "" || row.License == "" || row.SHA256 == "" {
			t.Fatalf("%s: source, license, or sha256 is empty", row.Path)
		}
	}
	if committed < 40 {
		t.Fatalf("manifest lists %d committed rows, want at least 40", committed)
	}
	if err := CheckFiles(rows, CorpusDir()); err != nil {
		t.Fatalf("CheckFiles: %v", err)
	}
}

// TestValidationManifestParse locks the parser's field rules with a synthetic
// manifest so a broken row fails before the corpus is touched.
func TestValidationManifestParse(t *testing.T) {
	sha := strings.Repeat("a", 64)
	repo := LicenseRepoAuthored
	good := manifestHeader + "\n" +
		manifestLine("paths/a.pdf", sha, "12", repo, repo, "a feature", ExpectPaint) +
		manifestLine("external/b.pdf", strings.Repeat("b", 64), "34", repo, repo, "a feature", ExpectStruct) +
		manifestLine("paths/c.pdf", strings.Repeat("c", 64), "56", repo, repo, "a feature", "refuse:limitcheck")
	rows, err := ParseManifest([]byte(good))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("parsed %d rows, want 3", len(rows))
	}
	if rows[0].Bytes != 12 || rows[1].Refused() || rows[2].RefuseError() != "limitcheck" {
		t.Fatalf("parsed rows: %+v", rows)
	}
	if !rows[1].External() || rows[0].External() {
		t.Fatalf("External() is wrong: %+v", rows)
	}

	bad := map[string]string{
		"empty source":  manifestLine("paths/a.pdf", sha, "12", "", repo, "f", ExpectPaint),
		"empty license": manifestLine("paths/a.pdf", sha, "12", repo, "", "f", ExpectPaint),
		"empty sha":     manifestLine("paths/a.pdf", "", "12", repo, repo, "f", ExpectPaint),
		"short sha":     manifestLine("paths/a.pdf", "abc", "12", repo, repo, "f", ExpectPaint),
		"upper sha":     manifestLine("paths/a.pdf", strings.ToUpper(sha), "12", repo, repo, "f", ExpectPaint),
		"zero bytes":    manifestLine("paths/a.pdf", sha, "0", repo, repo, "f", ExpectPaint),
		"bad expect":    manifestLine("paths/a.pdf", sha, "12", repo, repo, "f", "maybe"),
		"bare refuse":   manifestLine("paths/a.pdf", sha, "12", repo, repo, "f", ExpectRefuse),
		"parent path":   manifestLine("../a.pdf", sha, "12", repo, repo, "f", ExpectPaint),
		"absolute path": manifestLine("/a.pdf", sha, "12", repo, repo, "f", ExpectPaint),
		"six columns":   strings.Join([]string{"paths/a.pdf", sha, "12", repo, repo, "f"}, "\t") + "\n",
		"empty line":    "\n",
	}
	dupe := manifestLine("paths/a.pdf", sha, "12", repo, repo, "f", ExpectPaint)
	bad["duplicate path"] = dupe + dupe
	for name, text := range bad {
		if _, err := ParseManifest([]byte(manifestHeader + "\n" + text)); err == nil {
			t.Errorf("%s: ParseManifest accepted the row", name)
		}
	}
}

// TestValidationManifestRejectsUnlisted proves CheckFiles fails with the name
// of a file that has no row and with the name of a file whose digest moved.
func TestValidationManifestRejectsUnlisted(t *testing.T) {
	dir := t.TempDir()
	body := []byte("hello")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	row := Row{
		Path:    "a.pdf",
		SHA256:  digest,
		Bytes:   int64(len(body)),
		Source:  LicenseRepoAuthored,
		License: LicenseRepoAuthored,
		Feature: "f",
		Expect:  ExpectPaint,
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.pdf", body)
	if err := CheckFiles([]Row{row}, dir); err != nil {
		t.Fatalf("CheckFiles: %v", err)
	}
	write("stray.txt", []byte("unlisted"))
	if err := CheckFiles([]Row{row}, dir); err == nil || !strings.Contains(err.Error(), "stray.txt") {
		t.Fatalf("CheckFiles with a stray file = %v, want it to name stray.txt", err)
	}
	if err := os.Remove(filepath.Join(dir, "stray.txt")); err != nil {
		t.Fatal(err)
	}
	write("a.pdf", []byte("hello!"))
	if err := CheckFiles([]Row{row}, dir); err == nil || !strings.Contains(err.Error(), "a.pdf") {
		t.Fatalf("CheckFiles with a moved digest = %v, want it to name a.pdf", err)
	}
}

// TestValidationLicenses proves the license gate: the admitted set passes, an
// AGPL row fails, a CC BY-SA 4.0 row is external-only, and the checked-in
// corpus contains no excluded source family.
func TestValidationLicenses(t *testing.T) {
	base := Row{
		Path:    "text/a.pdf",
		SHA256:  strings.Repeat("a", 64),
		Bytes:   1,
		Source:  LicenseRepoAuthored,
		Feature: "f",
		Expect:  ExpectPaint,
	}
	testAdmittedLicenses(t, base)
	testRejectedLicenses(t, base)
	testExcludedTokens(t, base)

	rows, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := CheckLicenses(rows); err != nil {
		t.Fatalf("CheckLicenses: %v", err)
	}
	if err := CheckExcluded(rows); err != nil {
		t.Fatalf("CheckExcluded: %v", err)
	}
}

func testAdmittedLicenses(t *testing.T, base Row) {
	t.Helper()
	for _, license := range []string{
		LicenseApache2, LicenseMIT, LicenseBSD3, LicenseCC0,
		LicenseCCBY4, LicensePublicDomain, LicenseRepoAuthored,
	} {
		row := base
		row.License = license
		if err := CheckLicenses([]Row{row}); err != nil {
			t.Errorf("%s: %v", license, err)
		}
	}
}

func testRejectedLicenses(t *testing.T, base Row) {
	t.Helper()
	for _, license := range []string{"AGPL-3.0", LicenseCCBYSA4, "", "Proprietary"} {
		row := base
		row.License = license
		if err := CheckLicenses([]Row{row}); err == nil {
			t.Errorf("license %q: CheckLicenses accepted the row", license)
		}
	}
	external := base
	external.Path = "external/pdf20examples/a.pdf"
	external.License = LicenseCCBYSA4
	if err := CheckLicenses([]Row{external}); err != nil {
		t.Errorf("external CC BY-SA: %v", err)
	}
}

func testExcludedTokens(t *testing.T, base Row) {
	t.Helper()
	for _, token := range ExcludedTokens() {
		row := base
		row.Path = "pdfa/" + token + ".pdf"
		if err := CheckExcluded([]Row{row}); err == nil {
			t.Errorf("%s: CheckExcluded accepted a committed row", token)
		}
		external := row
		external.Path = "external/" + token + ".pdf"
		if err := CheckExcluded([]Row{external}); err != nil {
			t.Errorf("%s external: %v", token, err)
		}
	}
}

// manifestLine renders one tab-separated manifest data line.
func manifestLine(path, digest, bytes, source, license, feature, expect string) string {
	return strings.Join([]string{path, digest, bytes, source, license, feature, expect}, "\t") + "\n"
}
