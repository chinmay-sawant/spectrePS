package cli

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

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
	path := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	wantCode(t, []string{"rewrite", path}, 2)
}

func TestRewriteCLI(t *testing.T) {
	src := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	wantCode(t, []string{"rewrite", src}, 2)

	dir := t.TempDir()
	out := filepath.Join(dir, "out.pdf")
	want(t, []string{"rewrite", "-o", out, src}, 0, "", "")
	checkRewritePDF(t, out, true)

	raw := filepath.Join(dir, "raw.pdf")
	want(t, []string{"rewrite", "-compress=false", "-o", raw, src}, 0, "", "")
	checkRewritePDF(t, raw, false)
}

func checkRewritePDF(t *testing.T, path string, flate bool) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-")) {
		t.Fatalf("%s prefix %q", path, pdfPrefix(got))
	}
	has := bytes.Contains(got, []byte("FlateDecode"))
	if has != flate {
		t.Fatalf("%s FlateDecode=%v, want %v", path, has, flate)
	}
}

func pdfPrefix(got []byte) []byte {
	if len(got) > 16 {
		return got[:16]
	}
	return got
}

func TestRasterPDF(t *testing.T) {
	src := writeTemp(t, "in.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	out := filepath.Join(t.TempDir(), "out.ppm")
	want(t, []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("P6")) {
		t.Fatalf("ppm prefix %q", ppmPrefix(got))
	}
	empty := writeTemp(t, "empty.pdf", emptyKidsPDF(t))
	emptyOut := filepath.Join(t.TempDir(), "empty.ppm")
	args := []string{"raster", "-o", emptyOut, "-w", "20", "-h", "20", "-r", "72", empty}
	want(t, args, 1, "", "Error: /rangecheck in RasterizePage\n")
}

func TestValidate(t *testing.T) {
	wantCode(t, []string{"validate", filepath.Join(t.TempDir(), "missing.ps")}, 2)
	path := writeTemp(t, "in.ps", []byte("show"))
	want(t, []string{"validate", path}, 1, "", "Error: /undefined in show\n")
}

func TestValidatePS(t *testing.T) {
	good := writeTemp(t, "good.ps", []byte("1 2 add"))
	want(t, []string{"validate", good}, 0, "", "")
	bad := writeTemp(t, "bad.ps", []byte("add"))
	want(t, []string{"validate", bad}, 1, "", "Error: /stackunderflow in add\n")
}

func TestValidatePDF(t *testing.T) {
	good := writeTemp(t, "good.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	want(t, []string{"validate", good}, 0, "", "")

	broken := writeTemp(t, "cut.pdf", onePagePDF(t, "q")[:20])
	code, _, stderr := callRun(t, "validate", broken)
	if code != 1 || len(stderr) < len("Error:") || stderr[:len("Error:")] != "Error:" {
		t.Fatalf("truncated xref: code=%d stderr=%q", code, stderr)
	}

	locked := writeTemp(t, "enc.pdf", encryptedPDF(t))
	want(t, []string{"validate", locked}, 1, "", "Error: /invalidaccess in Encrypt\n")

	text := writeTemp(t, "text.pdf", onePagePDF(t, "(Hi) Tj"))
	want(t, []string{"validate", text}, 1, "", "Error: /undefined in Tj\n")
}

func TestValidateBanned(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("stay"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "in.ps")
	prog := []byte(psString(keep) + " deletefile\n")
	if err := os.WriteFile(src, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	before := dirFiles(t, dir)
	want(t, []string{"validate", src}, 1, "", "Error: /invalidaccess in deletefile\n")
	if got := dirFiles(t, dir); got != before {
		t.Fatalf("dir files = %q, want %q", got, before)
	}
	body, err := os.ReadFile(keep)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "stay" {
		t.Fatalf("keep.txt = %q", body)
	}
}

func encryptedPDF(t *testing.T) []byte {
	t.Helper()
	src := onePagePDF(t, "q")
	old := []byte("/Root 1 0 R >>")
	next := []byte("/Root 1 0 R /Encrypt << /Filter /Standard >> >>")
	if !bytes.Contains(src, old) {
		t.Fatal("trailer marker missing")
	}
	return bytes.Replace(src, old, next, 1)
}

func psString(path string) string {
	var b strings.Builder
	b.WriteByte('(')
	for i := range len(path) {
		switch path[i] {
		case '\\', '(', ')':
			b.WriteByte('\\')
		}
		b.WriteByte(path[i])
	}
	b.WriteByte(')')
	return b.String()
}

func dirFiles(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, "\n")
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

func ppmPrefix(got []byte) []byte {
	if len(got) > 16 {
		return got[:16]
	}
	return got
}

func onePagePDF(t *testing.T, content string) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>"),
		flateStream(t, content),
	}
	return classicXref(t, objects)
}

func emptyKidsPDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [ ] /Count 0 >>"),
	}
	return classicXref(t, objects)
}

func flateStream(t *testing.T, content string) []byte {
	t.Helper()
	compressed := flateBytes(t, []byte(content))
	var body bytes.Buffer
	writef(t, &body, "<< /Length %d /Filter /FlateDecode >>\nstream\n", len(compressed))
	writeAll(t, &body, compressed)
	writeString(t, &body, "\nendstream")
	return body.Bytes()
}

func flateBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zlib.NewWriter(&body)
	if _, err := writer.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

// classicXref writes a classic xref. Each entry is 20 bytes, including the end-of-line.
func classicXref(t *testing.T, objects [][]byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writeString(t, &body, "%PDF-1.4\n")
	offsets := make([]int, 1, len(objects)+1)
	for i, object := range objects {
		offsets = append(offsets, body.Len())
		writef(t, &body, "%d 0 obj\n", i+1)
		writeAll(t, &body, object)
		writeString(t, &body, "\nendobj\n")
	}
	xrefAt := body.Len()
	writef(t, &body, "xref\n0 %d\n", len(offsets))
	writeString(t, &body, "0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		writef(t, &body, "%010d 00000 n \n", offset)
	}
	writef(t, &body, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n", len(offsets), xrefAt)
	writeString(t, &body, "%%EOF\n")
	pdf := body.Bytes()
	checkObjectOffsets(t, pdf, offsets)
	return pdf
}

func checkObjectOffsets(t *testing.T, pdf []byte, offsets []int) {
	t.Helper()
	for i, offset := range offsets {
		if i == 0 {
			continue
		}
		want := fmt.Sprintf("%d 0 obj\n", i)
		end := offset + len(want)
		if end > len(pdf) || string(pdf[offset:end]) != want {
			t.Fatalf("object %d at %d", i, offset)
		}
	}
}

func writef(t *testing.T, body *bytes.Buffer, format string, args ...any) {
	t.Helper()
	if _, err := fmt.Fprintf(body, format, args...); err != nil {
		t.Fatal(err)
	}
}

func writeAll(t *testing.T, body *bytes.Buffer, data []byte) {
	t.Helper()
	if _, err := body.Write(data); err != nil {
		t.Fatal(err)
	}
}

func writeString(t *testing.T, body *bytes.Buffer, text string) {
	t.Helper()
	if _, err := body.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
