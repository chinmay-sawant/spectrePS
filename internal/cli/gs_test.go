package cli

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
	"golang.org/x/image/tiff"
)

// gsSolidPS writes a solid red 20 by 20 PostScript page.
func gsSolidPS(t *testing.T) string {
	t.Helper()
	prog := "1 0 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill"
	return writeTemp(t, "solid.ps", []byte(prog))
}

// gsThreePagePS writes the red, green, and blue fixture.
func gsThreePagePS(t *testing.T) string {
	t.Helper()
	return writeTemp(t, "three.ps", []byte(pagePSThreePages))
}

// gsPDF writes a two-page PDF, page 1 red and page 2 green.
func gsPDF(t *testing.T) string {
	t.Helper()
	return quickWinsTwoPagePDF(t, "0 1 0 rg 0 0 20 20 re f")
}

// gsDeviceArgs builds a 20 by 20 raster job for one device.
func gsDeviceArgs(device, out, src string, extra ...string) []string {
	args := []string{"gs", "-sDEVICE=" + device, "-sOutputFile=" + out}
	args = append(args, extra...)
	return append(args, "-g20x20", src)
}

// gsRasterArgs builds a 20 by 20 ppmraw job. The extras come before -g.
func gsRasterArgs(out, src string, extra ...string) []string {
	args := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out}
	args = append(args, extra...)
	return append(args, "-g20x20", src)
}

// gsPointsArgs builds a 20 by 20 job from the two point-size switches.
func gsPointsArgs(out, src string, extra ...string) []string {
	args := []string{
		"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out,
		"-dDEVICEWIDTHPOINTS=20", "-dDEVICEHEIGHTPOINTS=20",
	}
	args = append(args, extra...)
	return append(args, src)
}

// gsListArgs builds a raster command line with one -sPageList value.
func gsListArgs(out, list, src string) []string {
	return []string{
		"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-sPageList=" + list,
		"-g20x20", src,
	}
}

// gsCheckPPMHeader checks the P6 size line.
func gsCheckPPMHeader(t *testing.T, raw []byte, width, height int) {
	t.Helper()
	header := fmt.Sprintf("P6\n%d %d\n255\n", width, height)
	if len(raw) < len(header) || string(raw[:len(header)]) != header {
		t.Fatalf("ppm header %.16q, want %q", raw, header)
	}
}

// gsCheckP6 checks a 20 by 20 P6 body.
func gsCheckP6(t *testing.T, raw []byte) {
	t.Helper()
	if !bytes.HasPrefix(raw, []byte("P6\n20 20\n255\n")) {
		t.Fatalf("ppm prefix %.16q", raw)
	}
}

// gsCheckPNG decodes the bytes as a 20 by 20 PNG.
func gsCheckPNG(t *testing.T, raw []byte) {
	t.Helper()
	pic, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	if pic.Bounds().Dx() != 20 || pic.Bounds().Dy() != 20 {
		t.Fatalf("png bounds %v, want 20x20", pic.Bounds())
	}
}

// gsCheckJPEG decodes the bytes as a 20 by 20 JPEG.
func gsCheckJPEG(t *testing.T, raw []byte) {
	t.Helper()
	if !bytes.HasPrefix(raw, []byte{0xff, 0xd8}) {
		t.Fatalf("jpeg magic % x", raw[:2])
	}
	pic, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	if pic.Bounds().Dx() != 20 || pic.Bounds().Dy() != 20 {
		t.Fatalf("jpeg bounds %v, want 20x20", pic.Bounds())
	}
}

// gsCheckTIFF decodes the bytes as a 20 by 20 TIFF.
func gsCheckTIFF(t *testing.T, raw []byte) {
	t.Helper()
	if !bytes.HasPrefix(raw, []byte{0x49, 0x49, 0x2a, 0x00}) &&
		!bytes.HasPrefix(raw, []byte{0x4d, 0x4d, 0x00, 0x2a}) {
		t.Fatalf("tiff magic % x", raw[:4])
	}
	pic, err := tiff.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("tiff decode: %v", err)
	}
	if pic.Bounds().Dx() != 20 || pic.Bounds().Dy() != 20 {
		t.Fatalf("tiff bounds %v, want 20x20", pic.Bounds())
	}
}

