package spectreps_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestPDFPathMatchesPS(t *testing.T) {
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	cases := []struct {
		name    string
		content string
		program string
	}{
		{
			name:    "stroke",
			content: "0 0 m 10 0 l S",
			program: "0 0 moveto 10 0 lineto stroke",
		},
		{
			name:    "fill",
			content: "1 0 0 rg 0 0 10 10 re f",
			program: "1 0 0 setrgbcolor 0 0 moveto 10 0 lineto 10 10 lineto 0 10 lineto closepath fill",
		},
	}
	in := newInst(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertSamePixels(t, in, opt, tt.content, tt.program)
		})
	}
}

func TestRasterizePage(t *testing.T) {
	in := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, "0 0 m 10 0 l S"))
	if errors.Is(err, spectreps.ErrNotImplemented) || err != nil {
		t.Fatalf("OpenPDF() error = %v", err)
	}
	if _, err := in.RasterizePage(t.Context(), doc, 0, opt); err != nil {
		t.Fatal(err)
	}
	for _, pageIndex := range []int{-1, 1} {
		img, pageErr := in.RasterizePage(t.Context(), doc, pageIndex, opt)
		var job spectreps.JobError
		if !errors.As(pageErr, &job) || job.Msg != "rangecheck" {
			t.Fatalf("page %d error = %v", pageIndex, pageErr)
		}
		requireZeroPageImage(t, img)
	}
}

func TestRewritePixels(t *testing.T) {
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	cases := []struct {
		name    string
		content string
	}{
		{name: "stroke", content: "0 0 m 10 0 l S"},
		{name: "fill", content: "1 0 0 rg 0 0 10 10 re f"},
		{name: "gray", content: "0.5 g 0 0 8 8 re f"},
		{name: "eofill", content: "1 0 0 RG 0 0 10 10 re f*"},
		{name: "curve", content: "0 0 m 0 10 10 10 10 0 c S"},
		{name: "gsave", content: "q 0 1 0 rg 0 0 10 10 re f Q"},
		{name: "grestore", content: "1 0 0 rg q 0 1 0 rg Q 0 0 10 10 re f"},
		{name: "empty", content: ""},
	}
	in := newInst(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertRewritePixels(t, in, opt, tt.content, spectreps.DefaultRewriteOptions())
		})
	}
	t.Run("stroke-144", func(t *testing.T) {
		opt144 := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 144}
		assertRewritePixels(t, in, opt144, "0 0 m 10 0 l S", spectreps.DefaultRewriteOptions())
	})
	t.Run("stroke-raw", func(t *testing.T) {
		raw := spectreps.RewriteOptions{CompressStreams: false}
		assertRewritePixels(t, in, opt, "0 0 m 10 0 l S", raw)
	})
	t.Run("Tj", func(t *testing.T) {
		assertRewriteTj(t, in)
	})
}

func TestCTM(t *testing.T) {
	in := newInst(t)
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	cases := []struct {
		name    string
		content string
		black   [2]int
		white   [2]int
	}{
		{
			name:    "translate",
			content: "1 0 0 1 10 0 cm 0 0 m 10 0 l S",
			black:   [2]int{12, 0},
			white:   [2]int{8, 0},
		},
		{
			name:    "scale",
			content: "2 0 0 2 0 0 cm 0 0 m 5 0 l S",
			black:   [2]int{5, 0},
			white:   [2]int{12, 0},
		},
		{
			name:    "stroke-width",
			content: "4 0 0 4 0 0 cm 0 0 m 5 0 l S",
			black:   [2]int{5, 1},
			white:   [2]int{5, 2},
		},
		{
			name:    "restore",
			content: "q 1 0 0 1 10 0 cm Q 0 0 m 10 0 l S",
			black:   [2]int{5, 0},
			white:   [2]int{15, 0},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := in.OpenPDF(t.Context(), onePagePDF(t, tt.content))
			if err != nil {
				t.Fatal(err)
			}
			img, err := in.RasterizePage(t.Context(), doc, 0, opt)
			if err != nil {
				t.Fatal(err)
			}
			if !pixelBlack(img, tt.black[0], tt.black[1]) {
				t.Fatalf("pixel %v is not black", tt.black)
			}
			if !ctmWhitePixel(img, tt.white[0], tt.white[1]) {
				t.Fatalf("pixel %v is not white", tt.white)
			}
			assertRewritePixels(t, in, opt, tt.content, spectreps.DefaultRewriteOptions())
		})
	}
}

