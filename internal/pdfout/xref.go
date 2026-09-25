package pdfout

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const freeGen = 65535

func contentDigest(streams []pageStream) string {
	parts := make([][]byte, len(streams))
	for i, stream := range streams {
		parts[i] = stream.stored
	}
	return digest(parts...)
}

// digest is the SHA-256 hex of the parts in order.
func digest(parts ...[]byte) string {
	sum := sha256.New()
	for _, part := range parts {
		_, _ = sum.Write(part)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func writeXref(buf *bytes.Buffer, offsets []int) {
	fmt.Fprintf(buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString(xrefRow(0, freeGen, 'f'))
	for _, offset := range offsets[catalogNum:] {
		buf.WriteString(xrefRow(offset, 0, 'n'))
	}
}

func xrefRow(offset, gen int, flag byte) string {
	return fmt.Sprintf("%010d %05d %c \n", offset, gen, flag)
}

func writeTrailer(buf *bytes.Buffer, size, xrefAt int, digest string) {
	fmt.Fprintf(
		buf,
		"trailer\n<< /Size %d /Root %d 0 R /ID [<%s><%s>] >>\n",
		size,
		catalogNum,
		digest,
		digest,
	)
	fmt.Fprintf(buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
}