// TestGSRejectsAll checks the reject paths of the gs scanner. Every command
// line here stays rejected after the switch families land.
func TestGSRejectsAll(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ps")
	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", []string{"gs"}},
		{"no device", []string{"gs", missing}},
		{"unknown debug switch", []string{"gs", "-Z", missing}},
		{"unknown parameter", []string{"gs", "-dFoo", missing}},
		{"unknown set name", []string{"gs", "-sFoo=1", missing}},
		{"inline code", []string{"gs", "-c", "1 2 add"}},
		{"unsafe", []string{"gs", "-dNOSAFER", missing}},
		{"delayed safer", []string{"gs", "-dDELAYSAFER", missing}},
		{"unknown device", []string{"gs", "-sDEVICE=ps2write", "-sOutputFile=out.pdf", missing}},
		{"stdin", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png", "-"}},
		{"argument file", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png", "@args"}},
		{"missing input", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := callRun(t, tc.args...)
			if code != exitUsage {
				t.Fatalf("code = %d, want %d", code, exitUsage)
			}
			if stderr == "" {
				t.Fatal("stderr is empty, want a rejection message")
			}
		})
	}
}

func TestGSDevice(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	t.Run("ppmraw", func(t *testing.T) { gsDevicePPM(t, dir, src) })
	t.Run("png16m", func(t *testing.T) { gsDevicePNG(t, dir, src) })
	t.Run("jpeg", func(t *testing.T) { gsDeviceJPEG(t, dir, src) })
	t.Run("tiff24nc", func(t *testing.T) { gsDeviceTIFF(t, dir, src) })
	t.Run("bbox", func(t *testing.T) { gsDeviceBBox(t, src) })
	t.Run("inkcov", func(t *testing.T) { gsDeviceInkCov(t, src) })
	t.Run("pdfimage24", func(t *testing.T) { gsDevicePDFImage(t, dir, src) })
	t.Run("pdfwrite", func(t *testing.T) { gsDevicePDFWrite(t, dir) })
	t.Run("rejected", func(t *testing.T) { gsDeviceRejected(t, src) })
}

func gsDevicePPM(t *testing.T, dir, src string) {
	t.Helper()
	out := filepath.Join(dir, "ppmraw.dat")
	want(t, gsDeviceArgs("ppmraw", out, src), 0, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 20, 20)
}

func gsDevicePNG(t *testing.T, dir, src string) {
	t.Helper()
	out := filepath.Join(dir, "png16m.dat")
	want(t, gsDeviceArgs("png16m", out, src), 0, "", "")
	gsCheckPNG(t, readPayload(t, out))
}

func gsDeviceJPEG(t *testing.T, dir, src string) {
	t.Helper()
	out := filepath.Join(dir, "jpeg.dat")
	want(t, gsDeviceArgs("jpeg", out, src), 0, "", "")
	gsCheckJPEG(t, readPayload(t, out))

	low := filepath.Join(dir, "jpeg-low.jpg")
	high := filepath.Join(dir, "jpeg-high.jpg")
	want(t, gsDeviceArgs("jpeg", low, src, "-dJPEGQ=20"), 0, "", "")
	want(t, gsDeviceArgs("jpeg", high, src, "-dJPEGQ=90"), 0, "", "")
	if bytes.Equal(readPayload(t, low), readPayload(t, high)) {
		t.Fatal("-dJPEGQ did not change the JPEG bytes")
	}
}

func gsDeviceTIFF(t *testing.T, dir, src string) {
	t.Helper()
	out := filepath.Join(dir, "tiff24nc.dat")
	want(t, gsDeviceArgs("tiff24nc", out, src), 0, "", "")
	gsCheckTIFF(t, readPayload(t, out))
}

