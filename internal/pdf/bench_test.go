package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// benchContent is a 2000-stroke page body, the same shape as the checked-in
// path PDF, so the open and paint benchmarks carry a realistic stream.
func benchContent() []byte {
	var buf bytes.Buffer
	for i := range 2000 {
		fmt.Fprintf(&buf, "0 %d m 200 %d l S\n", i, i)
	}
	return buf.Bytes()
}

// benchFlate wraps raw in a zlib stream.
func benchFlate(raw []byte) []byte {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// benchClassicPDF builds a one-page classic xref file with a Flate content
// stream.
func benchClassicPDF() []byte {
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object(pageBody)
	doc.object(streamBody("/Filter /FlateDecode", benchFlate(benchContent())))
	return doc.classic("")
}

// benchXRefStreamPDF builds a one-page file whose page tree lives in a Flate
// object stream and whose xref is a Flate stream.
func benchXRefStreamPDF() []byte {
	plain, first := objectStreamPlain()
	objStm := streamBody(
		fmt.Sprintf("/Type /ObjStm /N 2 /First %d /Filter /FlateDecode", first),
		benchFlate(plain),
	)
	page := pageBody
	content := streamBody("", benchContent())

	doc := newDoc()
	doc.put(streamObjs, objStm)
	doc.put(streamPage, page)
	doc.put(streamContent, content)
	xrefAt := doc.buf.Len()
	rows := streamRows(doc.offsets, xrefAt)
	dict := fmt.Sprintf(
		"/Type /XRef /Size %d /Root 1 0 R /W [1 2 2] /Filter /FlateDecode",
		streamSize,
	)
	doc.put(streamXRef, streamBody(dict, benchFlate(rows)))
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

// benchMustOpen opens src and fails the benchmark on error.
func benchMustOpen(b *testing.B, src []byte) *File {
	b.Helper()
	file, err := Open(b.Context(), src)
	if err != nil {
		b.Fatal(err)
	}
	return file
}

// BenchmarkOpenClassicXRef parses a classic xref file with a Flate page.
func BenchmarkOpenClassicXRef(b *testing.B) {
	src := benchClassicPDF()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Open(b.Context(), src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOpenXRefStream parses an xref stream file with an object stream.
func BenchmarkOpenXRefStream(b *testing.B) {
	src := benchXRefStreamPDF()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Open(b.Context(), src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPaintPage paints one page of the classic file onto a 200 by 200
// pixmap.
func BenchmarkPaintPage(b *testing.B) {
	src := benchClassicPDF()
	file := benchMustOpen(b, src)
	content, err := file.Content(0)
	if err != nil {
		b.Fatal(err)
	}
	pixmap := graphics.NewPixmap(200, 200)
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := file.PaintPage(b.Context(), 0, pixmap, 1); err != nil {
			b.Fatal(err)
		}
	}
}
