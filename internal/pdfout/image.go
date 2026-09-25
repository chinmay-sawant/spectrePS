package pdfout

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	defaultImageDPI  = 72
	imageChannels    = 3
	cmykChannels     = 4
	imagePageObjects = 3
	pointsPerInch    = 72
	lumaRed          = 0.299
	lumaGreen        = 0.587
	lumaBlue         = 0.114
	byteScale        = 255
)

// ImageColorSpace selects the color space of the packed image streams.
type ImageColorSpace int

const (
	// ImageRGB stores 24-bit RGB, 3 bytes per pixel, with /DeviceRGB.
	ImageRGB ImageColorSpace = iota
	// ImageGray stores 8-bit gray, 1 byte per pixel, with /DeviceGray.
	ImageGray
	// ImageCMYK stores 32-bit CMYK, 4 bytes per pixel, with /DeviceCMYK.
	ImageCMYK
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
// It is WriteImagesColor with ImageRGB. Each image stream is Flate, and each
// MediaBox is [0 0 width*72/dpi height*72/dpi] points. A dpi of zero or less
// selects 72. A canceled context returns ctx.Err() and nil.
// A nil context panics with "pdfout: nil context".
func WriteImages(ctx context.Context, pages []graphics.Image, dpi float64) ([]byte, error) {
	return WriteImagesColor(ctx, pages, dpi, ImageRGB)
}

// WriteImagesColor builds a classic PDF 1.4 file with one image per page in
// space. ImageRGB stores 3 bytes per pixel, ImageGray 1, and ImageCMYK 4. Each
// stream is Flate, and each MediaBox is [0 0 width*72/dpi height*72/dpi]
// points. A dpi of zero or less selects 72. A canceled context returns
// ctx.Err() and nil. A nil context panics with "pdfout: nil context".
func WriteImagesColor(
	ctx context.Context,
	pages []graphics.Image,
	dpi float64,
	space ImageColorSpace,
) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	built, err := collectImages(pages, dpi, space)
	if err != nil {
		return nil, err
	}
	return buildImageFile(built), nil
}

func collectImages(pages []graphics.Image, dpi float64, space ImageColorSpace) ([]imagePage, error) {
	if dpi <= 0 {
		dpi = defaultImageDPI
	}
	built := make([]imagePage, len(pages))
	for i, img := range pages {
		stored, err := flateBytes(packSamples(img, space))
		if err != nil {
			return nil, err
		}
		width := pointSize(img.Width, dpi)
		height := pointSize(img.Height, dpi)
		paint := fmt.Sprintf("q %s 0 0 %s 0 0 cm /Im0 Do Q", width, height)
		built[i] = imagePage{
			body:     imageBody(img, stored, space),
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

// packSamples drops stride padding and converts each pixel to space. Any value
// other than ImageGray or ImageCMYK stores RGB.
func packSamples(img graphics.Image, space ImageColorSpace) []byte {
	switch space {
	case ImageGray:
		return packedGray(img)
	case ImageCMYK:
		return packedCMYK(img)
	case ImageRGB:
		return packedRGB(img)
	default:
		return packedRGB(img)
	}
}

// packedGray converts one RGB triple per pixel to one luma byte.
func packedGray(img graphics.Image) []byte {
	out := make([]byte, 0, img.Width*img.Height)
	for row := range img.Height {
		base := row * img.Stride
		for col := range img.Width {
			at := base + col*imageChannels
			out = append(out, luma(img.Pixels[at], img.Pixels[at+1], img.Pixels[at+2]))
		}
	}
	return out
}

// luma is BT.601 Y, rounded to a byte.
func luma(red, green, blue byte) byte {
	value := lumaRed*float64(red) + lumaGreen*float64(green) + lumaBlue*float64(blue)
	return byte(math.Round(value))
}

// packedCMYK converts one RGB triple per pixel to four CMYK bytes.
func packedCMYK(img graphics.Image) []byte {
	out := make([]byte, 0, img.Width*img.Height*cmykChannels)
	for row := range img.Height {
		base := row * img.Stride
		for col := range img.Width {
			at := base + col*imageChannels
			c, m, y, k := rgbToCMYK(img.Pixels[at], img.Pixels[at+1], img.Pixels[at+2])
			out = append(out, c, m, y, k)
		}
	}
	return out
}

// rgbToCMYK applies the K-first conversion frozen in documentation/devices.md.
func rgbToCMYK(red, green, blue byte) (byte, byte, byte, byte) {
	redF := float64(red) / byteScale
	greenF := float64(green) / byteScale
	blueF := float64(blue) / byteScale
	black := 1 - math.Max(redF, math.Max(greenF, blueF))
	if black >= 1 {
		return 0, 0, 0, toByte(black)
	}
	cyan := (1 - redF - black) / (1 - black)
	magenta := (1 - greenF - black) / (1 - black)
	yellow := (1 - blueF - black) / (1 - black)
	return toByte(cyan), toByte(magenta), toByte(yellow), toByte(black)
}

func toByte(value float64) byte {
	return byte(math.Round(value * byteScale))
}

func imageBody(img graphics.Image, stored []byte, space ImageColorSpace) []byte {
	var buf bytes.Buffer
	buf.WriteString("<< /Type /XObject /Subtype /Image")
	fmt.Fprintf(&buf, " /Width %d /Height %d", img.Width, img.Height)
	fmt.Fprintf(&buf, " /ColorSpace %s /BitsPerComponent 8", space.deviceName())
	fmt.Fprintf(&buf, " /Filter /FlateDecode /Length %d >>\nstream\n", len(stored))
	buf.Write(stored)
	buf.WriteString("\nendstream")
	return buf.Bytes()
}

// deviceName is the PDF name of the stream color space.
func (space ImageColorSpace) deviceName() string {
	switch space {
	case ImageGray:
		return "/DeviceGray"
	case ImageCMYK:
		return "/DeviceCMYK"
	case ImageRGB:
		return "/DeviceRGB"
	default:
		return "/DeviceRGB"
	}
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