func gsDeviceBBox(t *testing.T, src string) {
	t.Helper()
	want(t, []string{"gs", "-sDEVICE=bbox", src}, 0,
		"%%BoundingBox: 0 0 20 20\n%%HiResBoundingBox: 0 0 20 20\n", "")
}

func gsDeviceInkCov(t *testing.T, src string) {
	t.Helper()
	args := []string{"gs", "-sDEVICE=inkcov", "-g20x20", src}
	want(t, args, 0, "Page 1\n0.00000 1.00000 1.00000 RGB\n", "")
}

func gsDevicePDFImage(t *testing.T, dir, src string) {
	t.Helper()
	out := filepath.Join(dir, "pdfimage24.pdf")
	want(t, gsDeviceArgs("pdfimage24", out, src), 0, "", "")
	checkPDFImageOutput(t, out, 1)
}

func gsDevicePDFWrite(t *testing.T, dir string) {
	t.Helper()
	src := gsPDF(t)
	out := filepath.Join(dir, "pdfwrite.pdf")
	args := []string{"gs", "-q", "-dBATCH", "-dNOPAUSE", "-sDEVICE=pdfwrite", "-sOutputFile=" + out, src}
	want(t, args, 0, "", "")
	checkPDFImageOutput(t, out, 2)
}

func gsDeviceRejected(t *testing.T, src string) {
	t.Helper()
	for _, name := range []string{"pnggray", "ps2write", "txtwrite", "pam", "tiff32nc"} {
		args := []string{"gs", "-sDEVICE=" + name, "-sOutputFile=out.dat", src}
		code, _, stderr := callRun(t, args...)
		if code != exitUsage || !strings.Contains(stderr, "-sDEVICE="+name) {
			t.Fatalf("%s: code=%d stderr=%q", name, code, stderr)
		}
	}
	quality := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=out.dat", "-dJPEGQ=50", src}
	want(t, quality, exitUsage, "", "spectreps: -dJPEGQ needs -sDEVICE=jpeg\n")
}

func TestRasterFormatFlag(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	cases := []struct {
		name   string
		format string
		suffix string
		check  func(*testing.T, []byte)
	}{
		{"png over ppm", "png", ".ppm", gsCheckPNG},
		{"ppm over png", "ppm", ".png", gsCheckP6},
		{"jpeg over ppm", "jpeg", ".ppm", gsCheckJPEG},
		{"tiff over ppm", "tiff", ".ppm", gsCheckTIFF},
		{"ppm over tiff", "ppm", ".tiff", gsCheckP6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(dir, "format-"+tc.format+tc.suffix)
			args := []string{"raster", "-format", tc.format, "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
			want(t, args, 0, "", "")
			tc.check(t, readPayload(t, out))
		})
	}
	t.Run("rejected", func(t *testing.T) {
		out := filepath.Join(dir, "bad.ppm")
		args := []string{"raster", "-format", "bmp", "-o", out, "-w", "20", "-h", "20", "-r", "72", src}
		want(t, args, exitUsage, "", "spectreps: -format wants ppm, png, jpeg, or tiff, got \"bmp\"\n")
		pageNoFile(t, out)
	})
	t.Run("device wins over suffix", func(t *testing.T) {
		out := filepath.Join(dir, "device.ppm")
		want(t, gsDeviceArgs("png16m", out, src), 0, "", "")
		gsCheckPNG(t, readPayload(t, out))
	})
}

func TestGSOutputFile(t *testing.T) {
	t.Run("pages", gsOutputPages)
	t.Run("rejected forms", gsOutputRejected)
	t.Run("bbox output", gsOutputBBox)
	t.Run("single page", gsOutputSingle)
	t.Run("pdfwrite percent", gsOutputPDFWrite)
	t.Run("missing and multi", gsOutputMissing)
}

func gsOutputPages(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	pattern := filepath.Join(dir, "page-%d.ppm")
	want(t, gsRasterArgs(pattern, src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-2.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "page-3.ppm")), 0, 0, 255)
	pageNoFile(t, filepath.Join(dir, "page-4.ppm"))
}

