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

import "testing"

// TestPDFEncryptRefusal proves a trailer /Encrypt is refused with the Encrypt
// op whatever shape the security dictionary takes.
func TestPDFEncryptRefusal(t *testing.T) {
	t.Run("inline dictionary", func(t *testing.T) {
		src := textPageTrailer(t, "q", " /Encrypt << /Filter /Standard >>")
		_, err := Open(t.Context(), src)
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("indirect reference to an R2 dictionary", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPage(t, r2Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("R3 dictionary", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPage(t, r3Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("R4 dictionary with an AESV2 crypt filter", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPage(t, r4Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("R6 dictionary with an AESV3 crypt filter", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPage(t, r6Dictionary()))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("EncryptMetadata false changes nothing", func(t *testing.T) {
		_, err := Open(t.Context(), lockedPage(t, r4Dictionary()+" /EncryptMetadata false"))
		wantJob(t, err, opEncrypt, errAccess)
	})
	t.Run("a null Encrypt is not encrypted", func(t *testing.T) {
		report, err := mustOpen(t, textPageTrailer(t, "q", " /Encrypt null")).Info()
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		if report.Pages != 1 {
			t.Fatalf("pages %d, want 1", report.Pages)
		}
	})
}

// lockedPage builds the shared one-page document with the trailer /Encrypt
// naming object 5, which carries the security dictionary. The reference is
// indirect and the trailer keeps an /ID because that is how a writer emits it.
func lockedPage(t *testing.T, dict string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object(pageBody)
	doc.object(streamBody("", []byte("q")))
	doc.object(dict)
	return doc.classic(" /ID [<0011> <0011>] /Encrypt 5 0 R")
}

// The four dictionaries below are the Standard security handler revisions
// Spectre refuses. Their /O and /U are the fixed lengths the revisions
// require, so the shapes are the ones real files carry rather than a
// placeholder that any /Encrypt entry would satisfy.

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

func encryptZeroHex(n int) string {
	const zeros = "0000000000000000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000"
	return zeros[:n*2]
}
