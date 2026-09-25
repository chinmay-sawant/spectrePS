package psout

import (
	"bytes"
	"context"
	"strconv"
)

const (
	headerLine   = "%!PS-Adobe-3.0\n"
	boxLine      = "%%BoundingBox: 0 0 612 792\n"
	pagesLine    = "%%Pages: "
	pageLine     = "%%Page: "
	showpageLine = "showpage\n"
	trailerLine  = "%%EOF\n"
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

// WriteOptions selects output framing. The zero value is the supported shape
// for this tag: a fixed 612 by 792 box and no compression. Flate stays off
// until the interpreter reads FlateDecode, and media options wait.
type WriteOptions struct{}

// Write frames pages as one date-free PostScript program.
// The header is %!PS-Adobe-3.0 with a fixed 612 by 792 box, a prolog defines
// the short path operators m, l, S, f, and f* in terms of the long operators,
// each page gets a %%Page comment and one showpage, and the program ends with
// %%EOF. Write adds no creation date, so two calls with the same pages return
// equal bytes.
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
	_ = opts
	var buf bytes.Buffer
	writeHeader(&buf, len(pages))
	for idx, page := range pages {
		writePage(&buf, idx+1, page.Content)
	}
	buf.WriteString(trailerLine)
	return buf.Bytes(), nil
}

func writeHeader(buf *bytes.Buffer, count int) {
	buf.WriteString(headerLine)
	buf.WriteString(boxLine)
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
