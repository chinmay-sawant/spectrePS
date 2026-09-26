package psout

import (
	"bytes"
	"fmt"
	"testing"
)

// The package had no benchmark before this file. Emit and Write together are
// every byte of the exported WritePostScript job. The synthetic content is
// path operators only, because a page that paints an image returns undefined
// in Do at emit.go:42 and the writer emits no image operators.

// benchSegments is the path count in the synthetic page body.
const benchSegments = 2000

// benchPageContent builds a page body of benchSegments stroked lines, the same
// shape as the checked-in path PDF.
func benchPageContent() []byte {
	var buf bytes.Buffer
	for i := range benchSegments {
		fmt.Fprintf(&buf, "0 %d m 200 %d l S\n", i, i)
	}
	return buf.Bytes()
}

// BenchmarkEmit replays one page body through the recorder. The cost is the
// content scanner plus one recorded path per segment.
func BenchmarkEmit(b *testing.B) {
	content := benchPageContent()
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		out, err := Emit(b.Context(), content)
		if err != nil {
			b.Fatal(err)
		}
		if len(out) == 0 {
			b.Fatal("empty PostScript output")
		}
	}
}

// BenchmarkWrite frames ten pages as one PostScript program, so the header, the
// prolog, the ten %%Page comments, and the bounding box are all in the loop.
func BenchmarkWrite(b *testing.B) {
	content := benchPageContent()
	pages := make([]Page, 10)
	for i := range pages {
		pages[i] = Page{Content: content}
	}
	opt := WriteOptions{WidthPt: 200, HeightPt: 200}
	b.SetBytes(int64(len(content) * len(pages)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Write(b.Context(), pages, opt); err != nil {
			b.Fatal(err)
		}
	}
}
