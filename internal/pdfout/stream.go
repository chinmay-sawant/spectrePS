package pdfout

import (
	"bytes"
	"compress/zlib"
	"fmt"
)

func streamParts(content []byte, compress bool) ([]byte, []byte, error) {
	stored, err := storedBytes(content, compress)
	if err != nil {
		return nil, nil, err
	}
	return stored, streamBody(stored, compress), nil
}

func storedBytes(content []byte, compress bool) ([]byte, error) {
	if !compress {
		return content, nil
	}
	return flateBytes(content)
}

func flateBytes(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(content)
	closeErr := writer.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return buf.Bytes(), nil
}

func streamBody(stored []byte, compress bool) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<< /Length %d", len(stored))
	if compress {
		buf.WriteString(" /Filter /FlateDecode")
	}
	buf.WriteString(" >>\nstream\n")
	buf.Write(stored)
	buf.WriteString("\nendstream")
	return buf.Bytes()
}
