package spectreps_test

import (
	"bytes"
	"testing"
)

// TestValidationOpenStructs opens an xref stream and an object stream through
// the public OpenPDF. PageCount walks the page tree, so it must be the leaf
// count, not the /Count field of the root node.
func TestValidationOpenStructs(t *testing.T) {
	in := newInst(t)
	cases := []struct {
		name  string
		pages int
		src   []byte
	}{
		{name: "xref stream", pages: 2, src: validationXRefStreamPDF(t)},
		{name: "object stream", pages: 2, src: validationObjectStreamPDF(t)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := in.OpenPDF(t.Context(), tt.src)
			if err != nil {
				t.Fatalf("OpenPDF() error = %v", err)
			}
			if doc == nil {
				t.Fatal("OpenPDF() document = nil")
			}
			if got := doc.PageCount(); got != tt.pages {
				t.Fatalf("PageCount() = %d, want %d", got, tt.pages)
			}
		})
	}
}

// validationXRefStreamPDF builds a two-page file whose only xref section is an
// xref stream. The /Pages node lies with /Count 1, so PageCount proves the walk.
func validationXRefStreamPDF(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writeString(t, &body, "%PDF-1.5\n")
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>"),
	}
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, body.Len())
		writef(t, &body, "%d 0 obj\n", i+1)
		writeAll(t, &body, object)
		writeString(t, &body, "\nendobj\n")
	}
	xrefAt := body.Len()
	offsets = append(offsets, xrefAt)
	rows := validationOffsetsRows(offsets)
	validationWriteXRefStream(t, &body, len(offsets)-1, rows, len(offsets))
	writef(t, &body, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return body.Bytes()
}

// validationObjectStreamPDF packs the catalog, pages, and page nodes into one
// object stream. The xref stream names the packed numbers with type 2 rows.
func validationObjectStreamPDF(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writeString(t, &body, "%PDF-1.5\n")
	packed := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] >>"),
	}
	var header bytes.Buffer
	var packedBody bytes.Buffer
	for i, object := range packed {
		writef(t, &header, "%d %d ", i+1, packedBody.Len())
		writeAll(t, &packedBody, object)
		writeString(t, &packedBody, "\n")
	}
	objectStream := append(append([]byte{}, header.Bytes()...), packedBody.Bytes()...)
	offsets := []int{0, -1, -1, -1, -1, body.Len()}
	writef(t, &body, "5 0 obj\n<< /Type /ObjStm /N %d /First %d /Length %d >>\nstream\n",
		len(packed), header.Len(), len(objectStream))
	writeAll(t, &body, objectStream)
	writeString(t, &body, "\nendstream\nendobj\n")
	xrefAt := body.Len()
	offsets = append(offsets, xrefAt)
	rows := validationPackedRows(offsets)
	validationWriteXRefStream(t, &body, len(offsets)-1, rows, len(offsets))
	writef(t, &body, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return body.Bytes()
}

// validationOffsetsRows writes one type 1 row per object: object 0 free, every
// other object a byte offset.
func validationOffsetsRows(offsets []int) []byte {
	rows := validationAppendRow(nil, 0, 0, 65535)
	for _, offset := range offsets[1:] {
		rows = validationAppendRow(rows, 1, offset, 0)
	}
	return rows
}

// validationPackedRows writes type 2 rows for objects 1 through 4, which live
// in object stream 5, then type 1 rows for 5 and the xref stream itself.
func validationPackedRows(offsets []int) []byte {
	rows := validationAppendRow(nil, 0, 0, 65535)
	for num := 1; num <= 4; num++ {
		rows = validationAppendRow(rows, 2, 5, num-1)
	}
	for _, offset := range offsets[5:] {
		rows = validationAppendRow(rows, 1, offset, 0)
	}
	return rows
}

// validationAppendRow appends one W [1 4 2] row.
func validationAppendRow(rows []byte, kind, second, third int) []byte {
	rows = append(rows, byte(kind))
	rows = append(rows, byte(second>>24), byte(second>>16), byte(second>>8), byte(second))
	return append(rows, byte(third>>8), byte(third))
}

// validationWriteXRefStream writes the xref stream object and its raw rows.
func validationWriteXRefStream(t *testing.T, body *bytes.Buffer, num int, rows []byte, size int) {
	t.Helper()
	writef(t, body, "%d 0 obj\n", num)
	writef(t, body, "<< /Type /XRef /Size %d /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n",
		size, len(rows))
	writeAll(t, body, rows)
	writeString(t, body, "\nendstream\nendobj\n")
}
