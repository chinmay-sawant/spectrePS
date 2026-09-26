package psout

import (
	"bytes"
	"context"
	"math"
	"strconv"
)

const (
	headerLine   = "%!PS-Adobe-3.0\n"
	pagesLine    = "%%Pages: "
	pageLine     = "%%Page: "
	showpageLine = "showpage\n"
	trailerLine  = "%%EOF\n"
	// defaultBoxWidth and defaultBoxHeight are the reader default used when the
	// caller does not name a page. A DSC box is in points and must be integers,
	// so a fractional box is floored at the minimum and ceiled at the maximum.
	defaultBoxWidth  = 612
	defaultBoxHeight = 792
)

// prologText defines the short path operators the recorder emits. The long
// names are the ones RunPostScript implements, and a bare m or S is not a
// PostScript operator, so the program carries its own definitions the way the
// ps2write prolog does.
const prologText = "%%BeginProlog\n" +
	"/m /moveto load def\n" +
	"/l /lineto load def\n" +
	"/S /stroke load def\n" +
	"/f /fill load def\n" +
	"/f* /eofill load def\n" +
	"%%EndProlog\n"

// Page is one page of PostScript operators in 72 dpi points.
type Page struct {
	Content []byte
}

// WriteOptions selects output framing. WidthPt and HeightPt are the DSC
// %%BoundingBox in points, and they should be the document's real /MediaBox.
// A non-positive width or height falls back to the 612 by 792 reader default.
// Compression stays off until the interpreter reads FlateDecode, and the other
// media options wait.
type WriteOptions struct {
	WidthPt  float64
	HeightPt float64
}

// box returns the %%BoundingBox corners. PostScript DSC wants integers, so the
// minimum is floored and the maximum is ceiled, which is the same rule the
// bbox command uses.
func (opts WriteOptions) box() (int, int) {
	width, height := opts.WidthPt, opts.HeightPt
	if width <= 0 {
		width = defaultBoxWidth
	}
	if height <= 0 {
		height = defaultBoxHeight
	}
	return int(math.Floor(width)), int(math.Ceil(height))
}

// Write frames pages as one date-free PostScript program.
// The header is %!PS-Adobe-3.0 with the %%BoundingBox from opts, a prolog
// defines the short path operators m, l, S, f, and f* in terms of the long
// operators, each page gets a %%Page comment and one showpage, and the program
// ends with %%EOF. Write adds no creation date, so two calls with the same
// pages and the same options return equal bytes.
// A canceled context returns ctx.Err() and a nil slice.
// A nil context panics with "psout: nil context".
// Empty pages return a non-nil program with zero showpage lines.
func Write(ctx context.Context, pages []Page, opts WriteOptions) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	boxWidth, boxHeight := opts.box()
	writeHeader(&buf, len(pages), boxWidth, boxHeight)
	for idx, page := range pages {
		writePage(&buf, idx+1, page.Content)
	}
	buf.WriteString(trailerLine)
	return buf.Bytes(), nil
}

func writeHeader(buf *bytes.Buffer, count int, width, height int) {
	buf.WriteString(headerLine)
	buf.WriteString("%%BoundingBox: 0 0 ")
	buf.WriteString(strconv.Itoa(width))
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(height))
	buf.WriteByte('\n')
	buf.WriteString(pagesLine)
	buf.WriteString(strconv.Itoa(count))
	buf.WriteByte('\n')
	buf.WriteString(prologText)
}

func writePage(buf *bytes.Buffer, number int, content []byte) {
	buf.WriteString(pageLine)
	buf.WriteString(strconv.Itoa(number))
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(number))
	buf.WriteByte('\n')
	buf.Write(content)
	buf.WriteString(showpageLine)
}
