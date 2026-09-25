package pdfout

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"
)

// flateWriterPool reuses one zlib writer across streams. A fresh writer
// allocates the deflate window and hash tables, about 700 KiB, so a rewrite
// that flates many streams pays that cost per stream without the pool.
var (
	// flateWriterPool reuses one zlib writer per active benchmark or job.
	flateWriterPool = sync.Pool{ //nolint:gochecknoglobals // one writer pool per process
		New: func() any { return zlib.NewWriter(io.Discard) },
	}
	errFlateWriterPoolType = errors.New(
		"pdfout: flate writer pool holds a non-writer")
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
	return withFlateWriter(func(writer *zlib.Writer) error {
		_, err := writer.Write(content)
		return err
	})
}

// withFlateWriter runs fn against one pooled zlib writer and returns the
// compressed bytes.
func withFlateWriter(fn func(*zlib.Writer) error) ([]byte, error) {
	var buf bytes.Buffer
	pooled := flateWriterPool.Get()
	writer, ok := pooled.(*zlib.Writer)
	if !ok {
		return nil, errFlateWriterPoolType
	}
	writer.Reset(&buf)
	err := fn(writer)
	closeErr := writer.Close()
	flateWriterPool.Put(writer)
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
