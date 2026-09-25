package cli

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

func TestRewriteLevels(t *testing.T) {
	checkRewriteLevelPages(t)
	checkRewriteLevelContent(t)
	checkRewriteLevelUsage(t)
}

// checkRewriteLevelPages runs every level on a two-page path fixture and checks
// the output page count. Level 0 keeps the default rewrite bytes.
func checkRewriteLevelPages(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "levels.pdf", twoPagePlainPDF(t))
	dir := t.TempDir()
	for level := 0; level <= maxRewriteLevel; level++ {
		out := filepath.Join(dir, fmt.Sprintf("level%d.pdf", level))
		want(t, []string{"rewrite", "-level", strconv.Itoa(level), "-o", out, src}, 0, "", "")
		if got := openPDFBytes(t, readPayload(t, out)).PageCount(); got != 2 {
			t.Fatalf("level %d: PageCount = %d, want 2", level, got)
		}
	}
	defaultOut := filepath.Join(dir, "default.pdf")
	want(t, []string{"rewrite", "-o", defaultOut, src}, 0, "", "")
	levelZero := filepath.Join(dir, "zero.pdf")
	want(t, []string{"rewrite", "-level", "0", "-o", levelZero, src}, 0, "", "")
	if !bytes.Equal(readPayload(t, defaultOut), readPayload(t, levelZero)) {
		t.Fatal("-level 0 changed the default rewrite bytes")
	}
}

// checkRewriteLevelContent checks that the pass-through levels keep a text
// stream that the level 0 emitter rejects.
func checkRewriteLevelContent(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "text.pdf", textPagePDF(t))
	dir := t.TempDir()
	levelZero := filepath.Join(dir, "zero.pdf")
	wantCode(t, []string{"rewrite", "-o", levelZero, src}, 1)
	if _, err := os.Stat(levelZero); err == nil {
		t.Fatal("level 0 wrote a file for text content")
	}
	for level := 1; level <= maxRewriteLevel; level++ {
		out := filepath.Join(dir, fmt.Sprintf("level%d.pdf", level))
		want(t, []string{"rewrite", "-level", strconv.Itoa(level), "-o", out, src}, 0, "", "")
		if got := openPDFBytes(t, readPayload(t, out)).PageCount(); got != 1 {
			t.Fatalf("level %d: PageCount = %d, want 1", level, got)
		}
	}
}

func checkRewriteLevelUsage(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "usage.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	out := filepath.Join(t.TempDir(), "out.pdf")
	for _, level := range []string{"6", "-1"} {
		code, _, stderr := callRun(t, "rewrite", "-level", level, "-o", out, src)
		if code != exitUsage {
			t.Fatalf("-level %s: code = %d, want %d", level, code, exitUsage)
		}
		wantMsg := fmt.Sprintf("spectreps: -level wants 0 through 5, got %s\n", level)
		if stderr != wantMsg {
			t.Fatalf("-level %s: stderr = %q, want %q", level, stderr, wantMsg)
		}
	}
}

func TestRewriteSamples(t *testing.T) {
	dir := filepath.Join("..", "..", "sampledata", "compress")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("sample directory absent: %v", err)
	}
	for _, name := range []string{"whatisthis.pdf", "path.pdf"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("sample absent: %v", err)
			}
			checkRewriteSample(t, path, name == "whatisthis.pdf")
		})
	}
}

// checkRewriteSample rewrites one sample at every level. The page count must
// match, a JPEG image must decode, and level 5 must be the smallest.
func checkRewriteSample(t *testing.T, path string, hasImage bool) {
	t.Helper()
	src := readPayload(t, path)
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	sizes, bounds := rewriteSampleLevels(t, in, doc, doc.PageCount(), hasImage)
	checkSampleSizes(t, sizes, hasImage)
	if hasImage {
		checkSampleCaps(t, bounds)
	}
}

