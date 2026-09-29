package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

func TestPDFReject(t *testing.T) {
	rejectTj(t)
	rejectDo(t)
	rejectEncrypt(t)
	rejectStandardHandlers(t)
	badLZW(t)
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

// rejectStandardHandlers locks the refusal against a complete, legal security
// handler rather than the bare /Filter /Standard stub above. Every revision
// the reader must refuse gets a fully populated dictionary with an /ID, because
// that is the shape a producer writes and the shape a future change is most
// likely to special-case on the way to accepting it. The batch2 corpus holds
// 57 such files, all of which Ghostscript opens with no password, so the only
// thing standing between them and the reader is this policy.
//
// The refusal is the contract, not a gap. `documentation/features.md`,
// `documentation/cli.md`, and `documentation/pdf-compatibility.md` all state
// it, and `plans/v0.0.1/10-deferred.md` 10.2 defers encryption behind RC4,
// AES-128, and AES-256 key derivation, a crypt filter model, and decryption at
// every string and stream read. Empty passwords do not make that smaller, so
// these cases are pinned until that plan lands and replaces them.
func rejectStandardHandlers(t *testing.T) {
	t.Helper()
	// The /O and /U values are the lengths each revision requires. Their
	// contents are not an empty-password encoding and do not need to be: the
	// reader must refuse before it derives a key, so the values never reach a
	// check. Writing them as hex strings keeps the lengths honest.
	const (
		o16 = "0123456789ABCDEF0123456789ABCDEF"
		o32 = "0123456789ABCDEF0123456789ABCDEF" +
			"0123456789ABCDEF0123456789ABCDEF"
		o48 = "0123456789ABCDEF0123456789ABCDEF" +
			"0123456789ABCDEF0123456789ABCDEF" +
			"0123456789ABCDEF0123456789ABCDEF"
		cryptFilters = " /CF << /StdCF << /CFM /AESV2 /Length 16 >> >>" +
			" /StmF /StdCF /StrF /StdCF"
	)
	cases := []struct {
		name string
		dict string
	}{
		{"RC4 40 bit, revision 2", "<< /Filter /Standard /V 1 /R 2 /Length 40 " +
			"/P -44 /O <" + o32 + "> /U <" + o16 + ">>"},
		{"RC4 128 bit, revision 3", "<< /Filter /Standard /V 2 /R 3 /Length 128 " +
			"/P -44 /O <" + o32 + "> /U <" + o32 + ">>"},
		{"crypt filters, revision 4", "<< /Filter /Standard /V 4 /R 4 /Length 128 " +
			"/P -44 /O <" + o32 + "> /U <" + o32 + ">" + cryptFilters + ">>"},
		{"AES 256, revision 5", "<< /Filter /Standard /V 5 /R 5 /Length 256 " +
			"/P -44 /O <" + o48 + "> /U <" + o48 + "> /OE <" + o32 + "> /UE <" + o32 +
			"> /Perms <" + o16 + ">" + cryptFilters + ">>"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// The /ID matters as much as the /U: an empty-password check
			// reads both, so a lock that omits it proves less.
			src := standardHandlerPage(t, testCase.dict)
			_, err := Open(t.Context(), src)
			wantJob(t, err, opEncrypt, errAccess)
		})
	}
}

// standardHandlerPage builds a one-page document whose trailer points at an
// indirect /Encrypt object and carries an /ID, which is how a producer writes
// an encrypted file.
func standardHandlerPage(t *testing.T, dict string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object(pageBody)
	doc.object(streamBody("", []byte("q\nQ")))
	doc.object(dict)
	return doc.classic(" /ID [<0123456789ABCDEF0123456789ABCDEF>" +
		"<0123456789ABCDEF0123456789ABCDEF>] /Encrypt 6 0 R")
}

// badLZW proves a malformed LZW stream fails as a decode error with the
// filter name, not as an unknown filter.
func badLZW(t *testing.T) {
	t.Helper()
	src := filteredPage(t, "/Filter /LZWDecode", []byte("hi"))
	file, err := Open(t.Context(), src)
	if err == nil {
		_, err = file.Content(0)
	}
	wantJob(t, err, opLZW, errSyntax)
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
	doc.object(pageBody)
	doc.object(streamBody("", []byte(marks)))
	return doc.classic(extra)
}

func filteredPage(t *testing.T, dict string, raw []byte) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object(pageBody)
	doc.object(streamBody(dict, raw))
	return doc.classic("")
}
