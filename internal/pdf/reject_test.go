package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

func TestPDFReject(t *testing.T) {
	rejectTj(t)
	rejectDo(t)
	rejectEncrypt(t)
	rejectLZW(t)
}

func rejectTj(t *testing.T) {
	t.Helper()
	file := mustOpen(t, textPage(t, "(Hi) Tj"))
	pix := graphics.NewPixmap(4, 4)
	err := file.PaintPage(t.Context(), 0, pix, 1)
	wantJob(t, err, "Tj", "undefined")
}

func rejectDo(t *testing.T) {
	t.Helper()
	file := mustOpen(t, textPage(t, "/Im Do"))
	pix := graphics.NewPixmap(4, 4)
	err := file.PaintPage(t.Context(), 0, pix, 1)
	wantJob(t, err, "Do", "undefined")
}

func rejectEncrypt(t *testing.T) {
	t.Helper()
	src := textPageTrailer(t, "q", " /Encrypt << /Filter /Standard >>")
	_, err := Open(t.Context(), src)
	wantJob(t, err, opEncrypt, errAccess)
}

func rejectLZW(t *testing.T) {
	t.Helper()
	src := filteredPage(t, "/Filter /LZWDecode", []byte("hi"))
	file, err := Open(t.Context(), src)
	if err == nil {
		_, err = file.Content(0)
	}
	wantJob(t, err, "LZWDecode", "undefined")
}

func textPage(t *testing.T, marks string) []byte {
	t.Helper()
	return textPageTrailer(t, marks, "")
}

func textPageTrailer(t *testing.T, marks, extra string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody("", []byte(marks)))
	return doc.classic(extra)
}

func filteredPage(t *testing.T, dict string, raw []byte) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	doc.object(streamBody(dict, raw))
	return doc.classic("")
}
