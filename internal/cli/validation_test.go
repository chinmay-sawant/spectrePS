package cli

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestValidationExitCodes locks exit 3 for a write failure on raster, rewrite,
// pdfimage, and ps, and exit 2 for a missing input path on raster, rewrite,
// ps, and text.
func TestValidationExitCodes(t *testing.T) {
	dir := t.TempDir()
	psPath := writeTemp(t, "in.ps", []byte("1 2 add"))
	pdfPath := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	outDir := filepath.Join(dir, "outdir")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")

	writeFailures := []struct {
		name string
		args []string
	}{
		{"raster", []string{"raster", "-o", outDir, "-w", "20", "-h", "20", "-r", "72", psPath}},
		{"rewrite", []string{"rewrite", "-o", outDir, pdfPath}},
		{"pdfimage", []string{"pdfimage", "-o", outDir, "-w", "20", "-h", "20", "-r", "72", psPath}},
		{"ps", []string{"ps", "-o", outDir, pdfPath}},
	}
	for _, tt := range writeFailures {
		t.Run("write "+tt.name, func(t *testing.T) {
			if code, _, _ := callRun(t, tt.args...); code != exitIO {
				t.Fatalf("code = %d, want %d", code, exitIO)
			}
		})
	}

	missingInputs := []struct {
		name string
		args []string
	}{
		{"raster", []string{"raster", "-o", filepath.Join(dir, "out.ppm"), missing}},
		{"rewrite", []string{"rewrite", "-o", filepath.Join(dir, "out.pdf"), missing}},
		{"ps", []string{"ps", "-o", filepath.Join(dir, "out.ps"), missing}},
		{"text", []string{"text", missing}},
	}
	for _, tt := range missingInputs {
		t.Run("missing "+tt.name, func(t *testing.T) {
			if code, _, _ := callRun(t, tt.args...); code != exitUsage {
				t.Fatalf("code = %d, want %d", code, exitUsage)
			}
		})
	}
}

// TestValidationFlagSurface locks the accepted-and-ignored flags and the
// rejected ones, each with its exit code.
func TestValidationFlagSurface(t *testing.T) {
	dir := t.TempDir()
	psPath := writeTemp(t, "in.ps", []byte("1 2 add"))
	runOut := filepath.Join(dir, "run-out.ppm")
	compareOut := filepath.Join(dir, "compare-out.ppm")
	outPDF := filepath.Join(dir, "out.pdf")

	accepted := []struct {
		name string
		args []string
		out  string
	}{
		{"run -o", []string{"run", "-o", runOut, psPath}, runOut},
		{"compare raster -o", []string{
			"compare", "raster", "-o", compareOut, "-w", "20", "-h", "20", "-r", "72", psPath, psPath,
		}, compareOut},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := callRun(t, tt.args...)
			if code != exitOK || stdout != "" || stderr != "" {
				t.Fatalf("code = %d stdout = %q stderr = %q, want exit 0 and no output", code, stdout, stderr)
			}
			if _, err := os.Stat(tt.out); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("%s wrote a file, want ignored", tt.out)
			}
		})
	}

	rejected := []struct {
		name string
		args []string
	}{
		{"run -colorspace", []string{"run", "-colorspace", "gray", psPath}},
		{"pdfimage -format", []string{"pdfimage", "-format", "png", "-o", outPDF, psPath}},
		{"pdfimage -jpegq", []string{"pdfimage", "-jpegq", "50", "-o", outPDF, psPath}},
		{"pdfimage -tiffcompress", []string{"pdfimage", "-tiffcompress", "none", "-o", outPDF, psPath}},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := callRun(t, tt.args...); code != exitUsage {
				t.Fatalf("code = %d, want %d", code, exitUsage)
			}
		})
	}
	if _, err := os.Stat(outPDF); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a rejected pdfimage wrote %s", outPDF)
	}
}

// validationImport is one internal/cli import path and whether a test file
// lists it. A test file may read the corpus manifest through
// internal/validation; production files may not.
type validationImport struct {
	path string
	test bool
}

