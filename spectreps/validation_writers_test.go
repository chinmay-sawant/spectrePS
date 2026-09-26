package spectreps_test

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationRewriteTags proves a tagged source keeps its structure tree,
// MCIDs, /Alt, and parent tree at levels 1 through 5, and that level 0 refuses
// the tagged input by name.
func TestValidationRewriteTags(t *testing.T) {
	in := newInst(t)
	src, bodies := validationTaggedPDF(t)
	if plain := validationStreamText(t, bodies[3]); !bytes.Contains(plain, []byte("MCID 0")) {
		t.Fatalf("fixture content has no MCID: %q", plain)
	}
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Tagged() {
		t.Fatal("Tagged() = false for the tagged fixture")
	}
	t.Run("level 0 refuses", func(t *testing.T) {
		checkRewriteTagsRefusal(t, in, doc)
	})
	for level := 1; level <= 5; level++ {
		t.Run(fmt.Sprintf("level %d", level), func(t *testing.T) {
			checkRewriteTagsLevel(t, in, doc, bodies, level)
		})
	}
}

// checkRewriteTagsRefusal proves level 0 refuses the tagged fixture by name
// and returns no bytes.
func checkRewriteTagsRefusal(t *testing.T, in *spectreps.Instance, doc *spectreps.Document) {
	t.Helper()
	out, err := in.RewritePDF(t.Context(), doc, spectreps.DefaultRewriteOptions())
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != rewriteOp || job.Msg != taggedMsg ||
		job.Error() != "Error: /tagged in RewritePDF" {
		t.Fatalf("RewritePDF() error = %v, want Error: /tagged in RewritePDF", err)
	}
	if out != nil {
		t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
	}
}

// checkRewriteTagsLevel proves one rewrite level keeps every source object and
// the structure tree, and keeps the page count.
func checkRewriteTagsLevel(
	t *testing.T,
	in *spectreps.Instance,
	doc *spectreps.Document,
	bodies [][]byte,
	level int,
) {
	t.Helper()
	out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: level})
	if err != nil {
		t.Fatalf("level %d: %v", level, err)
	}
	for i, body := range bodies {
		if !bytes.Contains(out, body) {
			t.Fatalf("level %d dropped source object %d: %q", level, i+1, body)
		}
	}
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatalf("level %d reopen: %v", level, err)
	}
	if !reopened.Tagged() {
		t.Fatalf("level %d dropped the structure tree", level)
	}
	if reopened.PageCount() != 1 {
		t.Fatalf("level %d PageCount = %d, want 1", level, reopened.PageCount())
	}
}

// validationTaggedPDF is a one-page tagged fixture whose content stream is
// already Flate, so every level copies it without decoding. The returned
// bodies are the exact object bodies a copy must keep: the structure tree, the
// structure element with its /Alt, the parent tree, and the marked content
// stream carrying the MCID.
func validationTaggedPDF(t *testing.T) ([]byte, [][]byte) {
	t.Helper()
	content := flateStream(t, "/P <</MCID 0>> BDC\n0 0 m 10 0 l S\nEMC")
	bodies := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> " +
			"/StructTreeRoot 5 0 R /Lang (en-US) >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << >> >>"),
		content,
		[]byte("<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 7 0 R >>"),
		[]byte("<< /Type /StructElem /S /P /P 5 0 R /Pg 3 0 R /K 0 /Alt (A tagged path) >>"),
		[]byte("<< /Nums [0 [6 0 R]] >>"),
	}
	return classicXref(t, bodies), bodies
}

// TestValidationRewriteContainers proves the pass-through writer does not copy
// a source /Type /XRef or /Type /ObjStm container, and that the page count
// holds at every level.
func TestValidationRewriteContainers(t *testing.T) {
	in := newInst(t)
	src := validationReadFile(t, filepath.Join(
		"..", "sampledata", "validation", "rewrite", "object-stream.pdf",
	))
	for _, needle := range validationContainerNeedles() {
		if !bytes.Contains(src, needle) {
			t.Fatalf("source is missing %q", needle)
		}
	}
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	pages := doc.PageCount()
	if pages == 0 {
		t.Fatal("source PageCount = 0")
	}
	for level := 1; level <= 5; level++ {
		t.Run(fmt.Sprintf("level %d", level), func(t *testing.T) {
			checkRewriteContainersLevel(t, in, doc, pages, level)
		})
	}
}

