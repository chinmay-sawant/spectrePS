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

// TestValidationManifestLabelAxes requires the four label columns to be
// present, in the closed vocabularies, with a non-negative page count. A typo
// in a label has to fail the parse rather than silently create a new report
// group, because the pass-rate report joins on these strings.
func TestValidationManifestLabelAxes(t *testing.T) {
	sha := strings.Repeat("a", 64)
	repo := LicenseRepoAuthored
	cases := []struct {
		name   string
		labels []string
		want   string
	}{
		{"unknown area", []string{"nosucharea", ProbeInfo, BasisSpec, "0"}, "area"},
		{"unknown probe", []string{"paths", "nosuchprobe", BasisSpec, "0"}, "probe"},
		{"unknown basis", []string{"paths", ProbeInfo, "nosuchbasis", "0"}, "basis"},
		{"negative pages", []string{"paths", ProbeInfo, BasisSpec, "-1"}, "pages"},
		{"non-numeric pages", []string{"paths", ProbeInfo, BasisSpec, "many"}, "pages"},
		{"empty area", []string{"", ProbeInfo, BasisSpec, "0"}, "area is empty"},
		{"empty basis", []string{"paths", ProbeInfo, "", "0"}, "basis is empty"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			bad := manifestLine("paths/a.pdf", sha, "12", repo, repo,
				"a feature", ExpectPaint, testCase.labels...)
			_, err := ParseManifest([]byte(manifestHeader + "\n" + bad))
			if err == nil {
				t.Fatalf("ParseManifest accepted %v", testCase.labels)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not mention %q", err, testCase.want)
			}
		})
	}
	t.Run("every label is accepted", func(t *testing.T) {
		for _, area := range KnownAreas() {
			for _, probe := range KnownProbes() {
				for _, basis := range KnownBasis() {
					assertLabelTriple(t, area, probe, basis)
				}
			}
		}
	})
}

// assertLabelTriple requires one area, probe, and basis combination to parse
// and to survive the round trip. The caller owns the three nested loops so this
// stays a single assertion.
func assertLabelTriple(t *testing.T, area, probe, basis string) {
	t.Helper()
	sha := strings.Repeat("a", 64)
	repo := LicenseRepoAuthored
	line := manifestLine("paths/a.pdf", sha, "12", repo, repo,
		"a feature", ExpectSurvive, area, probe, basis, "3")
	rows, err := ParseManifest([]byte(manifestHeader + "\n" + line))
	if err != nil {
		t.Fatalf("%s/%s/%s: %v", area, probe, basis, err)
	}
	row := rows[0]
	if row.Area != area || row.Probe != probe || row.Basis != basis {
		t.Fatalf("parsed %+v, want %s/%s/%s", row, area, probe, basis)
	}
	if row.Pages != 3 {
		t.Fatalf("pages %d, want 3", row.Pages)
	}
	if !row.Survived() {
		t.Fatalf("%+v: Survived() is false", row)
	}
}

// TestValidationManifestGated requires a baseline row to be reported rather
// than asserted, which is what makes the baseline count in the report a debt
// that has to come down.
func TestValidationManifestGated(t *testing.T) {
	base := Row{Basis: BasisSpec}
	if !base.Gated() {
		t.Error("a spec row is not gated")
	}
	gs := Row{Basis: BasisGS}
	if !gs.Gated() {
		t.Error("a gs row is not gated")
	}
	baseline := Row{Basis: BasisBaseline}
	if baseline.Gated() {
		t.Error("a baseline row is gated, so a recorded outcome became a gate")
	}
}

// TestValidationManifestCoverage requires the checked-in manifest to exercise
// the fetched tier and both label axes, so a refactor cannot quietly drop the
// live folder or the report.
func TestValidationManifestCoverage(t *testing.T) {
	rows, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	counts := map[string]map[string]int{}
	var external, survive int
	for _, row := range rows {
		tally(counts, row)
		if row.External() {
			external++
		}
		if row.Survived() {
			survive++
		}
	}
	if external == 0 {
		t.Error("no external rows, so the live tier is unreachable from the manifest")
	}
	if survive == 0 {
		t.Error("no survive rows, so the robustness contract is untested")
	}
	requireLabel(t, counts, "area", "handbuilt")
	requireLabel(t, counts, "area", "archival")
	requireLabel(t, counts, "area", "postscript")
	requireLabel(t, counts, "basis", BasisBaseline)
	requireLabel(t, counts, "basis", BasisSpec)
	requireLabel(t, counts, "probe", ProbePS)
}

// tally counts one row under each of its three label values.
func tally(counts map[string]map[string]int, row Row) {
	for _, label := range []struct{ field, value string }{
		{"area", row.Area}, {"probe", row.Probe}, {"basis", row.Basis},
	} {
		if counts[label.field] == nil {
			counts[label.field] = map[string]int{}
		}
		counts[label.field][label.value]++
	}
}

// requireLabel reports a missing label value. The point of each one is named at
// the call site, because a bare count does not say what is missing.
func requireLabel(t *testing.T, counts map[string]map[string]int, field, value string) {
	t.Helper()
	if counts[field][value] == 0 {
		t.Errorf("no rows with %s %q, so the report cannot show that group", field, value)
	}
}

// manifestLine renders one tab-separated manifest data line.
// manifestLine builds one 11-column data row. The four label columns are
// supplied by the caller so a synthetic row can exercise a specific area,
// probe, basis, or page count.
func manifestLine(path, digest, bytes, source, license, feature, expect string, labels ...string) string {
	fields := []string{path, digest, bytes, source, license, feature, expect}
	fields = append(fields, defaultLabels()...)
	if len(labels) > 0 {
		// An override replaces from area onward, so a test can set basis or
		// pages without spelling out the whole tail.
		for i, v := range labels {
			fields[7+i] = v
		}
	}
	return strings.Join(fields, "\t") + "\n"
}

// defaultLabels returns a valid label tail: the paths area, the info probe,
// the spec basis, and no page assertion.
func defaultLabels() []string {
	return []string{"paths", ProbeInfo, BasisSpec, "0"}
}

// TestValidationManifestSurviveRequiresNoErrorText requires survive to be a bare
// form and pins the rest of the expect vocabulary. A survive row asserts
// robustness, so pinning an error string on one would turn it into a refusal row
// and defeat the point.
func TestValidationManifestSurviveRequiresNoErrorText(t *testing.T) {
	sha := strings.Repeat("a", 64)
	repo := LicenseRepoAuthored
	line := manifestLine("handbuilt/a.pdf", sha, "12", repo, repo,
		"a defective file", ExpectSurvive)
	rows, err := ParseManifest([]byte(manifestHeader + "\n" + line))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !rows[0].Survived() || rows[0].Refused() {
		t.Fatalf("%+v: want survive and not refused", rows[0])
	}
	for _, bare := range []string{ExpectPaint, ExpectStruct, ExpectSurvive} {
		if reason := expectReason(bare); reason != "" {
			t.Errorf("%s: %s", bare, reason)
		}
	}
	if reason := expectReason(ExpectRefuse); reason == "" {
		t.Error("refuse with no error text was accepted")
	}
	if reason := expectReason(ExpectRefuse + "limitcheck"); reason != "" {
		t.Errorf("refuse:limitcheck: %s", reason)
	}
	if reason := expectReason(ExpectSurvive + "limitcheck"); reason == "" {
		t.Error("survive with an error suffix was accepted")
	}
}
