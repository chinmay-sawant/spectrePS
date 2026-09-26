package spectreps_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// rewriteOp is the RewritePDF job name. One test constant keeps the literal
// from spreading.
const rewriteOp = "RewritePDF"

// TestRewriteTags proves the tag switch builds a tagged file, the claim
// opt-in writes pdfuaid only after a passing preflight, a missing title keeps
// the tree and writes no claim, and a tagged input is refused.
func TestRewriteTags(t *testing.T) {
	in := newInst(t)
	src := onePagePDF(t, "0 0 m 10 0 l S")
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("positive", func(t *testing.T) {
		opt := spectreps.RewriteOptions{
			Tag: true, Claim: true, Title: "Annual report", Lang: "en-US",
		}
		out := rewritePair(t, in, doc, opt)
		checkTaggedWrite(t, in, out)
	})
	t.Run("no title", func(t *testing.T) {
		checkNoTitleRefusal(t, in, doc)
	})
	t.Run("tagged input", func(t *testing.T) {
		checkTaggedInputRefusal(t, in)
	})
	t.Run("pdfa combination", func(t *testing.T) {
		checkPDFACombination(t, in, doc)
	})
}

// checkTaggedWrite proves one tagged write carries the tree, the artifact
// wrap, and the claim, and passes the UA-2 preflight on reopen.
func checkTaggedWrite(t *testing.T, in *spectreps.Instance, out []byte) {
	t.Helper()
	if !bytes.Contains(out, []byte("/StructTreeRoot")) {
		t.Fatal("output lacks /StructTreeRoot")
	}
	if !bytes.Contains(out, []byte("/Artifact BMC")) {
		t.Fatal("output lacks the artifact wrap")
	}
	if !bytes.Contains(out, []byte("<pdfuaid:part>2</pdfuaid:part>")) {
		t.Fatal("output lacks pdfuaid:part 2")
	}
	if !bytes.Contains(out, []byte("<pdfuaid:rev>2024</pdfuaid:rev>")) {
		t.Fatal("output lacks pdfuaid:rev 2024")
	}
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Tagged() {
		t.Fatal("Tagged() = false after the tagged write")
	}
	if err := in.PreflightUA2(t.Context(), reopened); err != nil {
		t.Fatalf("PreflightUA2() = %v", err)
	}
}

// checkNoTitleRefusal proves a claim with no title keeps the tree and writes
// no claim.
func checkNoTitleRefusal(t *testing.T, in *spectreps.Instance, doc *spectreps.Document) {
	t.Helper()
	opt := spectreps.RewriteOptions{Tag: true, Claim: true, Lang: "en-US"}
	out, err := in.RewritePDF(t.Context(), doc, opt)
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != "PDFUA" || job.Msg != "ua2-title" {
		t.Fatalf("RewritePDF() error = %v, want ua2-title in PDFUA", err)
	}
	if out == nil {
		t.Fatal("refusal dropped the tree")
	}
	if bytes.Contains(out, []byte("<pdfuaid:part>")) {
		t.Fatal("refusal wrote a claim")
	}
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Tagged() {
		t.Fatal("refusal dropped the tagged bytes")
	}
}