// validationContainerNeedles are the dead container headers the source carries
// and a rewrite must drop.
func validationContainerNeedles() [][]byte {
	return [][]byte{[]byte("/Type /ObjStm"), []byte("/Type /XRef")}
}

// checkRewriteContainersLevel proves one rewrite level drops both dead
// containers and keeps the page count.
func checkRewriteContainersLevel(
	t *testing.T,
	in *spectreps.Instance,
	doc *spectreps.Document,
	pages, level int,
) {
	t.Helper()
	out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{Level: level})
	if err != nil {
		t.Fatalf("level %d: %v", level, err)
	}
	for _, needle := range validationContainerNeedles() {
		if bytes.Contains(out, needle) {
			t.Fatalf("level %d copied the dead container %q", level, needle)
		}
	}
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatalf("level %d reopen: %v", level, err)
	}
	if reopened.PageCount() != pages {
		t.Fatalf("level %d PageCount = %d, want %d", level, reopened.PageCount(), pages)
	}
}

// TestValidationRewritePDFA locks PDFA4 and PDFA4F combined with levels 1
// through 5: the claim header, the pdfaid metadata, the conformance letter,
// the page count, and stable bytes. The 4f source is the veraPDF embedded-file
// pass file; the two base sources are the sampledata/pdfa fixtures.
func TestValidationRewritePDFA(t *testing.T) {
	in := newInst(t)
	cases := []struct {
		name        string
		path        string
		mode        spectreps.PDFAMode
		conformance bool
	}{
		{
			name: "base path-a4",
			path: filepath.Join("..", "sampledata", "pdfa", "path-a4.pdf"),
			mode: spectreps.PDFA4,
		},
		{
			name: "base compliant-a4",
			path: filepath.Join("..", "sampledata", "pdfa", "compliant-a4.pdf"),
			mode: spectreps.PDFA4,
		},
		{
			name: "4f embedded files",
			path: filepath.Join("..", "sampledata", "validation", "pdfa", "4f-6-7-3-t01-pass-a.pdf"),
			mode: spectreps.PDFA4F, conformance: true,
		},
	}
	for _, testCase := range cases {
		src := validationReadFile(t, testCase.path)
		doc, err := in.OpenPDF(t.Context(), src)
		if err != nil {
			t.Fatalf("%s open: %v", testCase.name, err)
		}
		pages := doc.PageCount()
		for level := 1; level <= 5; level++ {
			t.Run(fmt.Sprintf("%s level %d", testCase.name, level), func(t *testing.T) {
				opt := spectreps.RewriteOptions{Level: level, PDFA: testCase.mode}
				out := rewritePair(t, in, doc, opt)
				wantPDFA4Payload(t, out, testCase.conformance)
				reopened, err := in.OpenPDF(t.Context(), out)
				if err != nil {
					t.Fatalf("%s level %d reopen: %v", testCase.name, level, err)
				}
				if reopened.PageCount() != pages {
					t.Fatalf("%s level %d PageCount = %d, want %d",
						testCase.name, level, reopened.PageCount(), pages)
				}
			})
		}
	}
}

// TestValidationWritePostScript locks the multi-page program shape and the
// image refusal through the public API.
func TestValidationWritePostScript(t *testing.T) {
	in := newInst(t)
	t.Run("multi page", func(t *testing.T) {
		checkWritePostScriptMultiPage(t, in)
	})
	t.Run("image refused", func(t *testing.T) {
		checkWritePostScriptImageRefused(t, in)
	})
}

