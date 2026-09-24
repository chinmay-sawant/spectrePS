package cli

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

const notImplemented = "spectreps: not implemented\n"

func callRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func want(t *testing.T, args []string, code int, stdout, stderr string) {
	t.Helper()
	gotCode, gotOut, gotErr := callRun(t, args...)
	if gotCode != code || gotOut != stdout || gotErr != stderr {
		t.Fatalf("run %q: code=%d stdout=%q stderr=%q, want code=%d stdout=%q stderr=%q",
			args, gotCode, gotOut, gotErr, code, stdout, stderr)
	}
}

func wantCode(t *testing.T, args []string, code int) {
	t.Helper()
	got, _, _ := callRun(t, args...)
	if got != code {
		t.Fatalf("run %q: code=%d, want %d", args, got, code)
	}
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func writePair(t *testing.T, a, b []byte) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a")
	pathB := filepath.Join(dir, "b")
	if err := os.WriteFile(pathA, a, 0o600); err != nil {
		t.Fatalf("write %s: %v", pathA, err)
	}
	if err := os.WriteFile(pathB, b, 0o600); err != nil {
		t.Fatalf("write %s: %v", pathB, err)
	}
	return pathA, pathB
}

func TestVersion(t *testing.T) {
	want(t, []string{"version"}, 0, "0.0.1\n", "")
	wantCode(t, []string{"version", "extra"}, 2)
}

func TestUsage(t *testing.T) {
	wantCode(t, nil, 2)
	wantCode(t, []string{"no-such-command"}, 2)
	wantCode(t, []string{"run", "-z"}, 2)
	wantCode(t, []string{"run"}, 2)
	wantCode(t, []string{"compare"}, 2)
	wantCode(t, []string{"compare", "bytes", writeTemp(t, "a", []byte("x"))}, 2)
	wantCode(t, []string{"validate", "-o", "out"}, 2)
}

func TestRun(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ps")
	wantCode(t, []string{"run", missing}, 2)
	wantCode(t, []string{"run", t.TempDir()}, 3)

	path := writeTemp(t, "in.ps", []byte("1 2 add"))
	want(t, []string{"run", path}, 0, "", "")
	want(t, []string{"run", "-w", "200", "-h", "100", "-r", "72", path}, 0, "", "")
	bad := writeTemp(t, "bad.ps", []byte("show"))
	want(t, []string{"run", bad}, 1, "", "Error: /undefined in show\n")
}