func gsOutputRejected(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	cases := []struct {
		value   string
		message string
	}{
		{"-", "spectreps: -sOutputFile=- is a stream, not a file path\n"},
		{"%stdout", "spectreps: -sOutputFile=%stdout is a stream, not a file path\n"},
		{"%pipe%cat", "spectreps: -sOutputFile=%pipe%cat is a stream, not a file path\n"},
		{"page-%03d.ppm", "spectreps: -sOutputFile wants only %d in a path, got \"page-%03d.ppm\"\n"},
	}
	for _, tc := range cases {
		args := gsRasterArgs(tc.value, src)
		want(t, args, exitUsage, "", tc.message)
	}
}

func gsOutputBBox(t *testing.T) {
	t.Helper()
	src := gsSolidPS(t)
	out := filepath.Join(t.TempDir(), "bbox.txt")
	args := []string{"gs", "-sDEVICE=bbox", "-sOutputFile=" + out, src}
	want(t, args, exitUsage, "", "spectreps: -sOutputFile has no place on -sDEVICE=bbox\n")
	pageNoFile(t, out)
}

func gsOutputSingle(t *testing.T) {
	t.Helper()
	src := writeTemp(t, "one.ps", []byte(pagePSRed))
	dir := t.TempDir()
	out := filepath.Join(dir, "single.ppm")
	want(t, gsRasterArgs(out, src), 0, "", "")
	pageCheckFill(t, ppmBody(t, out), 255, 0, 0)
}

func gsOutputPDFWrite(t *testing.T) {
	t.Helper()
	src := gsPDF(t)
	out := filepath.Join(t.TempDir(), "out-%d.pdf")
	args := []string{"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + out, src}
	message := fmt.Sprintf("spectreps: -sOutputFile wants a plain path on -sDEVICE=pdfwrite, got %q\n", out)
	want(t, args, exitUsage, "", message)
}

func gsOutputMissing(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	code, _, stderr := callRun(t, "gs", "-sDEVICE=ppmraw", src)
	if code != exitUsage || !strings.Contains(stderr, "gs needs -sOutputFile=path for -sDEVICE=ppmraw") {
		t.Fatalf("missing output: code=%d stderr=%q", code, stderr)
	}
	out := filepath.Join(t.TempDir(), "plain.ppm")
	code, _, stderr = callRun(t, gsRasterArgs(out, src)...)
	if code != exitUsage || !strings.Contains(stderr, "multiple pages need a page number") {
		t.Fatalf("multi page: code=%d stderr=%q", code, stderr)
	}
	pageNoFile(t, out)
}

func TestGSPageRange(t *testing.T) {
	t.Run("first page", gsRangeFirst)
	t.Run("last page", gsRangeLast)
	t.Run("both pages", gsRangeBoth)
	t.Run("pdf input", gsRangePDF)
	t.Run("errors", gsRangeErrors)
}

func gsRangeFirst(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "first-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dFirstPage=2"), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "first-1.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "first-2.ppm")), 0, 0, 255)
	pageNoFile(t, filepath.Join(dir, "first-3.ppm"))
}

func gsRangeLast(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "last-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dLastPage=2"), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "last-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "last-2.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "last-3.ppm"))

	clamped := filepath.Join(dir, "clamp-%d.ppm")
	want(t, gsRasterArgs(clamped, src, "-dLastPage=9"), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "clamp-3.ppm")), 0, 0, 255)
}

func gsRangeBoth(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "both-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dFirstPage=2", "-dLastPage=3"), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "both-1.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "both-2.ppm")), 0, 0, 255)
	pageNoFile(t, filepath.Join(dir, "both-3.ppm"))
}