// checkWritePostScriptMultiPage locks the program shape for a two-page
// document and the round trip back through the interpreter.
func checkWritePostScriptMultiPage(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), validationTwoPagePDF(t))
	if err != nil {
		t.Fatal(err)
	}
	program, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareFiles(program, second); !compared.Equal {
		t.Fatalf("CompareFiles = %+v", compared)
	}
	if !bytes.HasPrefix(program, []byte("%!PS-Adobe-3.0")) {
		t.Fatalf("program prefix = %q, want %%!PS-Adobe-3.0", program)
	}
	if !bytes.Contains(program, []byte("%%Pages: 2")) {
		t.Fatalf("program has no page count: %q", program)
	}
	if got := bytes.Count(program, []byte("%%Page: ")); got != 2 {
		t.Fatalf("%%Page count = %d, want 2", got)
	}
	if got := bytes.Count(program, []byte("showpage")); got != 2 {
		t.Fatalf("showpage count = %d, want 2", got)
	}
	checkPostScriptRoundTrip(t, in, doc, program)
}

// checkWritePostScriptImageRefused locks the image refusal through the public
// API: undefined in Do and no output bytes.
func checkWritePostScriptImageRefused(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	doc, err := in.OpenPDF(t.Context(), validationImagePagePDF(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != "Do" || job.Msg != undefinedMsg ||
		job.Error() != "Error: /undefined in Do" {
		t.Fatalf("WritePostScript() error = %v, want Error: /undefined in Do", err)
	}
	if out != nil {
		t.Fatalf("WritePostScript() bytes = %#v, want nil", out)
	}
}

// validationTwoPagePDF is two path pages: a stroke and a fill.
func validationTwoPagePDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>"),
		plainStream(t, "0 0 m 10 0 l S"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R /Resources << >> >>"),
		plainStream(t, "1 0 0 rg 0 0 10 10 re f"),
	}
	return classicXref(t, objects)
}

// validationImagePagePDF is one DCT image page, which the PostScript writer
// cannot emit.
func validationImagePagePDF(t *testing.T) []byte {
	t.Helper()
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << /XObject << /Im0 5 0 R >> >> >>"),
		plainStream(t, "q 20 0 0 20 0 0 cm /Im0 Do Q"),
		dctImageObject(t, 8, 8),
	}
	return classicXref(t, objects)
}

// checkPostScriptRoundTrip runs the program back through the interpreter and
// compares each page with the source document.
func checkPostScriptRoundTrip(
	t *testing.T,
	in *spectreps.Instance,
	doc *spectreps.Document,
	program []byte,
) {
	t.Helper()
	opt := spectreps.RunOptions{PageWidthPt: 20, PageHeightPt: 20, ResolutionDPI: 72}
	pages, err := in.RunPostScript(t.Context(), program, opt)
	if err != nil {
		t.Fatalf("RunPostScript() = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("RunPostScript() pages = %d, want 2", len(pages))
	}
	for page := range pages {
		before, err := in.RasterizePage(t.Context(), doc, page, opt)
		if err != nil {
			t.Fatalf("page %d raster: %v", page, err)
		}
		if compared := spectreps.CompareRaster(before, pages[page]); !compared.Equal {
			t.Fatalf("page %d CompareRaster = %+v", page, compared)
		}
	}
}

// TestValidationImagePDFContract locks the public bitmap PDF surface: CMYK
// bytes, the dpi floor, an empty page slice, an unknown color, and the
// MediaBox formula.
func TestValidationImagePDFContract(t *testing.T) {
	in := newInst(t)
	t.Run("cmyk bytes", func(t *testing.T) {
		checkImagePDFCMYKBytes(t, in)
	})
	t.Run("dpi floor", func(t *testing.T) {
		checkImagePDFDPIFloor(t, in)
	})
	t.Run("empty pages", func(t *testing.T) {
		checkImagePDFEmptyPages(t, in)
	})
	t.Run("unknown color", func(t *testing.T) {
		checkImagePDFUnknownColor(t, in)
	})
	t.Run("media box formula", func(t *testing.T) {
		checkImagePDFMediaBox(t, in)
	})
}

// checkImagePDFCMYKBytes proves ImagePDFColor stores CMYK samples inverted and
// names the device.
func checkImagePDFCMYKBytes(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	pages := []spectreps.PageImage{{
		Width: 2, Height: 1, Stride: 6,
		Pixels: []byte{255, 0, 0, 255, 255, 255},
	}}
	out, err := in.ImagePDFColor(t.Context(), pages, 72, spectreps.ImageColorCMYK)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("/DeviceCMYK")) {
		t.Fatal("output is missing /DeviceCMYK")
	}
	got := inflatePageImage(t, out)
	want := []byte{0, 255, 255, 0, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("CMYK stream = %v, want %v", got, want)
	}
}