func TestRaster(t *testing.T) {
	path := writeTemp(t, "in.ps", []byte("1 2 add"))
	wantCode(t, []string{"raster", path}, 2)
	out := filepath.Join(t.TempDir(), "out.ppm")
	want(t, []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", path}, 0, "", "")
}

func TestRewrite(t *testing.T) {
	path := writeTemp(t, "in.pdf", []byte("%PDF"))
	wantCode(t, []string{"rewrite", path}, 2)
	want(t, []string{"rewrite", "-o", "out.pdf", path}, 1, "", notImplemented)
}

func TestValidate(t *testing.T) {
	wantCode(t, []string{"validate", filepath.Join(t.TempDir(), "missing.ps")}, 2)
	path := writeTemp(t, "in.ps", []byte("show"))
	want(t, []string{"validate", path}, 1, "", "Error: /undefined in show\n")
}

func TestCompareBytes(t *testing.T) {
	equalA, equalB := writePair(t, []byte("hello"), []byte("hello"))
	want(t, []string{"compare", "bytes", equalA, equalB}, 0, "", "")

	byteA, byteB := writePair(t, []byte("hello"), []byte("hallo"))
	want(t, []string{"compare", "bytes", byteA, byteB}, 1, "mismatch byte 1\n", "")

	lenA, lenB := writePair(t, []byte("ab"), []byte("abcd"))
	want(t, []string{"compare", "bytes", lenA, lenB}, 1, "mismatch length 2\n", "")

	file, _ := writePair(t, []byte("ab"), []byte("ab"))
	missing := filepath.Join(t.TempDir(), "missing")
	wantCode(t, []string{"compare", "bytes", file, missing}, 2)

	dir := t.TempDir()
	one := filepath.Join(dir, "a")
	if err := os.WriteFile(one, []byte("ab"), 0o600); err != nil {
		t.Fatalf("write %s: %v", one, err)
	}
	wantCode(t, []string{"compare", "bytes", one, dir}, 3)
}

func TestCompareRaster(t *testing.T) {
	file, _ := writePair(t, []byte("1 2 add"), []byte("1 2 add"))
	missing := filepath.Join(t.TempDir(), "missing")
	wantCode(t, []string{"compare", "raster", file, missing}, 2)
}

func TestRasterFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.ps")
	if err := os.WriteFile(src, []byte("0 0 moveto 10 0 lineto stroke"), 0o600); err != nil {
		t.Fatal(err)
	}
	ppm := filepath.Join(dir, "out.ppm")
	want(t, []string{"raster", "-o", ppm, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	pngPath := filepath.Join(dir, "out.png")
	want(t, []string{"raster", "-o", pngPath, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	comparePPMAndPNG(t, ppm, pngPath)
	checkPagedRaster(t, dir)
}

func comparePPMAndPNG(t *testing.T, ppmPath, pngPath string) {
	t.Helper()
	ppmBytes, err := os.ReadFile(ppmPath)
	if err != nil {
		t.Fatal(err)
	}
	pngBytes, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	header := []byte("P6\n20 20\n255\n")
	if !bytes.HasPrefix(ppmBytes, header) || len(ppmBytes) != len(header)+20*20*3 {
		t.Fatalf("ppm len %d", len(ppmBytes))
	}
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 20 || decoded.Bounds().Dy() != 20 {
		t.Fatalf("png bounds %v", decoded.Bounds())
	}
	matchPNGBody(t, decoded, ppmBytes[len(header):])
}

func matchPNGBody(t *testing.T, decoded image.Image, body []byte) {
	t.Helper()
	for y := range 20 {
		for x := range 20 {
			red, green, blue, _ := decoded.At(x, y).RGBA()
			i := (y*20 + x) * 3
			if byte(red>>8) != body[i] || byte(green>>8) != body[i+1] || byte(blue>>8) != body[i+2] {
				t.Fatalf("png pixel %d,%d", x, y)
			}
		}
	}
}

func checkPagedRaster(t *testing.T, dir string) {
	t.Helper()
	two := filepath.Join(dir, "two.ps")
	//nolint:dupword // two showpage operators produce two pages
	if err := os.WriteFile(two, []byte("showpage showpage"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "pages.ppm")
	wantCode(t, []string{"raster", "-o", plain, "-w", "8", "-h", "8", "-r", "72", two}, 2)
	pattern := filepath.Join(dir, "page-%d.ppm")
	want(t, []string{"raster", "-o", pattern, "-w", "8", "-h", "8", "-r", "72", two}, 0, "", "")
	if _, err := os.Stat(filepath.Join(dir, "page-1.ppm")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "page-2.ppm")); err != nil {
		t.Fatal(err)
	}
}

func TestCompareRasterCLI(t *testing.T) {
	dir := t.TempDir()
	sameA := filepath.Join(dir, "a.ps")
	sameB := filepath.Join(dir, "b.ps")
	prog := []byte("0 0 moveto 10 0 lineto stroke")
	if err := os.WriteFile(sameA, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sameB, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, []string{"compare", "raster", "-r", "72", "-w", "20", "-h", "20", sameA, sameB}, 0, "", "")
	blank := filepath.Join(dir, "blank.ps")
	if err := os.WriteFile(blank, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := callRun(t, "compare", "raster", "-r", "72", "-w", "20", "-h", "20", sameA, blank)
	prefix := "mismatch pixel "
	if code != 1 || stderr != "" || len(stdout) < len(prefix) || stdout[:len(prefix)] != prefix {
		t.Fatalf("mismatch code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCompareSameOptions(t *testing.T) {
	dir := t.TempDir()
	prog := []byte("0 0 moveto 8 0 lineto stroke")
	a := filepath.Join(dir, "a.ps")
	b := filepath.Join(dir, "b.ps")
	if err := os.WriteFile(a, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, []string{"compare", "raster", "-r", "72", "-w", "16", "-h", "16", a, b}, 0, "", "")
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	lowOpt := spectreps.RunOptions{PageWidthPt: 16, PageHeightPt: 16, ResolutionDPI: 72}
	highOpt := spectreps.RunOptions{PageWidthPt: 16, PageHeightPt: 16, ResolutionDPI: 144}
	low, err := in.RunPostScript(t.Context(), prog, lowOpt)
	if err != nil {
		t.Fatal(err)
	}
	high, err := in.RunPostScript(t.Context(), prog, highOpt)
	if err != nil {
		t.Fatal(err)
	}
	if spectreps.CompareRaster(low[0], high[0]).Equal {
		t.Fatal("program did not change with resolution")
	}
}
