// Package validation reads the checked-in validation corpus manifest and
// checks the committed tier under sampledata/validation.
//
// The manifest is sampledata/validation/manifest.tsv with seven columns:
// path, sha256, bytes, source, license, feature, and expect. Paths are
// relative to sampledata/validation and always use forward slashes. The
// committed tier is every row whose path does not start with "external/";
// those files are checked in. The external tier lives under
// sampledata/validation/external, is gitignored, and internal/validation/gen.go
// fetches it with -fetch-external. Tests skip the external tier when it is
// absent.
//
// The expect column is one of:
//
//   - paint: the corpus job rasterizes the file without an error and marks
//     the page where the case calls for marks.
//   - struct: the file opens and its page structure is the claim; painting is
//     not asserted.
//   - refuse:<JobError.Msg>: the corpus job returns an error whose Msg is the
//     exact text after refuse:, and writes no output.
//
// Every corpus test is named TestValidation<Area> so that
// go test -count=1 ./... -run TestValidation runs the group.
package validation

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ManifestName, TraceabilityName, and ReadmeName are the file names under
// sampledata/validation that are not corpus rows. README.md may sit in any
// folder and carries provenance prose.
const (
	ManifestName     = "manifest.tsv"
	TraceabilityName = "traceability.tsv"
	ReadmeName       = "README.md"
)

// ExternalPrefix marks a row in the fetched, non-committed tier.
const ExternalPrefix = "external/"

// Expect forms and the values that carry no error.
const (
	ExpectPaint  = "paint"
	ExpectStruct = "struct"
	// ExpectRefuse prefixes a row whose job must fail with a named error.
	ExpectRefuse = "refuse:"
)

// The admitted licenses, as the exact strings in the license column. The
// committed tier admits all of them. The external tier also admits
// LicenseCCBYSA4.
const (
	LicenseApache2      = "Apache-2.0"
	LicenseMIT          = "MIT"
	LicenseBSD3         = "BSD-3-Clause"
	LicenseCC0          = "CC0-1.0"
	LicenseCCBY4        = "CC-BY-4.0"
	LicensePublicDomain = "US-public-domain"
	LicenseRepoAuthored = "repo-authored"
	LicenseCCBYSA4      = "CC-BY-SA-4.0"
)

const (
	manifestHeader    = "path\tsha256\tbytes\tsource\tlicense\tfeature\texpect"
	manifestColumns   = 7
	sha256HexDigits   = 64
	scannerBufferSize = 64 << 10
	maxManifestLine   = 1 << 20
)

// Error is one manifest or corpus problem.
type Error struct {
	Path string // corpus-relative path, when the problem belongs to a row
	Line int    // manifest line, when the problem belongs to a row
	Msg  string
}

// Error returns the path or line, when known, followed by the message.
func (err *Error) Error() string {
	switch {
	case err.Path != "":
		return err.Path + ": " + err.Msg
	case err.Line > 0:
		return "line " + strconv.Itoa(err.Line) + ": " + err.Msg
	default:
		return err.Msg
	}
}

// Row is one manifest row.
type Row struct {
	Path    string // slash-separated, relative to sampledata/validation
	SHA256  string // lowercase hex
	Bytes   int64
	Source  string // pinned URL, or repo-authored
	License string
	Feature string
	Expect  string
}

// External reports whether the row lives in the fetched, non-committed tier.
func (row Row) External() bool {
	return strings.HasPrefix(row.Path, ExternalPrefix)
}

// Refused reports whether the row expects a named refusal.
func (row Row) Refused() bool {
	return strings.HasPrefix(row.Expect, ExpectRefuse)
}

// RefuseError returns the expected JobError.Msg for a refusal row, or "".
func (row Row) RefuseError() string {
	if !row.Refused() {
		return ""
	}
	return strings.TrimPrefix(row.Expect, ExpectRefuse)
}

// Root returns the module root directory, resolved from this source file so
// the caller's working directory does not matter.
func Root() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("validation: cannot locate the module root")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// CorpusDir returns the absolute path of sampledata/validation.
func CorpusDir() string {
	return filepath.Join(Root(), "sampledata", "validation")
}

