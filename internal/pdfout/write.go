// Package pdfout writes classic PDF 1.4 files.
package pdfout

import (
	"bytes"
	"context"
	"fmt"
)

const panicNilContext = "pdfout: nil context"

const (
	headerLine   = "%PDF-1.4\n"
	pageWidth    = 612
	pageHeight   = 792
	catalogNum   = 1
	pagesNum     = 2
	firstPageNum = 3
	pagePair     = 2
)

// Page is one rewritten page. Content holds PDF content operators in user space.
type Page struct {
	Content []byte
}

type pageStream struct {
	stored []byte
	body   []byte
}

// Write builds a classic PDF 1.4 file from pages, in order.
// compress wraps each content stream in zlib and sets /Filter /FlateDecode.
// A canceled context returns ctx.Err() and a nil slice. A nil context panics with "pdfout: nil context".
func Write(ctx context.Context, pages []Page, compress bool) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	streams, err := collectStreams(pages, compress)
	if err != nil {
		return nil, err
	}
	return buildFile(streams), nil
}

func collectStreams(pages []Page, compress bool) ([]pageStream, error) {
	streams := make([]pageStream, len(pages))
	for i, page := range pages {
		stored, body, err := streamParts(page.Content, compress)
		if err != nil {
			return nil, err
		}
		streams[i] = pageStream{stored: stored, body: body}
	}
	return streams, nil
}

func buildFile(streams []pageStream) []byte {
	var buf bytes.Buffer
	buf.WriteString(headerLine)
	offsets := objectOffsets(&buf, streams)
	xrefAt := buf.Len()
	writeXref(&buf, offsets)
	writeTrailer(&buf, len(offsets), xrefAt, contentDigest(streams))
	return buf.Bytes()
}

func objectOffsets(buf *bytes.Buffer, streams []pageStream) []int {
	// Index 0 is the free xref row. In-use objects start at 1.
	offsets := make([]int, 1, firstPageNum+pagePair*len(streams))
	offsets = append(offsets, writeObject(buf, catalogNum, catalogObject()))
	offsets = append(offsets, writeObject(buf, pagesNum, pagesObject(len(streams))))
	for i, stream := range streams {
		pageNum := firstPageNum + pagePair*i
		streamNum := pageNum + 1
		offsets = append(offsets, writeObject(buf, pageNum, pageObject(streamNum)))
		offsets = append(offsets, writeObject(buf, streamNum, stream.body))
	}
	return offsets
}

func catalogObject() []byte {
	return []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesNum))
}

func pagesObject(count int) []byte {
	var buf bytes.Buffer
	buf.WriteString("<< /Type /Pages /Kids ")
	writeKids(&buf, count)
	fmt.Fprintf(&buf, " /Count %d >>", count)
	return buf.Bytes()
}

func writeKids(buf *bytes.Buffer, count int) {
	if count == 0 {
		buf.WriteString("[]")
		return
	}
	buf.WriteByte('[')
	for i := range count {
		if i > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(buf, "%d 0 R", firstPageNum+pagePair*i)
	}
	buf.WriteByte(']')
}

func pageObject(streamNum int) []byte {
	const format = "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %d %d] " +
		"/Contents %d 0 R /Resources << >> >>"
	text := fmt.Sprintf(format, pagesNum, pageWidth, pageHeight, streamNum)
	return []byte(text)
}

func writeObject(buf *bytes.Buffer, num int, body []byte) int {
	offset := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n", num)
	buf.Write(body)
	buf.WriteString("\nendobj\n")
	return offset
}