func gsRangePDF(t *testing.T) {
	t.Helper()
	src := gsPDF(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "pdf-%d.ppm")
	want(t, gsRasterArgs(out, src, "-dFirstPage=2"), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "pdf-1.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "pdf-2.ppm"))
}

func gsRangeErrors(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "bad-%d.ppm")
	reversed := gsRasterArgs(out, src, "-dFirstPage=2", "-dLastPage=1")
	want(t, reversed, exitUsage, "", "spectreps: -dLastPage=1 is before -dFirstPage=2\n")
	zero := gsRasterArgs(out, src, "-dFirstPage=0")
	want(t, zero, exitUsage, "", "spectreps: -dFirstPage wants a page number, got \"0\"\n")

	pdf := gsPDF(t)
	rewrite := []string{
		"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + filepath.Join(dir, "out.pdf"),
		"-dFirstPage=1", pdf,
	}
	want(t, rewrite, exitUsage, "", "spectreps: -dFirstPage has no place on -sDEVICE=pdfwrite\n")

	list := []string{
		"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + filepath.Join(dir, "list.pdf"),
		"-sPageList=1", pdf,
	}
	want(t, list, exitUsage, "", "spectreps: -sPageList has no place on -sDEVICE=pdfwrite\n")
}

func TestGSResolution(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "res.ppm")
	want(t, gsPointsArgs(out, src, "-r144"), 0, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 40, 40)

	want(t, gsPointsArgs(out, src, "-r0"), 0, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 20, 20)

	want(t, []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-r", src}, exitUsage, "",
		"spectreps: -r needs a resolution, e.g. -r300\n")
	want(t, []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-r300x300", src}, exitUsage, "",
		"spectreps: -r300x300 is not accepted, use -r300\n")
	want(t, []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-r-2", src}, exitUsage, "",
		"spectreps: -r-2 is not accepted, use -r300\n")

	pdf := gsPDF(t)
	rewrite := []string{"gs", "-sDEVICE=pdfwrite", "-sOutputFile=" + filepath.Join(dir, "out.pdf"), "-r144", pdf}
	want(t, rewrite, exitUsage, "", "spectreps: -r has no place on -sDEVICE=pdfwrite\n")
}

func TestGSPageSize(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "size.ppm")
	args := []string{
		"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out,
		"-dDEVICEWIDTHPOINTS=200", "-dDEVICEHEIGHTPOINTS=100", src,
	}
	want(t, args, 0, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 200, 100)

	args = []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-g144x72", src}
	want(t, args, 0, "", "")
	gsCheckPPMHeader(t, readPayload(t, out), 144, 72)

	args = []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-g144x72", "-r144", src}
	want(t, args, exitUsage, "", "spectreps: -g needs -r 72, got -r 144\n")
	args = []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-g144", src}
	want(t, args, exitUsage, "", "spectreps: -g wants WxH, got \"144\"\n")
	args = []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out, "-dDEVICEWIDTHPOINTS=x", src}
	want(t, args, exitUsage, "", "spectreps: -dDEVICEWIDTHPOINTS wants a positive size, got \"x\"\n")
}

func TestGSIgnoredSwitches(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	args := func(out string, extra ...string) []string {
		all := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + out}
		all = append(all, extra...)
		return append(all, "-g20x20", src)
	}
	plain := filepath.Join(dir, "plain.ppm")
	ignored := filepath.Join(dir, "ignored.ppm")
	want(t, args(plain), 0, "", "")
	want(t, args(ignored, "-q", "-dBATCH", "-dNOPAUSE", "-dSAFER", "-dFIXEDMEDIA"), 0, "", "")
	if !bytes.Equal(readPayload(t, plain), readPayload(t, ignored)) {
		t.Fatal("the ignored switches changed the output bytes")
	}
	want(t, args(plain, "-dBATCH=true", "-dNOPAUSE=1"), 0, "", "")
	want(t, args(plain, "-dSAFER=false"), exitUsage, "",
		"spectreps: -dSAFER=false would turn off always-on behavior\n")
	want(t, args(plain, "-dNOSAFER"), exitUsage, "",
		"spectreps: -dNOSAFER is rejected, SAFER is always on\n")
	want(t, args(plain, "-dDELAYSAFER"), exitUsage, "",
		"spectreps: -dDELAYSAFER is rejected, SAFER is always on\n")
}