// Load reads and parses sampledata/validation/manifest.tsv.
func Load() ([]Row, error) {
	return LoadFile(filepath.Join(CorpusDir(), ManifestName))
}

// LoadFile reads and parses one manifest file.
func LoadFile(path string) ([]Row, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rows, err := ParseManifest(data)
	if err != nil {
		return nil, &Error{Path: path, Line: 0, Msg: err.Error()}
	}
	return rows, nil
}

// ParseManifest parses the manifest bytes. Every row needs a path, a 64-digit
// lowercase SHA-256, a positive byte count, a non-empty source, license, and
// feature, and a valid expect value. Paths are unique and stay inside the
// corpus directory.
func ParseManifest(data []byte) ([]Row, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, scannerBufferSize), maxManifestLine)
	rows := []Row{}
	seen := map[string]bool{}
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimRight(scanner.Text(), "\r")
		if line == 1 {
			if text != manifestHeader {
				return nil, rowError(line, fmt.Sprintf("header %q, want %q", text, manifestHeader))
			}
			continue
		}
		row, err := parseRow(line, text)
		if err != nil {
			return nil, err
		}
		if seen[row.Path] {
			return nil, rowError(line, "duplicate path "+row.Path)
		}
		seen[row.Path] = true
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rowError(0, "no rows")
	}
	return rows, nil
}

// parseRow checks one data line. The caller has consumed the header.
func parseRow(line int, text string) (Row, error) {
	if text == "" {
		return Row{}, rowError(line, "empty line")
	}
	fields := strings.Split(text, "\t")
	if len(fields) != manifestColumns {
		return Row{}, rowError(line, fmt.Sprintf("%d columns, want %d", len(fields), manifestColumns))
	}
	var row Row
	row.Path = fields[0]
	row.SHA256 = fields[1]
	row.Source = fields[3]
	row.License = fields[4]
	row.Feature = fields[5]
	row.Expect = fields[6]
	if reason := pathReason(row.Path); reason != "" {
		return Row{}, rowError(line, reason)
	}
	if reason := sha256Reason(row.SHA256); reason != "" {
		return Row{}, rowError(line, reason)
	}
	bytes, reason := parseBytes(fields[2])
	if reason != "" {
		return Row{}, rowError(line, "bytes: "+reason)
	}
	row.Bytes = bytes
	if reason := emptyFieldReason(row); reason != "" {
		return Row{}, rowError(line, reason)
	}
	if reason := expectReason(row.Expect); reason != "" {
		return Row{}, rowError(line, reason)
	}
	return row, nil
}

// pathReason rejects absolute paths, parent jumps, and backslashes.
func pathReason(path string) string {
	switch {
	case path == "":
		return "path is empty"
	case strings.Contains(path, "\\"):
		return fmt.Sprintf("path %q contains a backslash", path)
	case strings.HasPrefix(path, "/"):
		return fmt.Sprintf("path %q is absolute", path)
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	switch {
	case cleaned != path:
		return fmt.Sprintf("path %q is not clean", path)
	case cleaned == ".." || strings.HasPrefix(cleaned, "../"):
		return fmt.Sprintf("path %q leaves the corpus directory", path)
	}
	return ""
}

func sha256Reason(text string) string {
	if len(text) != sha256HexDigits {
		return fmt.Sprintf("sha256 %q is not %d hex digits", text, sha256HexDigits)
	}
	if strings.ToLower(text) != text {
		return fmt.Sprintf("sha256 %q has an uppercase digit", text)
	}
	if _, err := hex.DecodeString(text); err != nil {
		return fmt.Sprintf("sha256 %q is not hex", text)
	}
	return ""
}

func parseBytes(text string) (int64, string) {
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Sprintf("%q is not an integer", text)
	}
	if value <= 0 {
		return 0, fmt.Sprintf("%d is not positive", value)
	}
	return value, ""
}

func emptyFieldReason(row Row) string {
	for _, pair := range []struct{ name, value string }{
		{"source", row.Source},
		{"license", row.License},
		{"feature", row.Feature},
		{"expect", row.Expect},
	} {
		if strings.TrimSpace(pair.value) == "" {
			return pair.name + " is empty"
		}
	}
	return ""
}