func ctmWhitePixel(img spectreps.PageImage, x, yFromBottom int) bool {
	row := img.Height - 1 - yFromBottom
	i := row*img.Stride + x*3
	return img.Pixels[i] == 255 && img.Pixels[i+1] == 255 && img.Pixels[i+2] == 255
}

func TestRewriteStable(t *testing.T) {
	in := newInst(t)
	src := onePagePDF(t, "0 0 m 10 0 l S")
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("OpenPDF() document = nil")
	}
	compressed := rewritePair(t, in, doc, spectreps.DefaultRewriteOptions())
	plain := rewritePair(t, in, doc, spectreps.RewriteOptions{CompressStreams: false})
	if spectreps.CompareFiles(compressed, plain).Equal {
		t.Fatal("compressed buffer matches uncompressed")
	}
	if spectreps.CompareFiles(compressed, src).Equal {
		t.Fatal("compressed rewrite matches input")
	}
	if spectreps.CompareFiles(plain, src).Equal {
		t.Fatal("uncompressed rewrite matches input")
	}
	if bytes.Contains(compressed, []byte("CreationDate")) || bytes.Contains(compressed, []byte("ModDate")) {
		t.Fatal("compressed buffer contains a date")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := in.RewritePDF(ctx, doc, spectreps.DefaultRewriteOptions())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RewritePDF() error = %v, want context.Canceled", err)
	}
	if out != nil {
		t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
	}
}

func assertRewritePixels(
	t *testing.T,
	in *spectreps.Instance,
	opt spectreps.RunOptions,
	content string,
	rewrite spectreps.RewriteOptions,
) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, content))
	if err != nil {
		t.Fatal(err)
	}
	before, err := in.RasterizePage(t.Context(), doc, 0, opt)
	if err != nil {
		t.Fatal(err)
	}
	rewritten, err := in.RewritePDF(t.Context(), doc, rewrite)
	if err != nil {
		t.Fatal(err)
	}
	again, err := in.OpenPDF(t.Context(), rewritten)
	if err != nil {
		t.Fatal(err)
	}
	after, err := in.RasterizePage(t.Context(), again, 0, opt)
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareRaster(before, after); !compared.Equal {
		t.Fatalf("CompareRaster = %+v", compared)
	}
}

func assertRewriteTj(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, "(Hi) Tj"))
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("OpenPDF() document = nil")
	}
	out, err := in.RewritePDF(t.Context(), doc, spectreps.DefaultRewriteOptions())
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != "undefined" || job.Op != "Tj" {
		t.Fatalf("RewritePDF() error = %v, want undefined in Tj", err)
	}
	if out != nil {
		t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
	}
}

func rewritePair(
	t *testing.T,
	in *spectreps.Instance,
	doc *spectreps.Document,
	opt spectreps.RewriteOptions,
) []byte {
	t.Helper()
	first, err := in.RewritePDF(t.Context(), doc, opt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := in.RewritePDF(t.Context(), doc, opt)
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareFiles(first, second); !compared.Equal {
		t.Fatalf("CompareFiles = %+v", compared)
	}
	return first
}

func assertSamePixels(
	t *testing.T,
	in *spectreps.Instance,
	opt spectreps.RunOptions,
	content string,
	program string,
) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), onePagePDF(t, content))
	if err != nil {
		t.Fatal(err)
	}
	got, err := in.RasterizePage(t.Context(), doc, 0, opt)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := in.RunPostScript(t.Context(), []byte(program), opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d", len(pages))
	}
	if compared := spectreps.CompareRaster(got, pages[0]); !compared.Equal {
		t.Fatalf("CompareRaster = %+v", compared)
	}
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
