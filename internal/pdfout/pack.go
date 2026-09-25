package pdfout

import (
	"bytes"
	"fmt"
)

const (
	packedHeaderLine = "%PDF-1.5\n"
	// streamWord ends the dictionary of a written stream body. Both a source
	// clone and pdf.SerializeValue put it after the dictionary.
	streamWord = "\nstream\n"
	// xrefRowWidth is the byte count of one W [1 4 2] row.
	xrefRowWidth = 7
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
	objStmNum := 0
	xrefNum := len(objects)
	if len(packed) > 0 {
		objStmNum = len(objects)
		xrefNum = len(objects) + 1
	}
	size := xrefNum + 1

	var buf bytes.Buffer
	buf.WriteString(packedHeaderLine)
	offsets := make([]int, size)
	for num := 1; num < len(objects); num++ {
		if objects[num] == nil || !isStreamBody(objects[num]) {
			offsets[num] = -1
			continue
		}
		offsets[num] = writeObject(&buf, num, objects[num])
	}
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
	return []byte(fmt.Sprintf(
		"<< /Type /ObjStm /N %d /First %d /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream",
		count,
		first,
		len(stored),
		stored,
	))
}

func xrefStreamBody(size, root int, sum string, stored []byte) []byte {
	return []byte(fmt.Sprintf(
		"<< /Type /XRef /Size %d /Root %d 0 R /W [1 4 2] /ID [<%s><%s>] /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream",
		size,
		root,
		sum,
		sum,
		len(stored),
		stored,
	))
}

// packedXrefRows writes W [1 4 2] rows for objects 0 through size-1. A packed
// body is a type 2 row that names the object stream and its index.
func packedXrefRows(size int, offsets, packed []int, objStmNum int) []byte {
	at := make(map[int]int, len(packed))
	for index, num := range packed {
		at[num] = index
	}
	rows := make([]byte, 0, xrefRowWidth*size)
	rows = appendPackedRow(rows, 0, 0, freeGen)
	for num := 1; num < size; num++ {
		if offsets[num] >= 0 {
			rows = appendPackedRow(rows, 1, offsets[num], 0)
			continue
		}
		if index, ok := at[num]; ok {
			rows = appendPackedRow(rows, 2, objStmNum, index)
			continue
		}
		rows = appendPackedRow(rows, 0, 0, 0)
	}
	return rows
}

func appendPackedRow(rows []byte, kind byte, second, third int) []byte {
	return append(
		rows,
		kind,
		byte(second>>24),
		byte(second>>16),
		byte(second>>8),
		byte(second),
		byte(third>>8),
		byte(third),
	)
}