func expectReason(expect string) string {
	switch expect {
	case ExpectPaint, ExpectStruct:
		return ""
	}
	if !strings.HasPrefix(expect, ExpectRefuse) {
		return fmt.Sprintf("expect %q is not paint, struct, or refuse:<error>", expect)
	}
	if strings.TrimSpace(strings.TrimPrefix(expect, ExpectRefuse)) == "" {
		return fmt.Sprintf("expect %q names no error", expect)
	}
	return ""
}

// AdmittedLicense reports whether the license gate admits one row. A
// CC BY-SA 4.0 row is admitted in the external tier only.
func AdmittedLicense(license string, external bool) bool {
	switch license {
	case LicenseApache2, LicenseMIT, LicenseBSD3, LicenseCC0,
		LicenseCCBY4, LicensePublicDomain, LicenseRepoAuthored:
		return true
	case LicenseCCBYSA4:
		return external
	}
	return false
}

// ExcludedTokens returns the source tokens that may not appear in a committed
// row. The list follows the license gate in plans/v0.0.4/5-validation.md row
// 1.4: the Isartor files, the GhostPDL and MuPDF examples, the iText
// resources, the PDFBox testfiles, and the pdf.js files built around
// third-party assets.
func ExcludedTokens() []string {
	return []string{
		"isartor",
		"ghostpdl",
		"mupdf",
		"itext",
		"pdfbox",
		"tracemonkey",
		"tamreview",
		"firefox_logo",
		"pdfjs_wikipedia",
		"22060_a1_01_plans",
		"openoffice",
		"agpl",
	}
}

// CheckFiles checks every committed digest and byte count, and fails on a file
// under dir that has no manifest row. External rows are checked when the file
// is present and skipped when it is absent, because the tier is optional.
func CheckFiles(rows []Row, dir string) error {
	if err := checkRowFiles(rows, dir); err != nil {
		return err
	}
	return checkUnlisted(rows, dir)
}

// checkRowFiles checks the digest and byte count of every row.
func checkRowFiles(rows []Row, dir string) error {
	listed := make(map[string]bool, len(rows))
	for _, row := range rows {
		if listed[row.Path] {
			return pathError(row.Path, "duplicate manifest row")
		}
		listed[row.Path] = true
		full := filepath.Join(dir, filepath.FromSlash(row.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) && row.External() {
				continue
			}
			return fmt.Errorf("%s: %w", row.Path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != row.SHA256 {
			return pathError(row.Path, fmt.Sprintf("sha256 %s, want %s", got, row.SHA256))
		}
		if int64(len(data)) != row.Bytes {
			return pathError(row.Path, fmt.Sprintf("%d bytes, want %d", len(data), row.Bytes))
		}
	}
	return nil
}

// checkUnlisted walks the committed tier and fails on a file with no row.
func checkUnlisted(rows []Row, dir string) error {
	listed := make(map[string]bool, len(rows))
	for _, row := range rows {
		listed[row.Path] = true
	}
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == strings.TrimSuffix(ExternalPrefix, "/") {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == ManifestName || rel == TraceabilityName || filepath.Base(rel) == ReadmeName {
			return nil
		}
		if !listed[rel] {
			return pathError(rel, "file is not in the manifest")
		}
		return nil
	})
}

// CheckLicenses applies the license gate.
func CheckLicenses(rows []Row) error {
	for _, row := range rows {
		if !AdmittedLicense(row.License, row.External()) {
			return pathError(row.Path, fmt.Sprintf("license %q is not admitted", row.License))
		}
	}
	return nil
}

// CheckExcluded rejects the source families that stay out of the committed
// tier even when their license passes the gate.
func CheckExcluded(rows []Row) error {
	for _, row := range rows {
		if row.External() {
			continue
		}
		haystack := strings.ToLower(row.Path + " " + row.Source)
		for _, token := range ExcludedTokens() {
			if strings.Contains(haystack, token) {
				return pathError(row.Path, fmt.Sprintf("source matches the excluded token %q", token))
			}
		}
	}
	return nil
}

func rowError(line int, msg string) error {
	return &Error{Path: "", Line: line, Msg: msg}
}

func pathError(path, msg string) error {
	return &Error{Path: path, Line: 0, Msg: msg}
}