// checkImagePDFDPIFloor proves a dpi at or below zero falls back to 72.
func checkImagePDFDPIFloor(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	pages := []spectreps.PageImage{validationPageImage(2, 1)}
	want := imagePDFBytes(t, in, pages)
	for _, dpi := range []float64{0, -4} {
		got, err := in.ImagePDF(t.Context(), pages, dpi)
		if err != nil {
			t.Fatal(err)
		}
		if compared := spectreps.CompareFiles(want, got); !compared.Equal {
			t.Fatalf("dpi %v bytes differ: %+v", dpi, compared)
		}
		if !bytes.Contains(got, []byte("/MediaBox [0 0 2 1]")) {
			t.Fatalf("dpi %v has no 2 by 1 MediaBox", dpi)
		}
	}
}

// checkImagePDFEmptyPages proves a nil and an empty page slice both produce a
// readable zero-page PDF.
func checkImagePDFEmptyPages(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	for _, pages := range [][]spectreps.PageImage{nil, {}} {
		out, err := in.ImagePDF(t.Context(), pages, 72)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
			t.Fatalf("empty output prefix = %q", out[:min(8, len(out))])
		}
		if !bytes.Contains(out, []byte("/Count 0")) {
			t.Fatal("empty output has no /Count 0")
		}
		doc, err := in.OpenPDF(t.Context(), out)
		if err != nil {
			t.Fatal(err)
		}
		if doc.PageCount() != 0 {
			t.Fatalf("empty PageCount = %d, want 0", doc.PageCount())
		}
	}
}

// checkImagePDFUnknownColor proves an unknown color falls back to the RGB
// bytes.
func checkImagePDFUnknownColor(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	pages := []spectreps.PageImage{validationPageImage(3, 2)}
	fallback, err := in.ImagePDFColor(t.Context(), pages, 72, spectreps.ImageColor(99))
	if err != nil {
		t.Fatal(err)
	}
	if compared := spectreps.CompareFiles(imagePDFBytes(t, in, pages), fallback); !compared.Equal {
		t.Fatalf("unknown color bytes differ: %+v", compared)
	}
	if !bytes.Contains(fallback, []byte("/DeviceRGB")) {
		t.Fatal("unknown color did not select /DeviceRGB")
	}
}

// checkImagePDFMediaBox proves the MediaBox formula is pixels times 72 over
// dpi.
func checkImagePDFMediaBox(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	out, err := in.ImagePDFColor(t.Context(), []spectreps.PageImage{
		validationPageImage(3, 2),
	}, 96, spectreps.ImageColorRGB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("/MediaBox [0 0 2.25 1.5]")) {
		t.Fatalf("MediaBox = pixels*72/dpi missing: %q", out)
	}
}

// validationPageImage is one width by height RGB page with a tight stride.
func validationPageImage(width, height int) spectreps.PageImage {
	stride := width * 3
	pixels := make([]byte, stride*height)
	for y := range height {
		for x := range width {
			at := y*stride + x*3
			pixels[at] = byte(x * 17)
			pixels[at+1] = byte(y * 23)
			pixels[at+2] = byte((x + y) * 11)
		}
	}
	return spectreps.PageImage{Width: width, Height: height, Stride: stride, Pixels: pixels}
}

// validationReadFile reads one committed fixture. A missing fixture fails the
// test: the corpus is checked in.
func validationReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

// validationStreamText inflates one Flate stream body built by flateStream.
func validationStreamText(t *testing.T, body []byte) []byte {
	t.Helper()
	head := bytes.Index(body, []byte("\nstream\n"))
	if head < 0 {
		t.Fatalf("stream body has no stream marker: %q", body)
	}
	rest := body[head+len("\nstream\n"):]
	tail := bytes.Index(rest, []byte("\nendstream"))
	if tail < 0 {
		t.Fatalf("stream body has no endstream: %q", body)
	}
	reader, err := zlib.NewReader(bytes.NewReader(rest[:tail]))
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