// checkTaggedInputRefusal proves the tag switch refuses a tagged input.
func checkTaggedInputRefusal(t *testing.T, in *spectreps.Instance) {
	t.Helper()
	tagged, err := in.OpenPDF(t.Context(), taggedPDF(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := in.RewritePDF(t.Context(), tagged, spectreps.RewriteOptions{Tag: true})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != rewriteOp || job.Msg != taggedMsg {
		t.Fatalf("RewritePDF() error = %v, want tagged in RewritePDF", err)
	}
	if out != nil {
		t.Fatalf("bytes = %#v, want nil", out)
	}
}

// checkPDFACombination proves the two claims are refused together.
func checkPDFACombination(t *testing.T, in *spectreps.Instance, doc *spectreps.Document) {
	t.Helper()
	out, err := in.RewritePDF(t.Context(), doc, spectreps.RewriteOptions{
		Tag: true, PDFA: spectreps.PDFA4,
	})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Op != rewriteOp || job.Msg != "unsupported" {
		t.Fatalf("RewritePDF() error = %v, want unsupported in RewritePDF", err)
	}
	if out != nil {
		t.Fatalf("bytes = %#v, want nil", out)
	}
}

// TestTagRoundTrip proves a generated file reopens, rasterizes with marked
// content skipped, extracts its text, and stays byte-equal across two runs.
// The translated case proves the recorder reproduces device placement
// through cm, trap 2.
func TestTagRoundTrip(t *testing.T) {
	in := newInst(t)
	cases := []struct {
		name    string
		content string
		text    string
	}{
		{
			name:    "plain",
			content: "BT /F1 12 Tf 10 20 Td (AB) Tj ET",
			text:    "AB\r\n",
		},
		{
			name:    "translated",
			content: "1 0 0 1 50 30 cm BT /F1 12 Tf 0 0 Td (AB) Tj ET",
			text:    "AB\r\n",
		},
		{
			name: "translated image",
			content: "BT /F1 12 Tf 10 20 Td (AB) Tj ET\n" +
				"q 40 0 0 40 20 10 cm /Im0 Do Q",
			text: "AB\r\n",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			checkRoundTripCase(t, in, tt.content, tt.text)
		})
	}
}

// checkRoundTripCase tags one fixture, compares the source and generated
// rasters, and checks the extracted text.
func checkRoundTripCase(t *testing.T, in *spectreps.Instance, content, wantText string) {
	t.Helper()
	src := type1ContentPDF(t, content)
	doc, err := in.OpenPDF(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	opt := spectreps.RewriteOptions{
		Tag: true, Claim: true, Title: "Round trip", Lang: "en-US",
	}
	out := rewritePair(t, in, doc, opt)
	reopened, err := in.OpenPDF(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Tagged() {
		t.Fatal("Tagged() = false")
	}
	rasterOpt := spectreps.RunOptions{PageWidthPt: 200, PageHeightPt: 100, ResolutionDPI: 72}
	before, err := in.RasterizePage(t.Context(), doc, 0, rasterOpt)
	if err != nil {
		t.Fatal(err)
	}
	after, err := in.RasterizePage(t.Context(), reopened, 0, rasterOpt)
	if err != nil {
		t.Fatalf("RasterizePage() = %v", err)
	}
	if compared := spectreps.CompareRaster(before, after); !compared.Equal {
		t.Fatalf("CompareRaster = %+v", compared)
	}
	text, err := in.ExtractText(t.Context(), reopened, 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != wantText {
		t.Fatalf("ExtractText() = %q, want %q", text, wantText)
	}
}

// type1ContentPDF embeds the synthetic Type 1 font and one small image and
// runs one content stream, so the generated file rasterizes as well as
// extracts.
func type1ContentPDF(t *testing.T, content string) []byte {
	t.Helper()
	program, lengths := type1synth.Program(type1synth.Options{LenIV: 4})
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> /XObject << /Im0 8 0 R >> >> >>"),
		flateStream(t, content),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /SynthType1 /FontDescriptor 6 0 R >>"),
		[]byte("<< /Type /FontDescriptor /FontName /SynthType1 /Flags 4 /FontFile 7 0 R >>"),
		type1FontObject(t, program, lengths),
		taggedImageObject(t),
	}
	return classicXref(t, objects)
}

// taggedImageObject is one four by four RGB Flate image with an /Alt entry,
// which the tag builder reads before it decodes.
func taggedImageObject(t *testing.T) []byte {
	t.Helper()
	pixels := make([]byte, 4*4*3)
	for index := range pixels {
		pixels[index] = byte(index * 5)
	}
	compressed := flateBytes(t, pixels)
	head := fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 4 /Height 4 "+
		"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode "+
		"/Alt (A small chart) /Length %d >>\nstream\n", len(compressed))
	out := append([]byte(head), compressed...)
	return append(out, []byte("\nendstream")...)
}
