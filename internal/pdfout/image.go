package pdfout

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	defaultImageDPI  = 72
	imageChannels    = 3
	imagePageObjects = 3
	pointsPerInch    = 72
)

// imagePage holds the stream bodies and the media size for one image page.
type imagePage struct {
	body     []byte
	content  []byte
	stored   []byte
	widthPt  string
	heightPt string
}

// WriteImages builds a classic PDF 1.4 file with one 24-bit RGB image per page.
// Each image stream is Flate, and each MediaBox is [0 0 width*72/dpi height*72/dpi] points.
// A dpi of zero or less selects 72.
// A canceled context returns ctx.Err() and nil. A nil context panics with "pdfout: nil context".
func WriteImages(ctx context.Context, pages []graphics.Image, dpi float64) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	built, err := collectImages(pages, dpi)
	if err != nil {
		return nil, err
	}
	return buildImageFile(built), nil
}

func collectImages(pages []graphics.Image, dpi float64) ([]imagePage, error) {
	if dpi <= 0 {
		dpi = defaultImageDPI
	}
	built := make([]imagePage, len(pages))
	for i, img := range pages {
		stored, err := flateBytes(packedRGB(img))
		if err != nil {
			return nil, err
		}
		width := pointSize(img.Width, dpi)
		height := pointSize(img.Height, dpi)
		paint := fmt.Sprintf("q %s 0 0 %s 0 0 cm /Im0 Do Q", width, height)
		built[i] = imagePage{
			body:     imageBody(img, stored),
			content:  streamBody([]byte(paint), false),
			stored:   stored,
			widthPt:  width,
			heightPt: height,
		}
	}
	return built, nil
}

// pointSize converts a pixel count at dpi to points.
func pointSize(pixels int, dpi float64) string {
	return strconv.FormatFloat(float64(pixels)*pointsPerInch/dpi, 'f', -1, 64)
}

// packedRGB drops stride padding. Row 0 stays first.
func packedRGB(img graphics.Image) []byte {
	rowBytes := img.Width * imageChannels
	if img.Stride == rowBytes {
		return img.Pixels
	}
	packed := make([]byte, 0, img.Height*rowBytes)
	for row := range img.Height {
		start := row * img.Stride
		packed = append(packed, img.Pixels[start:start+rowBytes]...)
	}
	return packed
}

func imageBody(img graphics.Image, stored []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("<< /Type /XObject /Subtype /Image")
	fmt.Fprintf(&buf, " /Width %d /Height %d", img.Width, img.Height)
	buf.WriteString(" /ColorSpace /DeviceRGB /BitsPerComponent 8")
	fmt.Fprintf(&buf, " /Filter /FlateDecode /Length %d >>\nstream\n", len(stored))
	buf.Write(stored)
	buf.WriteString("\nendstream")
	return buf.Bytes()
}

func buildImageFile(pages []imagePage) []byte {
	var buf bytes.Buffer
	buf.WriteString(headerLine)
	offsets := imageOffsets(&buf, pages)
	xrefAt := buf.Len()
	writeXref(&buf, offsets)
	writeTrailer(&buf, len(offsets), xrefAt, imageDigest(pages))
	return buf.Bytes()
}

func imageOffsets(buf *bytes.Buffer, pages []imagePage) []int {
	// Index 0 is the free xref row. In-use objects start at 1.
	offsets := make([]int, 1, firstPageNum+imagePageObjects*len(pages))
	offsets = append(offsets, writeObject(buf, catalogNum, catalogObject()))
	offsets = append(offsets, writeObject(buf, pagesNum, imagePagesObject(len(pages))))
	for i, page := range pages {
		pageNum := firstPageNum + imagePageObjects*i
		contentNum := pageNum + 1
		imageNum := contentNum + 1
		offsets = append(offsets, writeObject(buf, pageNum, imagePageObject(page, contentNum, imageNum)))
		offsets = append(offsets, writeObject(buf, contentNum, page.content))
		offsets = append(offsets, writeObject(buf, imageNum, page.body))
	}
	return offsets
}

func imagePagesObject(count int) []byte {
	var buf bytes.Buffer
	buf.WriteString("<< /Type /Pages /Kids ")
	writeImageKids(&buf, count)
	fmt.Fprintf(&buf, " /Count %d >>", count)
	return buf.Bytes()
}

func writeImageKids(buf *bytes.Buffer, count int) {
	if count == 0 {
		buf.WriteString("[]")
		return
	}
	buf.WriteByte('[')
	for i := range count {
		if i > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(buf, "%d 0 R", firstPageNum+imagePageObjects*i)
	}
	buf.WriteByte(']')
}

func imagePageObject(page imagePage, contentNum, imageNum int) []byte {
	const format = "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] " +
		"/Contents %d 0 R /Resources << /XObject << /Im0 %d 0 R >> >> >>"
	return []byte(fmt.Sprintf(format, pagesNum, page.widthPt, page.heightPt, contentNum, imageNum))
}

func imageDigest(pages []imagePage) string {
	parts := make([][]byte, len(pages))
	for i, page := range pages {
		parts[i] = page.stored
	}
	return digest(parts...)
}