// TestValidationNoProcess walks every non-ignored Go file in the module and
// fails on an os/exec, net, or cgo import, then checks that internal/cli
// imports only the public package, the standard library, and x/image/tiff.
func TestValidationNoProcess(t *testing.T) {
	root := filepath.Join("..", "..")
	cliDir := filepath.Clean(filepath.Join(root, "internal", "cli"))
	fset := token.NewFileSet()
	var cliImports []validationImport

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && validationSkipDir(path, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		imports, skip, parseErr := validationFileImports(fset, path, entry.Name())
		if parseErr != nil {
			return parseErr
		}
		if skip {
			return nil
		}
		validationCheckProcessImports(t, path, imports)
		if filepath.Dir(path) == cliDir {
			testFile := strings.HasSuffix(entry.Name(), "_test.go")
			for _, importPath := range imports {
				cliImports = append(cliImports, validationImport{path: importPath, test: testFile})
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if len(cliImports) == 0 {
		t.Fatal("walk found no internal/cli imports")
	}
	validationCheckCLIImports(t, cliImports)
}

// validationFileImports parses one Go file. The bool is true when the build
// ignores the file.
func validationFileImports(fset *token.FileSet, path, name string) ([]string, bool, error) {
	if !strings.HasSuffix(name, ".go") {
		return nil, true, nil
	}
	dir := filepath.Dir(path)
	inBuild, matchErr := build.Default.MatchFile(dir, name)
	if matchErr != nil {
		return nil, false, fmt.Errorf("%s: %w", path, matchErr)
	}
	if !inBuild {
		return nil, true, nil
	}
	parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if parseErr != nil {
		return nil, false, fmt.Errorf("%s: %w", path, parseErr)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return nil, false, fmt.Errorf("%s: %w", path, unquoteErr)
		}
		imports = append(imports, importPath)
	}
	return imports, false, nil
}

// validationCheckProcessImports fails on an os/exec, net, or cgo import.
func validationCheckProcessImports(t *testing.T, path string, imports []string) {
	t.Helper()
	for _, importPath := range imports {
		if importPath == "os/exec" || importPath == "net" ||
			strings.HasPrefix(importPath, "net/") || importPath == "C" {
			t.Errorf("%s imports %q", path, importPath)
		}
	}
}

// validationCheckCLIImports fails on an internal/cli import outside the public
// package, the standard library, and x/image/tiff. A test file may also read
// the validation corpus manifest through internal/validation.
func validationCheckCLIImports(t *testing.T, cliImports []validationImport) {
	t.Helper()
	const (
		publicPkg     = "github.com/chinmay-sawant/spectrePS/spectreps"
		tiffPkg       = "golang.org/x/image/tiff"
		validationPkg = "github.com/chinmay-sawant/spectrePS/internal/validation"
	)
	seen := make(map[string]bool, len(cliImports))
	for _, imp := range cliImports {
		first := imp.path
		if slash := strings.IndexByte(imp.path, '/'); slash >= 0 {
			first = imp.path[:slash]
		}
		if !strings.Contains(first, ".") {
			continue
		}
		allowed := imp.path == publicPkg || imp.path == tiffPkg ||
			(imp.test && imp.path == validationPkg)
		if !allowed {
			t.Errorf("internal/cli imports %q, want only %s, the standard library, or %s",
				imp.path, publicPkg, tiffPkg)
		}
		seen[imp.path] = true
	}
	if !seen[publicPkg] {
		t.Fatalf("internal/cli does not import %s", publicPkg)
	}
	if !seen[tiffPkg] {
		t.Fatalf("internal/cli does not import %s", tiffPkg)
	}
}

// validationSkipDir reports whether the walk skips the directory. Hidden
// directories, build output, proof-tool checkouts, and nested modules hold no
// module source.
func validationSkipDir(path, name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "bin", "references", "verapdf", "ghostscript", "vendor":
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return true
	}
	return false
}

// TestValidationRasterGeometry locks the default 612 by 792 at 72 dpi, the
// negative geometry limitcheck, and the -pages forms.
func TestValidationRasterGeometry(t *testing.T) {
	dir := t.TempDir()
	blank := writeTemp(t, "blank.ps", []byte(""))
	two := writeTemp(t, "two.ps", []byte("showpage showpage")) //nolint:dupword // two showpage operators produce two pages

	t.Run("default geometry", func(t *testing.T) {
		out := filepath.Join(dir, "default.ppm")
		want(t, []string{"raster", "-o", out, blank}, 0, "", "")
		width, height := validationPPMSize(t, out)
		if width != 612 || height != 792 {
			t.Fatalf("default raster = %dx%d, want 612x792", width, height)
		}
	})
	t.Run("negative geometry", func(t *testing.T) {
		out := filepath.Join(dir, "negative.ppm")
		for _, flag := range []string{"-w", "-h", "-r"} {
			args := []string{"raster", "-o", out, flag, "-1", blank}
			want(t, args, 1, "", "Error: /limitcheck in raster\n")
		}
	})
	t.Run("pages forms", func(t *testing.T) {
		pattern := filepath.Join(dir, "sel-%d.ppm")
		base := []string{"raster", "-o", pattern, "-w", "8", "-h", "8", "-r", "72"}
		want(t, append(base, "-pages", "2", two), 0, "", "")
		validationSinglePageOutput(t, dir, "sel-1")
		want(t, append(base, "-pages", "2-", two), 0, "", "")
		validationSinglePageOutput(t, dir, "sel-1")
		want(t, append(base, "-pages", "-1", two), 0, "", "")
		validationSinglePageOutput(t, dir, "sel-1")
		want(t, append(base, "-pages", "1-9", two), 0, "", "")
		if _, err := os.Stat(filepath.Join(dir, "sel-1.ppm")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "sel-2.ppm")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("pages errors", func(t *testing.T) {
		out := filepath.Join(dir, "err-%d.ppm")
		base := []string{"raster", "-o", out, "-w", "8", "-h", "8", "-r", "72"}
		want(t, append(base, "-pages", "3", two), 1, "", "Error: /rangecheck in pages\n")
		want(t, append(base, "-pages", "0", two), 1, "", "Error: /rangecheck in pages\n")
		wantCode(t, append(base, "-pages", "x", two), 2)
	})
}

func validationSinglePageOutput(t *testing.T, dir, stem string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, stem+".ppm")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sel-2.ppm")); err == nil {
		t.Fatalf("%s selected two pages", stem)
	}
}

// validationPPMSize reads the width and height from a P6 header.
func validationPPMSize(t *testing.T, path string) (int, int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const magic = "P6\n"
	if !bytes.HasPrefix(raw, []byte(magic)) {
		t.Fatalf("%s is not a P6 file", path)
	}
	rest := raw[len(magic):]
	end := bytes.IndexByte(rest, '\n')
	if end < 0 {
		t.Fatalf("%s has no header line", path)
	}
	fields := strings.Fields(string(rest[:end]))
	if len(fields) != 2 {
		t.Fatalf("%s size line = %q", path, rest[:end])
	}
	width, widthErr := strconv.Atoi(fields[0])
	height, heightErr := strconv.Atoi(fields[1])
	if widthErr != nil || heightErr != nil {
		t.Fatalf("%s size line = %q", path, rest[:end])
	}
	if !bytes.HasPrefix(rest[end+1:], []byte("255\n")) {
		t.Fatalf("%s has no max-value line", path)
	}
	return width, height
}

// TestValidationCompareCLIText locks the mismatch strings the CLI can reach:
// a pixel offset, a byte offset, and a length difference.
func TestValidationCompareCLIText(t *testing.T) {
	dir := t.TempDir()
	prog := []byte("0 0 moveto 10 0 lineto stroke")
	painted := filepath.Join(dir, "painted.ps")
	blank := filepath.Join(dir, "blank.ps")
	if err := os.WriteFile(painted, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blank, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	two := filepath.Join(dir, "two.ps")
	//nolint:dupword // two showpage operators produce two pages
	if err := os.WriteFile(two, []byte("showpage showpage"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := callRun(t, "compare", "raster", "-w", "20", "-h", "20", "-r", "72", painted, blank)
	if code != exitJob || stderr != "" || !strings.HasPrefix(stdout, "mismatch pixel ") {
		t.Fatalf("pixel mismatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = callRun(t, "compare", "raster", "-w", "8", "-h", "8", "-r", "72", two, painted)
	if code != exitJob || stderr != "" || stdout != "mismatch length\n" {
		t.Fatalf("length mismatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	byteA, byteB := writePair(t, []byte("hello"), []byte("hallo"))
	want(t, []string{"compare", "bytes", byteA, byteB}, 1, "mismatch byte 1\n", "")
	lenA, lenB := writePair(t, []byte("ab"), []byte("abcd"))
	want(t, []string{"compare", "bytes", lenA, lenB}, 1, "mismatch length 2\n", "")
}

// TestValidationCorpusMeasure runs bbox, inkcov, and ink_cov over the
// two-page fixture and the path PDF, with and without -pages. The path PDF
// case uses 144 dpi so the fractional pixel edge exercises floor and ceiling.
func TestValidationCorpusMeasure(t *testing.T) {
	twoPage := filepath.Join("..", "..", "sampledata", "fixtures", "gs-argv-input.pdf")
	pathOnly := filepath.Join("..", "..", "sampledata", "compress", "path.pdf")

	const twoPageAll = "%%BoundingBox: 0 0 20 20\n%%HiResBoundingBox: 0 0 20 20\n" +
		"%%BoundingBox: 0 0 20 20\n%%HiResBoundingBox: 0 0 20 20\n"
	const twoPageInkcov = "Page 1\n0.00000 1.00000 1.00000 RGB\nPage 2\n1.00000 0.00000 1.00000 RGB\n"
	const twoPageInkAmount = "Page 1\n0.00000 100.00000 100.00000 RGB\nPage 2\n100.00000 0.00000 100.00000 RGB\n"
	const twoPageSecond = "%%BoundingBox: 0 0 20 20\n%%HiResBoundingBox: 0 0 20 20\n"

	t.Run("two page no pages", func(t *testing.T) {
		validationSample(t, twoPage)
		want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", twoPage}, 0, twoPageAll, "")
		want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", twoPage}, 0, twoPageInkcov, "")
		want(t, []string{"ink_cov", "-w", "20", "-h", "20", "-r", "72", twoPage}, 0, twoPageInkAmount, "")
	})
	t.Run("two page with pages", func(t *testing.T) {
		validationSample(t, twoPage)
		want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", "-pages", "2", twoPage},
			0, twoPageSecond, "")
		want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", "-pages", "2", twoPage},
			0, "Page 1\n1.00000 0.00000 1.00000 RGB\n", "")
		want(t, []string{"ink_cov", "-w", "20", "-h", "20", "-r", "72", "-pages", "2", twoPage},
			0, "Page 1\n100.00000 0.00000 100.00000 RGB\n", "")
	})
	t.Run("path fractional edge", func(t *testing.T) {
		validationSample(t, pathOnly)
		want(t, []string{"bbox", "-w", "200", "-h", "200", "-r", "144", pathOnly},
			0, "%%BoundingBox: 0 99 200 101\n%%HiResBoundingBox: 0 99.5 200 100.5\n", "")
		want(t, []string{"inkcov", "-w", "200", "-h", "200", "-r", "144", pathOnly},
			0, "Page 1\n0.00000 0.00500 0.00500 RGB\n", "")
		want(t, []string{"ink_cov", "-w", "200", "-h", "200", "-r", "144", pathOnly},
			0, "Page 1\n0.00000 0.50000 0.50000 RGB\n", "")
	})
	t.Run("path with pages", func(t *testing.T) {
		validationSample(t, pathOnly)
		want(t, []string{"bbox", "-w", "200", "-h", "200", "-r", "144", "-pages", "1", pathOnly},
			0, "%%BoundingBox: 0 99 200 101\n%%HiResBoundingBox: 0 99.5 200 100.5\n", "")
		want(t, []string{"inkcov", "-w", "200", "-h", "200", "-r", "144", "-pages", "1", pathOnly},
			0, "Page 1\n0.00000 0.00500 0.00500 RGB\n", "")
		want(t, []string{"ink_cov", "-w", "200", "-h", "200", "-r", "144", "-pages", "1", pathOnly},
			0, "Page 1\n0.00000 0.50000 0.50000 RGB\n", "")
	})
}

func validationSample(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample absent: %v", err)
	}
}
