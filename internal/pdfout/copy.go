package pdfout

import (
	"bytes"
	"context"
	"fmt"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// CopySource is the reader view the copy writer needs.
// *pdf.File implements it.
type CopySource interface {
	ObjectCount() int
	RootNum() int
	RawObject(num int) ([]byte, bool)
	ObjectValue(num int) (pdf.Value, bool, error)
}

// CopyOptions holds complete replacement object bodies keyed by object number.
// An override wins over the source body and is written as given.
type CopyOptions struct {
	Overrides map[int][]byte
}

// WriteCopy builds a classic PDF 1.4 file from every in-use source object.
// An object uses the override when present, then the stored source bytes, then
// pdf.SerializeValue. A free or missing number stays free.
// The trailer uses /Root from src and /ID as the SHA-256 of the written bodies.
// Two calls on the same source return equal bytes, and the file carries no
// /Info and no dates.
// A canceled context returns ctx.Err() and nil.
// A nil context panics with "pdfout: nil context".
func WriteCopy(ctx context.Context, src CopySource, opt CopyOptions) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	objects, err := collectCopy(src, opt)
	if err != nil {
		return nil, err
	}
	return buildCopyFile(src.RootNum(), objects), nil
}

// collectCopy returns one body per object number. Index 0 is unused, and a nil
// body marks a free number.
func collectCopy(src CopySource, opt CopyOptions) ([][]byte, error) {
	objects := make([][]byte, src.ObjectCount()+1)
	for num := 1; num < len(objects); num++ {
		body, err := copyBody(src, opt, num)
		if err != nil {
			return nil, err
		}
		objects[num] = body
	}
	return objects, nil
}

func copyBody(src CopySource, opt CopyOptions, num int) ([]byte, error) {
	if body, ok := opt.Overrides[num]; ok {
		return body, nil
	}
	if body, ok := src.RawObject(num); ok {
		return body, nil
	}
	val, ok, err := src.ObjectValue(num)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return pdf.SerializeValue(val), nil
}

func buildCopyFile(root int, objects [][]byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(headerLine)
	offsets := make([]int, len(objects))
	written := make([][]byte, 0, len(objects))
	for num := 1; num < len(objects); num++ {
		if objects[num] == nil {
			offsets[num] = -1
			continue
		}
		offsets[num] = writeObject(&buf, num, objects[num])
		written = append(written, objects[num])
	}
	xrefAt := buf.Len()
	writeCopyXref(&buf, offsets)
	writeCopyTrailer(&buf, len(offsets), root, xrefAt, digest(written...))
	return buf.Bytes()
}

// writeCopyXref writes one row per object number. A negative offset is free.
func writeCopyXref(buf *bytes.Buffer, offsets []int) {
	fmt.Fprintf(buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString(xrefRow(0, freeGen, 'f'))
	for _, offset := range offsets[1:] {
		if offset < 0 {
			buf.WriteString(xrefRow(0, 0, 'f'))
			continue
		}
		buf.WriteString(xrefRow(offset, 0, 'n'))
	}
}

func writeCopyTrailer(buf *bytes.Buffer, size, root, xrefAt int, sum string) {
	fmt.Fprintf(
		buf,
		"trailer\n<< /Size %d /Root %d 0 R /ID [<%s><%s>] >>\n",
		size,
		root,
		sum,
		sum,
	)
	fmt.Fprintf(buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
}