func TestGSInputFile(t *testing.T) {
	src := gsSolidPS(t)
	dir := t.TempDir()
	base := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + filepath.Join(dir, "out.ppm"), "-g20x20"}
	want(t, append(base, src), 0, "", "")
	dashF := filepath.Join(dir, "dashf.ppm")
	args := []string{"gs", "-sDEVICE=ppmraw", "-sOutputFile=" + dashF, "-g20x20", "-f", src}
	want(t, args, 0, "", "")
	if !bytes.Equal(readPayload(t, filepath.Join(dir, "out.ppm")), readPayload(t, dashF)) {
		t.Fatal("-f changed the output bytes")
	}

	want(t, append(base, "-f"), exitUsage, "", "spectreps: -f needs an input file\n")
	code, _, stderr := callRun(t, append(base, src, src)...)
	if code != exitUsage || !strings.Contains(stderr, "gs takes one input file") {
		t.Fatalf("two inputs: code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = callRun(t, append(base, "-f", src, src)...)
	if code != exitUsage || !strings.Contains(stderr, "gs takes one input file") {
		t.Fatalf("two -f inputs: code=%d stderr=%q", code, stderr)
	}
	want(t, append(base, "-c", "1 2 add"), exitUsage, "",
		"spectreps: -c runs inline PostScript, pass a file\n")
	want(t, append(base, "-"), exitUsage, "", "spectreps: - is stdin, not accepted, pass a file\n")
	want(t, append(base, "@args"), exitUsage, "", "spectreps: @args is not accepted, pass the input path\n")

	missing := filepath.Join(dir, "missing.ps")
	wantCode(t, append(base, missing), exitUsage)
}

func TestGSParamSyntax(t *testing.T) {
	src := gsSolidPS(t)
	cases := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"device without value", []string{"gs", "-sDEVICE", src}, "spectreps: -sDEVICE needs a device name\n"},
		{"device empty", []string{"gs", "-sDEVICE=", src}, "spectreps: -sDEVICE needs a device name\n"},
		{
			"output without value",
			[]string{"gs", "-sDEVICE=ppmraw", "-sOutputFile", src},
			"spectreps: -sOutputFile needs a path\n",
		},
		{"first page without value", []string{"gs", "-dFirstPage", src}, "spectreps: -dFirstPage needs =N\n"},
		{
			"first page not a number",
			[]string{"gs", "-dFirstPage=x", src},
			"spectreps: -dFirstPage wants a page number, got \"x\"\n",
		},
		{
			"jpeg quality not a number",
			[]string{"gs", "-sDEVICE=jpeg", "-dJPEGQ=x", src},
			"spectreps: -dJPEGQ wants a number, got \"x\"\n",
		},
		{"unknown set name", []string{"gs", "-sFoo=1", src}, "spectreps: -sFoo is not in the gs allowlist\n"},
		{"unknown define", []string{"gs", "-dFoo", src}, "spectreps: -dFoo is not in the gs allowlist\n"},
		{
			"boolean word value",
			[]string{"gs", "-dNOPAUSE=maybe", src},
			"spectreps: -dNOPAUSE=maybe would turn off always-on behavior\n",
		},
		{"debug switch", []string{"gs", "-Z", src}, "spectreps: -Z is not in the gs allowlist\n"},
		{"double dash", []string{"gs", "--", src}, "spectreps: -- is not in the gs allowlist\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want(t, tc.args, exitUsage, "", tc.stderr)
		})
	}
}

func TestGSPageList(t *testing.T) {
	t.Run("single page", gsListSingle)
	t.Run("comma list", gsListCommaList)
	t.Run("mixed list", gsListMixed)
	t.Run("range", gsListRange)
	t.Run("rejected", gsListRejected)
	t.Run("combination", gsListCombination)
}

func gsListSingle(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "list-%d.ppm")
	want(t, gsListArgs(out, "2", src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-1.ppm")), 0, 255, 0)
	pageNoFile(t, filepath.Join(dir, "list-2.ppm"))
}

func gsListCommaList(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "list-%d.ppm")
	want(t, gsListArgs(out, "1,2,3", src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-1.ppm")), 255, 0, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-2.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-3.ppm")), 0, 0, 255)
}