func rewriteSampleLevels(
	t *testing.T,
	in *spectreps.Instance,
	doc *spectreps.Document,
	wantPages int,
	hasImage bool,
) ([]int, []image.Rectangle) {
	t.Helper()
	sizes := make([]int, maxRewriteLevel+1)
	bounds := make([]image.Rectangle, maxRewriteLevel+1)
	for level := 1; level <= maxRewriteLevel; level++ {
		out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: level})
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		sizes[level] = len(out)
		outDoc, err := in.OpenPDF(t.Context(), out)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if outDoc.PageCount() != wantPages {
			t.Fatalf("level %d: PageCount = %d, want %d", level, outDoc.PageCount(), wantPages)
		}
		images := decodedJPEGImages(t, level, out)
		if hasImage && len(images) != 1 {
			t.Fatalf("level %d: decoded %d JPEG images, want 1", level, len(images))
		}
		if len(images) > 0 {
			bounds[level] = images[0]
		}
	}
	return sizes, bounds
}

func checkSampleSizes(t *testing.T, sizes []int, hasImage bool) {
	t.Helper()
	for level := 1; level < maxRewriteLevel; level++ {
		if sizes[maxRewriteLevel] > sizes[level] {
			t.Fatalf("level 5 wrote %d bytes, level %d wrote %d", sizes[maxRewriteLevel], level, sizes[level])
		}
	}
	if hasImage && sizes[maxRewriteLevel] >= sizes[1] {
		t.Fatalf("level 5 wrote %d bytes, level 1 wrote %d", sizes[maxRewriteLevel], sizes[1])
	}
}

// decodedJPEGImages decodes every image stream that reads as a JPEG. A stream
// that does not decode is skipped, and the caller checks the count it needs.
func decodedJPEGImages(t *testing.T, level int, payload []byte) []image.Rectangle {
	t.Helper()
	var out []image.Rectangle
	marker := []byte("/Subtype /Image")
	rest := payload
	for {
		idx := bytes.Index(rest, marker)
		if idx < 0 {
			return out
		}
		rest = rest[idx:]
		streamAt := bytes.Index(rest, []byte("stream\n"))
		if streamAt < 0 {
			t.Fatalf("level %d: image stream start missing", level)
		}
		body := rest[streamAt+len("stream\n"):]
		end := bytes.Index(body, []byte("\nendstream"))
		if end < 0 {
			t.Fatalf("level %d: image stream end missing", level)
		}
		if pic, err := jpeg.Decode(bytes.NewReader(body[:end])); err == nil {
			out = append(out, pic.Bounds())
		}
		rest = body[end:]
	}
}

// checkSampleCaps checks the longest-side caps at levels 3 through 5.
func checkSampleCaps(t *testing.T, bounds []image.Rectangle) {
	t.Helper()
	source := bounds[1]
	if source.Dx() == 0 {
		t.Fatal("level 1 image missing")
	}
	for _, testCase := range []struct{ level, sideCap int }{
		{level: 3, sideCap: 1754},
		{level: 4, sideCap: 1123},
		{level: 5, sideCap: 842},
	} {
		got := bounds[testCase.level]
		if got.Dx() == 0 {
			t.Fatalf("level %d image missing", testCase.level)
		}
		if source.Dx() <= testCase.sideCap && source.Dy() <= testCase.sideCap {
			if got != source {
				t.Fatalf("level %d resampled an image under the cap", testCase.level)
			}
			continue
		}
		if longest := max(got.Dx(), got.Dy()); longest != testCase.sideCap {
			t.Fatalf("level %d longest side = %d, want %d", testCase.level, longest, testCase.sideCap)
		}
	}
}

func twoPagePlainPDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>"),
		plainStream(t, "0 0 m 10 0 l S"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R /Resources << >> >>"),
		plainStream(t, "0 0 m 5 5 l S"),
	}
	return classicXref(t, objects)
}

func textPagePDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		plainStream(t, "BT /F1 12 Tf 5 5 Td (Hi) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
	return classicXref(t, objects)
}

