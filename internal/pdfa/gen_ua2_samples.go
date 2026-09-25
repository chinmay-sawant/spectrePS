//go:build ignore

// Command gen_ua2_samples writes the sampledata/pdfua2 fixtures. Run it from
// the module root:
//
//	go run internal/pdfa/gen_ua2_samples.go
//
// The files are deterministic: no dates are written. The tagged sample passes
// pdfa.PreflightUA2; the untagged sample fails it with ua2-marked. veraPDF is
// never a build or runtime dependency.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
)

// sampleDir is the repo-relative output directory. The untagged sample is a
// deliberate negative fixture and goes under negative/, which the make check
// excludes from the veraPDF run.
const sampleDir = "sampledata/pdfua2"

func main() {
	root := rootDir()
	tagged := taggedSample()
	untagged := untaggedSample()
	writeSample(filepath.Join(root, sampleDir, "tagged-ua2.pdf"), tagged)
	writeSample(filepath.Join(root, sampleDir, "negative", "untagged.pdf"), untagged)
	report("tagged-ua2.pdf", tagged)
	report("negative/untagged.pdf", untagged)
}

// rootDir is the module root, from this file's path.
func rootDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fail("cannot locate the generator source")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// taggedSample is the fixture the UA-2 rules accept.
func taggedSample() []byte {
	packet := pdfa.UA2XMP(pdfa.UA2Metadata{
		Title: "Annual report",
		Lang:  "en-US",
		Part:  "2",
		Rev:   "2024",
	})
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> /StructTreeRoot 5 0 R " +
			"/Lang (en-US) /ViewerPreferences << /DisplayDocTitle true >> /Metadata 9 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << >> /StructParents 0 >>",
		streamBody("/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC"),
		"<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 8 0 R /ParentTreeNextKey 1 >>",
		"<< /Type /StructElem /S /Document /P 5 0 R /NS 10 0 R /K [7 0 R] >>",
		"<< /Type /StructElem /S /Figure /Alt (A figure) /P 6 0 R /Pg 3 0 R /K 0 >>",
		"<< /Nums [0 [7 0 R]] >>",
		metadataBody(string(packet)),
		"<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>",
	}
	return classicPDF(objects)
}

// untaggedSample is the fixture the ua2-marked rule refuses.
func untaggedSample() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << >> >>",
		streamBody("0 0 m 10 0 l S"),
	}
	return classicPDF(objects)
}

// classicPDF writes a classic-xref file with a 1.4 header.
func classicPDF(objects []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 1, len(objects)+1)
	for index, body := range objects {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, body)
	}
	xrefAt := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets), xrefAt)
	return buf.Bytes()
}

// metadataBody wraps the XMP packet in the stream dictionary ISO 32000-2
// requires: /Type /Metadata and /Subtype /XML. veraPDF UA-2 clause 8.11.1
// fails when either entry is missing.
func metadataBody(content string) string {
	return fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

// streamBody wraps one uncompressed stream.
func streamBody(content string) string {
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

// writeSample writes one sample, creating the directory.
func writeSample(path string, src []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		fail("write %s: %v", path, err)
	}
}

// report opens one sample and prints its UA-2 preflight outcome.
func report(name string, src []byte) {
	file, err := pdf.Open(context.Background(), src)
	if err != nil {
		fail("open %s: %v", name, err)
	}
	if err := pdfa.PreflightUA2(context.Background(), file); err != nil {
		fmt.Printf("%s: %v\n", name, err)
		return
	}
	fmt.Printf("%s: passes PreflightUA2\n", name)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen_ua2_samples: "+format+"\n", args...)
	os.Exit(1)
}