func gsListMixed(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "list-%d.ppm")
	want(t, gsListArgs(out, "1-2,3", src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-3.ppm")), 0, 0, 255)
}

func gsListRange(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "list-%d.ppm")
	want(t, gsListArgs(out, "2-3", src), 0, "", "")
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-1.ppm")), 0, 255, 0)
	pageCheckFill(t, ppmBody(t, filepath.Join(dir, "list-2.ppm")), 0, 0, 255)
	pageNoFile(t, filepath.Join(dir, "list-3.ppm"))
}

func gsListRejected(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	out := filepath.Join(t.TempDir(), "list-%d.ppm")
	cases := []struct {
		list   string
		reason string
	}{
		{"even", "even and odd selections are not accepted"},
		{"odd", "even and odd selections are not accepted"},
		{"3-1", "ranges must run upward from page 1"},
		{"1,3", "the pages are not contiguous"},
		{"1-2,2-3", "the pages repeat or overlap"},
		{"2,2", "the pages repeat or overlap"},
		{"1-", "open ranges are not accepted, use -dFirstPage or -dLastPage"},
		{"-3", "open ranges are not accepted, use -dFirstPage or -dLastPage"},
		{"0", "pages are numbers from 1"},
		{"x", "pages are numbers from 1"},
		{"1-2-3", "use pages and ranges, for example 1-3,5"},
		{"@pages", "pages are numbers from 1"},
	}
	for _, tc := range cases {
		code, _, stderr := callRun(t, gsListArgs(out, tc.list, src)...)
		wantPrefix := "spectreps: -sPageList=" + tc.list + " is not accepted, " + tc.reason
		if code != exitUsage || !strings.HasPrefix(stderr, wantPrefix) {
			t.Fatalf("-sPageList=%s: code=%d stderr=%q, want prefix %q", tc.list, code, stderr, wantPrefix)
		}
	}
}

func gsListCombination(t *testing.T) {
	t.Helper()
	src := gsThreePagePS(t)
	out := filepath.Join(t.TempDir(), "list-%d.ppm")
	args := append(gsListArgs(out, "1", src), "-dFirstPage=1")
	want(t, args, exitUsage, "", "spectreps: -sPageList cannot be combined with -dFirstPage or -dLastPage\n")
}

// TestGSEndToEnd runs a ps2pdf shaped job against the checked-in fixture and
// opens and rasterizes the PDF it writes.
func TestGSEndToEnd(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "gs-argv-input.pdf")
	out := filepath.Join(t.TempDir(), "out.pdf")
	args := []string{
		"gs", "-q", "-dBATCH", "-dNOPAUSE",
		"-sDEVICE=pdfwrite", "-sOutputFile=" + out, fixture,
	}
	want(t, args, 0, "", "")
	payload := readPayload(t, out)
	if !bytes.HasPrefix(payload, []byte("%PDF-")) {
		t.Fatalf("output prefix %.8q", payload)
	}
	doc := openPDFBytes(t, payload)
	if doc.PageCount() != 2 {
		t.Fatalf("PageCount = %d, want 2", doc.PageCount())
	}
	in, err := spectreps.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close() })
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	colors := [][3]byte{{255, 0, 0}, {0, 255, 0}}
	for page := range doc.PageCount() {
		img, err := in.RasterizePage(t.Context(), doc, page, opt)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		gsCheckFirstPixel(t, img, colors[page])
	}
}

// gsCheckFirstPixel fails unless the first pixel is the wanted RGB triple.
func gsCheckFirstPixel(t *testing.T, img spectreps.PageImage, want [3]byte) {
	t.Helper()
	if len(img.Pixels) < 3 {
		t.Fatal("page image is empty")
	}
	if img.Pixels[0] != want[0] || img.Pixels[1] != want[1] || img.Pixels[2] != want[2] {
		t.Fatalf("first pixel = %d,%d,%d, want %d,%d,%d",
			img.Pixels[0], img.Pixels[1], img.Pixels[2], want[0], want[1], want[2])
	}
}