func plainStream(t *testing.T, content string) []byte {
	t.Helper()
	return []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
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

func TestRasterJPEG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "solid.ps")
	prog := []byte("0 0.5 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill")
	if err := os.WriteFile(src, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	ppm := filepath.Join(dir, "out.ppm")
	want(t, []string{"raster", "-o", ppm, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	body := ppmBody(t, ppm)

	for _, quality := range []int{0, 75, 500} {
		out := filepath.Join(dir, fmt.Sprintf("q%d.jpg", quality))
		args := []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", "-jpegq", strconv.Itoa(quality), src}
		want(t, args, 0, "", "")
		checkJPEG(t, out, body, quality != 0)
	}

	jpegPath := filepath.Join(dir, "out.jpeg")
	want(t, []string{"raster", "-o", jpegPath, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	checkJPEG(t, jpegPath, body, true)

	wantCode(t, []string{"run", "-jpegq", "50", src}, 2)
	wantCode(t, []string{"compare", "raster", "-jpegq", "50", src, src}, 2)
}

func ppmBody(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header := []byte("P6\n20 20\n255\n")
	if !bytes.HasPrefix(raw, header) {
		t.Fatalf("ppm header %q", raw[:len(header)])
	}
	return raw[len(header):]
}

func checkJPEG(t *testing.T, path string, body []byte, exact bool) {
	t.Helper()
	pic := decodeJPEG(t, path)
	if pic.Bounds().Dx() != 20 || pic.Bounds().Dy() != 20 {
		t.Fatalf("%s bounds %v, want 20x20", path, pic.Bounds())
	}
	if exact {
		matchJPEGBody(t, pic, body)
	}
}

func decodeJPEG(t *testing.T, path string) image.Image {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte{0xff, 0xd8}) {
		t.Fatalf("%s is not a JPEG", path)
	}
	pic, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return pic
}

func matchJPEGBody(t *testing.T, pic image.Image, body []byte) {
	t.Helper()
	for y := range 20 {
		for x := range 20 {
			red, green, blue, _ := pic.At(x, y).RGBA()
			i := (y*20 + x) * 3
			if byte(red>>8) != body[i] || byte(green>>8) != body[i+1] || byte(blue>>8) != body[i+2] {
				t.Fatalf("jpeg pixel %d,%d", x, y)
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

func TestPDFImage(t *testing.T) {
	t.Run("usage", checkPDFImageUsage)
	t.Run("ps", checkPDFImagePS)
	t.Run("pdf", checkPDFImagePDF)
	t.Run("rewrite", checkPDFImageRewrite)
}

func checkPDFImageUsage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.pdf")
	ps := writeTemp(t, "in.ps", []byte("1 2 add"))
	missing := filepath.Join(t.TempDir(), "missing.ps")
	wantCode(t, []string{"pdfimage", ps}, 2)
	wantCode(t, []string{"pdfimage", "-o", out}, 2)
	wantCode(t, []string{"pdfimage", "-o", out, ps, ps}, 2)
	wantCode(t, []string{"pdfimage", "-o", out, missing}, 2)
}

func checkPDFImagePS(t *testing.T) {
	dir := t.TempDir()
	ps := filepath.Join(dir, "in.ps")
	if err := os.WriteFile(ps, []byte("0 0 moveto 10 0 lineto stroke"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.pdf")
	want(t, []string{"pdfimage", "-o", out, "-w", "20", "-h", "20", "-r", "72", ps}, 0, "", "")
	checkPDFImageOutput(t, out, 1)
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func checkPDFImagePDF(t *testing.T) {
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 5 0 R /Resources << >> >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R /Resources << >> >>"),
		flateStream(t, "0 0 m 10 0 l S"),
		flateStream(t, "1 0 0 rg 0 0 10 10 re f"),
	}
	src := writeTemp(t, "two.pdf", classicXref(t, objects))
	out := filepath.Join(t.TempDir(), "out.pdf")
	want(t, []string{"pdfimage", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	checkPDFImageOutput(t, out, 2)
}

func checkPDFImageRewrite(t *testing.T) {
	src := writeTemp(t, "path.pdf", onePagePDF(t, "0 0 m 10 0 l S"))
	out := filepath.Join(t.TempDir(), "rewrite.pdf")
	want(t, []string{"rewrite", "-o", out, src}, 0, "", "")
	checkPDFImageRaster(t, out, 1)
}

func TestPDFImageColor(t *testing.T) {
	t.Run("rgb", checkPDFImageColorRGB)
	t.Run("gray", checkPDFImageColorGray)
	t.Run("cmyk", checkPDFImageColorCMYK)
	t.Run("usage", checkPDFImageColorUsage)
}

func checkPDFImageColorRGB(t *testing.T) {
	src := writeTemp(t, "red.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.pdf")
	named := filepath.Join(dir, "rgb.pdf")
	args := []string{"pdfimage", "-o", plain, "-w", "20", "-h", "20", "-r", "72", src}
	want(t, args, 0, "", "")
	args = []string{"pdfimage", "-colorspace", "rgb", "-o", named, "-w", "20", "-h", "20", "-r", "72", src}
	want(t, args, 0, "", "")
	plainBytes := readPayload(t, plain)
	namedBytes := readPayload(t, named)
	if !bytes.Equal(plainBytes, namedBytes) {
		t.Fatal("-colorspace rgb changed the bytes")
	}
	if !bytes.Contains(plainBytes, []byte("/ColorSpace /DeviceRGB")) {
		t.Fatal("missing /DeviceRGB")
	}
}

func checkPDFImageColorGray(t *testing.T) {
	src := writeTemp(t, "red.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	out := filepath.Join(t.TempDir(), "gray.pdf")
	args := []string{"pdfimage", "-colorspace", "gray", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
	want(t, args, 0, "", "")
	payload := readPayload(t, out)
	if !bytes.Contains(payload, []byte("/ColorSpace /DeviceGray")) {
		t.Fatal("missing /DeviceGray")
	}
	streams := imageStreamBodies(t, payload)
	if len(streams) != 1 {
		t.Fatalf("image streams = %d, want 1", len(streams))
	}
	wantBytes := bytes.Repeat([]byte{76}, 400)
	if !bytes.Equal(streams[0], wantBytes) {
		t.Fatalf("gray stream = %d bytes, want %d", len(streams[0]), len(wantBytes))
	}
}

func checkPDFImageColorCMYK(t *testing.T) {
	src := writeTemp(t, "red.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	out := filepath.Join(t.TempDir(), "cmyk.pdf")
	args := []string{"pdfimage", "-colorspace", "cmyk", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
	want(t, args, 0, "", "")
	payload := readPayload(t, out)
	if !bytes.Contains(payload, []byte("/ColorSpace /DeviceCMYK")) {
		t.Fatal("missing /DeviceCMYK")
	}
	streams := imageStreamBodies(t, payload)
	if len(streams) != 1 {
		t.Fatalf("image streams = %d, want 1", len(streams))
	}
	wantBytes := bytes.Repeat([]byte{0, 255, 255, 0}, 400)
	if !bytes.Equal(streams[0], wantBytes) {
		t.Fatalf("cmyk stream = %d bytes, want %d", len(streams[0]), len(wantBytes))
	}
}

func checkPDFImageColorUsage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.pdf")
	ps := writeTemp(t, "in.ps", []byte("1 2 add"))
	wantCode(t, []string{"pdfimage", "-colorspace", "srgb", "-o", out, ps}, 2)
}

func readPayload(t *testing.T, path string) []byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func imageStreamBodies(t *testing.T, payload []byte) [][]byte {
	t.Helper()
	marker := []byte("/Subtype /Image")
	var out [][]byte
	rest := payload
	for {
		found := bytes.Index(rest, marker)
		if found < 0 {
			return out
		}
		rest = rest[found:]
		start := bytes.Index(rest, []byte("stream\n"))
		if start < 0 {
			t.Fatal("image stream start missing")
		}
		body := rest[start+len("stream\n"):]
		end := bytes.Index(body, []byte("\nendstream"))
		if end < 0 {
			t.Fatal("image stream end missing")
		}
		out = append(out, inflateStreamBytes(t, body[:end]))
		rest = body[end:]
	}
}

func inflateStreamBytes(t *testing.T, src []byte) []byte {
	t.Helper()
	reader, err := zlib.NewReader(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return plain
}

func checkPDFImageOutput(t *testing.T, path string, pages int) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(payload, []byte("%PDF-")) {
		t.Fatalf("%s prefix %q", path, pdfPrefix(payload))
	}
	doc := openPDFBytes(t, payload)
	if doc.PageCount() != pages {
		t.Fatalf("PageCount = %d, want %d", doc.PageCount(), pages)
	}
}

func checkPDFImageRaster(t *testing.T, path string, pages int) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	doc, err := in.OpenPDF(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if doc.PageCount() != pages {
		t.Fatalf("PageCount = %d, want %d", doc.PageCount(), pages)
	}
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	for page := range pages {
		if _, err := in.RasterizePage(t.Context(), doc, page, opt); err != nil {
			t.Fatal(err)
		}
	}
}

func openPDFBytes(t *testing.T, payload []byte) *spectreps.Document {
	t.Helper()
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	doc, err := in.OpenPDF(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestBBox(t *testing.T) {
	square := "0 0 moveto 10 0 lineto 10 10 lineto 0 10 lineto closepath fill"
	white := "%%BoundingBox: 0 0 0 0\n%%HiResBoundingBox: 0 0 0 0\n"
	marked := "%%BoundingBox: 0 0 10 10\n%%HiResBoundingBox: 0 0 10 10\n"

	path := writeTemp(t, "square.ps", []byte(square))
	want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", path}, 0, marked, "")
	want(t, []string{"bbox", "-w", "20", "-h", "20", path}, 0, marked, "")

	missing := filepath.Join(t.TempDir(), "missing.ps")
	wantCode(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", missing}, 2)

	blank := writeTemp(t, "blank.ps", []byte(""))
	want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", blank}, 0, white, "")

	two := writeTemp(t, "two.ps", []byte("showpage "+square+" showpage"))
	want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", two}, 0, white+marked, "")

	pdf := writeTemp(t, "square.pdf", onePagePDF(t, "0 0 m 10 0 l 10 10 l 0 10 l h f"))
	want(t, []string{"bbox", "-w", "20", "-h", "20", "-r", "72", pdf}, 0, marked, "")
}

func TestInkcov(t *testing.T) {
	square := "0 0 moveto 10 0 lineto 10 10 lineto 0 10 lineto closepath fill"
	white := "Page 1\n0.00000 0.00000 0.00000 RGB\n"
	quarter := "Page 1\n0.25000 0.25000 0.25000 RGB\n"

	path := writeTemp(t, "square.ps", []byte(square))
	want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", path}, 0, quarter, "")
	want(t, []string{"inkcov", "-w", "20", "-h", "20", path}, 0, quarter, "")

	missing := filepath.Join(t.TempDir(), "missing.ps")
	wantCode(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", missing}, 2)

	blank := writeTemp(t, "blank.ps", []byte(""))
	want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", blank}, 0, white, "")

	two := writeTemp(t, "two.ps", []byte("showpage "+square+" showpage"))
	want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", two}, 0,
		white+"Page 2\n0.25000 0.25000 0.25000 RGB\n", "")

	red := "Page 1\n0.00000 1.00000 1.00000 RGB\n"
	pdf := writeTemp(t, "red.pdf", onePagePDF(t, "1 0 0 rg 0 0 20 20 re f"))
	want(t, []string{"inkcov", "-w", "20", "-h", "20", "-r", "72", pdf}, 0, red, "")
}
