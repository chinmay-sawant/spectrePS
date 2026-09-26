package tag

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The package had no benchmark before this file. DerivePlan is the only
// superlinear loop on a public job: buildBlocks is O(lines * blocks) at
// reading.go:612 and orderFlow is O(n^2) with a slice shift per removal at
// reading.go:897. The tagged write is a RewritePDF writer, so this cost is on
// a product path.

// benchTagFile builds a one page fixture and opens it. It reuses the document
// writer and the kidRefs helper from fixture_test.go, which take no test
// handle, so the benchmark input matches what the reader tests open.
func benchTagFile(b *testing.B, pages int) *pdf.File {
	b.Helper()
	doc := newTagDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kidRefs(pages), pages))
	for range pages {
		contentNum := len(doc.offsets) + 1
		doc.object(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "+
			"/Contents %d 0 R /Resources << >> >>", contentNum))
		doc.object("<< /Length 0 >>\nstream\n\nendstream")
	}
	file, err := pdf.Open(b.Context(), doc.classic())
	if err != nil {
		b.Fatal(err)
	}
	return file
}

// benchRuns builds one recorder holding count text runs in a single column, so
// the reader sees one text block and orderFlow has count elements to order.
func benchRuns(count int) *Recorder {
	rec := NewRecorder()
	for i := range count {
		recordRun(rec, fmt.Sprintf("line %d of the benchmark page", i), 20, float64(750-i*8), 11)
	}
	return rec
}

// BenchmarkDerivePlan derives a plan from 50, 200, and 800 recorded text runs
// on one page. The slope across the three lines is the cost of orderFlow, and
// it is the reason the run count is the variable here.
func BenchmarkDerivePlan(b *testing.B) {
	for _, count := range []int{50, 200, 800} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			rec := benchRuns(count)
			b.SetBytes(int64(count))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := DerivePlan([]*Recorder{rec}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTaggedBuild runs the whole tagged write, which includes the
// pdfa.PreflightUA2 call the builder makes on its own output.
func BenchmarkTaggedBuild(b *testing.B) {
	file := benchTagFile(b, 1)
	rec := benchRuns(20)
	plan, err := DerivePlan([]*Recorder{rec})
	if err != nil {
		b.Fatal(err)
	}
	opt := Options{Title: "Benchmark document", Lang: "en-US"}
	b.SetBytes(int64(len(rec.Content())))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Build(b.Context(), file, []*Recorder{rec}, plan, opt); err != nil {
			b.Fatal(err)
		}
	}
}
