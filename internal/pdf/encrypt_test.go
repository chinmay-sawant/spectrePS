package pdf

// The /Encrypt refusal is a policy, not a parse failure. ISO 32000-1 lets a
// trailer name a Standard security handler, and every handler revision from
// /R 2 to /R 6 is a legal PDF that a reader may open when the user password is
// empty. Spectre implements no key derivation, so it refuses them all with the
// same named error, and these tests hold that line for the dictionary shapes
// real files carry: an indirect reference, each handler revision, a crypt
// filter, and /EncryptMetadata. plans/v0.0.1/10-deferred.md 10.2 gates the work.
// The last subtest is the neighbouring case that must not be refused: a null
// /Encrypt means the document is not encrypted and the file opens.
//
// Buckets b08 and b09 of the batch2 corpus hold 118 such files between them.
// The subtests below pin the shapes that b08 carries beyond the revisions
// above: literal-string /O and /U, a complete inline dictionary, an RC4 /V2
// crypt filter, over-long R5 entries, and the structurally broken trailers.
// Two b08 files carry no /ID at all, one carries /ID [()()], and one names an
// /Encrypt object the table cannot resolve. All three are still refused with
// the same error, and that is deliberate: the refusal is read from the trailer
// entry before any handler or key material is touched, which is also the
// behaviour the deferred decryption work has to preserve when it starts
// reading the handler itself.

import (
	"strings"
	"testing"
)

// TestPDFEncryptRefusal proves a trailer /Encrypt is refused with the Encrypt
// op whatever shape the security dictionary takes.
func TestPDFEncryptRefusal(t *testing.T) {
	refuseInlineHandlers(t)
	refuseLockedHandlers(t)
	refuseBrokenTrailers(t)
	refuseNullEncrypt(t)
}

// refuseInlineHandlers covers an /Encrypt dictionary written inline in the
// trailer, which one b08 file carries, and a reference to an object the table
// cannot resolve, which another carries. The refusal reads the trailer entry,
// so it never resolves the reference and never needs a key.
func refuseInlineHandlers(t *testing.T) {
	t.Helper()
	cases := []struct {
		name  string
		extra string
	}{
		{"bare dictionary", " /Encrypt << /Filter /Standard >>"},
		{"complete R3 dictionary", " /Encrypt " + r3Dictionary()},
		{"reference to an object the table does not carry", " /Encrypt 99 0 R"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Open(t.Context(), textPageTrailer(t, "q", testCase.extra))
			wantJob(t, err, opEncrypt, errAccess)
		})
	}
}

// refuseLockedHandlers covers every handler revision and every entry shape the
// b08 half of the batch2 corpus carries, all behind an indirect reference and
// a valid /ID.
func refuseLockedHandlers(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		dict string
	}{
		{"R2 dictionary", r2Dictionary()},
		{"R3 dictionary", r3Dictionary()},
		{"R4 dictionary with an AESV2 crypt filter", r4Dictionary()},
		{"R4 dictionary with an RC4 /V2 crypt filter", r4RC4Dictionary()},
		{"R4 dictionary with EncryptMetadata false", r4Dictionary() + " /EncryptMetadata false"},
		{"R5 dictionary with over-long O and U", r5OverlongDictionary()},
		{"R5 dictionary with EncryptMetadata true", r5MetadataDictionary()},
		{"R6 dictionary with an AESV3 crypt filter", r6Dictionary()},
		{"literal string O and U, revision 2", literalR2Dictionary()},
		{"literal string O and U, revision 3", literalR3Dictionary()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Open(t.Context(), lockedPage(t, testCase.dict))
			wantJob(t, err, opEncrypt, errAccess)
		})
	}
}

