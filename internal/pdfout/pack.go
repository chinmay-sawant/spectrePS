package pdfout

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	packedHeaderLine = "%PDF-1.5\n"
	// streamWord ends the dictionary of a written stream body. Both a source
	// clone and pdf.SerializeValue put it after the dictionary.
	streamWord = "\nstream\n"
	// A packed xref row uses W [1 4 2]: a type byte, a four-byte offset or
	// object stream number, and a two-byte index or generation.
	packedFieldWidth = 4
	packedIndexWidth = 2
	xrefRowWidth     = 1 + packedFieldWidth + packedIndexWidth
	// packedExtraObjects is the object stream plus the xref stream.
	packedExtraObjects = 2
)

// The three row kinds of an xref stream.
const (
	packedRowFree   = 0
	packedRowPlain  = 1
	packedRowPacked = 2
)

// isStreamBody reports whether one written object body holds a stream.
func isStreamBody(body []byte) bool {
	return bytes.Contains(body, []byte(streamWord))
}

// buildPackedCopyFile writes the copy as PDF 1.5. Stream bodies stay plain
// indirect objects, every other body goes into one Flate /Type /ObjStm, and a
// /Type /XRef stream carries the rows and the trailer. The /ID digest is over
// the written bodies in object-number order, so it matches buildCopyFile.
func buildPackedCopyFile(root int, objects [][]byte) ([]byte, error) {
	written, packed := splitPacked(objects)
	objStmNum, xrefNum, size := packedNumbers(len(objects), len(packed))

	var buf bytes.Buffer
	buf.WriteString(packedHeaderLine)
	offsets := writePackedStreams(&buf, objects)
	if len(packed) > 0 {
		stored, first, err := packedBody(packed, objects)
		if err != nil {
			return nil, err
		}
		offsets[objStmNum] = writeObject(&buf, objStmNum, objStmBody(len(packed), first, stored))
	}
	xrefAt := buf.Len()
	offsets[xrefNum] = xrefAt
	rows := packedXrefRows(size, offsets, packed, objStmNum)
	stored, err := flateBytes(rows)
	if err != nil {
		return nil, err
	}
	writeObject(&buf, xrefNum, xrefStreamBody(size, root, digest(written...), stored))
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return buf.Bytes(), nil
}

// splitPacked returns the written bodies in object-number order and the object
// numbers whose bodies go into the object stream.
func splitPacked(objects [][]byte) ([][]byte, []int) {
	written := make([][]byte, 0, len(objects))
	packed := make([]int, 0, len(objects))
	for num := 1; num < len(objects); num++ {
		body := objects[num]
		if body == nil {
			continue
		}
		written = append(written, body)
		if !isStreamBody(body) {
			packed = append(packed, num)
		}
	}
	return written, packed
}

// packedNumbers returns the object stream number, the xref stream number, and
// /Size. Without packed bodies the xref stream takes the next number.
func packedNumbers(count, packedCount int) (int, int, int) {
	if packedCount == 0 {
		return 0, count, count + 1
	}
	return count, count + 1, count + packedExtraObjects
}

// writePackedStreams writes every stream body as a plain object and returns
// one offset per object number. A nil or packed number gets -1.
func writePackedStreams(buf *bytes.Buffer, objects [][]byte) []int {
	offsets := make([]int, len(objects)+packedExtraObjects)
	for num := 1; num < len(objects); num++ {
		if objects[num] == nil || !isStreamBody(objects[num]) {
			offsets[num] = -1
			continue
		}
		offsets[num] = writeObject(buf, num, objects[num])
	}
	return offsets
}

// packedBody flates the object stream. first is the byte count of the header
// pairs, and the returned bytes are the stored stream.
func packedBody(packed []int, objects [][]byte) ([]byte, int, error) {
	var header bytes.Buffer
	offset := 0
	for i, num := range packed {
		if i > 0 {
			header.WriteByte(' ')
		}
		fmt.Fprintf(&header, "%d %d", num, offset)
		offset += len(objects[num]) + 1
	}
	header.WriteByte('\n')
	plain := make([]byte, 0, header.Len()+offset)
	plain = append(plain, header.Bytes()...)
	for _, num := range packed {
		plain = append(plain, objects[num]...)
		plain = append(plain, '\n')
	}
	stored, err := flateBytes(plain)
	if err != nil {
		return nil, 0, err
	}
	return stored, header.Len(), nil
}

func objStmBody(count, first int, stored []byte) []byte {
	const format = "<< /Type /ObjStm /N %d /First %d /Filter /FlateDecode /Length %d >>\n" +
		"stream\n%s\nendstream"
	return []byte(fmt.Sprintf(format, count, first, len(stored), stored))
}

func xrefStreamBody(size, root int, sum string, stored []byte) []byte {
	const format = "<< /Type /XRef /Size %d /Root %d 0 R /W [1 4 2] /ID [<%s><%s>] " +
		"/Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream"
	return []byte(fmt.Sprintf(format, size, root, sum, sum, len(stored), stored))
}

// packedXrefRows writes W [1 4 2] rows for objects 0 through size-1. A packed
// body is a type 2 row that names the object stream and its index.
func packedXrefRows(size int, offsets, packed []int, objStmNum int) []byte {
	packedAt := make(map[int]int, len(packed))
	for index, num := range packed {
		packedAt[num] = index
	}
	rows := make([]byte, 0, xrefRowWidth*size)
	rows = appendPackedRow(rows, packedRowFree, 0, freeGen)
	for num := 1; num < size; num++ {
		if offsets[num] >= 0 {
			rows = appendPackedRow(rows, packedRowPlain, offsets[num], 0)
			continue
		}
		if index, ok := packedAt[num]; ok {
			rows = appendPackedRow(rows, packedRowPacked, objStmNum, index)
			continue
		}
		rows = appendPackedRow(rows, packedRowFree, 0, 0)
	}
	return rows
}

func appendPackedRow(rows []byte, kind byte, second, third int) []byte {
	var fields [packedFieldWidth + packedIndexWidth]byte
	binary.BigEndian.PutUint32(fields[:packedFieldWidth], uint32(second)) //nolint:gosec // bounded in memory
	binary.BigEndian.PutUint16(fields[packedFieldWidth:], uint16(third))  //nolint:gosec // index or generation
	return append(append(rows, kind), fields[:]...)
}