// refuseBrokenTrailers covers the three b08 trailers that are not well formed:
// two with no /ID at all and one whose /ID entries are empty strings. A key
// derivation would need /ID, but the refusal reads only the trailer entry, so
// a broken one still gets the same error as a conforming file.
func refuseBrokenTrailers(t *testing.T) {
	t.Helper()
	t.Run("trailer without an ID, revision 2", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPageNoID(t, r2Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("trailer without an ID, revision 3", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPageNoID(t, r3Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("empty ID entries", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPageWithTrailer(t, r2Dictionary(), " /ID [()()]"))
		wantJob(t, err, opEncrypt, errAccess)
	})
}

// refuseNullEncrypt is the neighbouring case that must not be refused: a null
// /Encrypt means the document is not encrypted and the file opens.
func refuseNullEncrypt(t *testing.T) {
	t.Helper()
	report, err := mustOpen(t, textPageTrailer(t, "q", " /Encrypt null")).Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if report.Pages != 1 {
		t.Fatalf("pages %d, want 1", report.Pages)
	}
}

// lockedPage builds the shared one-page document with the trailer /Encrypt
// naming object 5, which carries the security dictionary. The reference is
// indirect and the trailer keeps an /ID because that is how a writer emits it.
func lockedPage(t *testing.T, dict string) []byte {
	t.Helper()
	return lockedPageWithTrailer(t, dict, " /ID [<0011> <0011>]")
}

// lockedPageNoID is lockedPage without the /ID. Bucket b08 holds two files
// shaped like this, so the key derivation input is missing its last field. The
// refusal is read from the trailer entry, so it never needs one.
func lockedPageNoID(t *testing.T, dict string) []byte {
	t.Helper()
	return lockedPageWithTrailer(t, dict, "")
}

func lockedPageWithTrailer(t *testing.T, dict, trailerExtra string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object(pageBody)
	doc.object(streamBody("", []byte("q")))
	doc.object(dict)
	return doc.classic(trailerExtra + " /Encrypt 5 0 R")
}

// The dictionaries below are the Standard security handler revisions Spectre
// refuses. Their /O and /U are the fixed lengths the revisions require, so the
// shapes are the ones real files carry rather than a placeholder that any
// /Encrypt entry would satisfy.

func r2Dictionary() string {
	return "<< /Filter /Standard /V 1 /R 2 /Length 40 /P 65476 " +
		"/O <" + encryptZeroHex(32) + "> /U <" + encryptZeroHex(32) + "> >>"
}

func r3Dictionary() string {
	return "<< /Filter /Standard /V 2 /R 3 /Length 128 /P -3904 " +
		"/O <" + encryptZeroHex(32) + "> /U <" + encryptZeroHex(32) + "> >>"
}

func r4Dictionary() string {
	return "<< /Filter /Standard /V 4 /R 4 /Length 128 /P -20 " +
		"/CF << /StdCF << /CFM /AESV2 /AuthEvent /DocOpen /Length 16 >> >> " +
		"/StmF /StdCF /StrF /StdCF /O <" + encryptZeroHex(32) + "> /U <" + encryptZeroHex(32) + "> >>"
}

func r6Dictionary() string {
	return "<< /Filter /Standard /V 5 /R 6 /Length 256 /P -3904 " +
		"/CF << /StdCF << /CFM /AESV3 >> >> /StmF /StdCF /StrF /StdCF " +
		"/O <" + encryptZeroHex(48) + "> /U <" + encryptZeroHex(48) + "> >>"
}

// r4RC4Dictionary is the /V2 crypt filter six b08 files carry: revision 4
// selects a crypt filter, and that filter names plain RC4, not AES.
func r4RC4Dictionary() string {
	return "<< /Filter /Standard /V 4 /R 4 /Length 128 /P -1052 " +
		"/CF << /StdCF << /CFM /V2 /Length 16 >> >> " +
		"/StmF /StdCF /StrF /StdCF /O <" + encryptZeroHex(32) + "> /U <" + encryptZeroHex(32) + "> >>"
}

// r5OverlongDictionary is the shape of GHOSTSCRIPT-690702-0 and -2: an R5
// AESV3 handler whose /O and /U are 127 bytes, the 48 the revision needs
// followed by NUL padding. A lenient reader slices the tail off; the refusal
// does not look at either entry.
func r5OverlongDictionary() string {
	tail := strings.Repeat("00", 127)
	return "<< /Filter /Standard /V 5 /R 5 /Length 256 /P -300 " +
		"/CF << /StdCF << /CFM /AESV3 >> >> /StmF /StdCF /StrF /StdCF " +
		"/O <" + tail + "> /U <" + tail + "> >>"
}

// r5MetadataDictionary is the shape of GHOSTSCRIPT-692343-0, whose /P is
// positive and whose metadata is encrypted, so the key derivation covers the
// metadata in both cases.
func r5MetadataDictionary() string {
	return "<< /Filter /Standard /V 5 /R 5 /Length 256 /P 2147418400 /EncryptMetadata true " +
		"/CF << /StdCF << /CFM /AESV3 /Length 32 >> >> " +
		"/StmF /StdCF /StrF /StdCF /O <" + encryptZeroHex(48) + "> /U <" + encryptZeroHex(48) + "> >>"
}

// literalR2Dictionary and literalR3Dictionary write /O and /U as literal
// strings rather than hex strings, the shape 53 of the 59 b08 files carry.
// The refusal reads only the trailer entry, so the entries never parse in the
// current reader; the pin is for the shape itself, which the deferred
// decryption work has to read.
func literalR2Dictionary() string {
	return "<< /Filter /Standard /V 1 /R 2 /Length 40 /P -64 /O " +
		literalString(handlerBytes(0x28)) + " /U " + literalString(handlerBytes(0x50)) + " >>"
}

func literalR3Dictionary() string {
	return "<< /Filter /Standard /V 2 /R 3 /Length 128 /P -3904 /O " +
		literalString(handlerBytes(0x28)) + " /U " + literalString(handlerBytes(0x50)) + " >>"
}

// handlerBytes returns 32 bytes stepping from seed, so the literal reader
// meets parentheses and backslashes the way it does in a real /O or /U.
func handlerBytes(seed byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed + byte(i)
	}
	return out
}

// literalString renders raw as a literal string, escaping the three bytes a
// literal needs escaped. Every other byte goes through as itself, which is
// what a producer writes.
func literalString(raw []byte) string {
	var builder strings.Builder
	builder.WriteByte('(')
	for _, ch := range raw {
		switch ch {
		case '(', ')', '\\':
			builder.WriteByte('\\')
		}
		builder.WriteByte(ch)
	}
	builder.WriteByte(')')
	return builder.String()
}

func encryptZeroHex(n int) string {
	const zeros = "0000000000000000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000"
	return zeros[:n*2]
}
