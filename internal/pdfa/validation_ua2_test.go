package pdfa

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestValidationUA2Edges locks the metadata reader edges and the context
// contract: a Flate /Metadata stream reads, dc:title escapes &, <, and >, the
// BCP 47 syntax boundaries are the chosen behavior, a canceled context returns
// ctx.Err(), and a nil context panics.
func TestValidationUA2Edges(t *testing.T) {
	t.Run("flate metadata", checkUA2FlateMetadata)
	t.Run("other filter", checkUA2OtherFilter)
	t.Run("title escapes", checkUA2TitleEscapes)
	t.Run("bcp 47", checkUA2LangEdges)
	t.Run("canceled context", checkUA2CanceledContext)
	t.Run("nil context", checkUA2NilContext)
}

// checkUA2FlateMetadata proves a Flate metadata stream reads whether /Filter
// is a name or a one-name array.
func checkUA2FlateMetadata(t *testing.T) {
	t.Helper()
	packet := UA2XMP(UA2Metadata{
		Title: "Compressed title",
		Lang:  "en-US",
		Part:  ua2PartValue,
		Rev:   ua2RevValue,
	})
	for _, filter := range []string{"/FlateDecode", "[/FlateDecode]"} {
		t.Run(filter, func(t *testing.T) {
			file := validationUA2MetadataFile(t, filter, validationFlate(t, packet))
			info := mustReadUA2(t, file)
			if !info.Metadata || info.Part != ua2PartValue || info.Rev != ua2RevValue {
				t.Fatalf("claim = %+v", info)
			}
			if info.Title != "Compressed title" || info.Lang != "en-US" {
				t.Fatalf("text = %+v", info)
			}
			wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
		})
	}
}

// checkUA2OtherFilter proves a filter other than Flate yields no packet text,
// so the missing dc:title fails ua2-title.
func checkUA2OtherFilter(t *testing.T) {
	t.Helper()
	file := validationUA2MetadataFile(t, "/ASCIIHexDecode", []byte("<x>"))
	info := mustReadUA2(t, file)
	if !info.Metadata {
		t.Fatal("Metadata = false")
	}
	if info.Part != "" || info.Rev != "" || info.Title != "" {
		t.Fatalf("other filter read a packet: %+v", info)
	}
	wantUA2Rule(t, PreflightUA2(t.Context(), file), ruleUA2Title)
}

// checkUA2TitleEscapes proves the packet escapes &, <, and >, and that the
// reader returns the original title, including a literal &lt;.
func checkUA2TitleEscapes(t *testing.T) {
	t.Helper()
	title := `A&B <C> "D" 'E' &lt;`
	packet := string(UA2XMP(UA2Metadata{
		Title: title,
		Part:  ua2PartValue,
		Rev:   ua2RevValue,
	}))
	for _, needle := range []string{"A&amp;B", "&lt;C&gt;", "&amp;lt;"} {
		if !strings.Contains(packet, needle) {
			t.Fatalf("packet lacks %q", needle)
		}
	}
	if strings.Contains(packet, "<C>") {
		t.Fatal("packet carries the raw title")
	}
	file := ua2File(t, ua2With(func(fix *ua2Fixture) { fix.title = title }))
	info := mustReadUA2(t, file)
	if info.Title != title {
		t.Fatalf("Title = %q, want %q", info.Title, title)
	}
}

// checkUA2LangEdges locks the BCP 47 syntax boundaries: two or three letters
// or the i and x tags, then one-to-eight alphanumeric subtags. A bare i or x
// is accepted, the chosen simplification next to primaryTagOK.
func checkUA2LangEdges(t *testing.T) {
	t.Helper()
	valid := []string{
		"en", "EN", "fil", "en-US", "de-CH-1901", "sl-rozaj-biske-1994",
		"x-private", "i-klingon", "en-x-custom", "en-abcdefgh",
		"x", "i", "en-1", "en-12345678",
	}
	for _, lang := range valid {
		if !langSyntaxOK(lang) {
			t.Errorf("langSyntaxOK(%q) = false, want true", lang)
		}
	}
	invalid := []string{
		"", "e", "engl", "english_us", "123", "en-", "-en", "en-US-",
		"en-abcdefghi", "en-U_S", "en--US",
	}
	for _, lang := range invalid {
		if langSyntaxOK(lang) {
			t.Errorf("langSyntaxOK(%q) = true, want false", lang)
		}
	}
	file := ua2File(t, ua2With(func(fix *ua2Fixture) { fix.lang = "de-CH-1901" }))
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
	file = ua2File(t, ua2With(func(fix *ua2Fixture) { fix.lang = "en-" }))
	wantUA2Rule(t, PreflightUA2(t.Context(), file), ruleUA2Lang)
}

// checkUA2CanceledContext proves a canceled context returns ctx.Err() before
// any check runs.
func checkUA2CanceledContext(t *testing.T) {
	t.Helper()
	file := ua2File(t, defaultUA2Fixture())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := PreflightUA2(ctx, file); !errors.Is(err, context.Canceled) {
		t.Fatalf("PreflightUA2() = %v, want context.Canceled", err)
	}
}

// checkUA2NilContext proves a nil context panics with the package message.
func checkUA2NilContext(t *testing.T) {
	t.Helper()
	file := ua2File(t, defaultUA2Fixture())
	defer func() {
		if recovered := recover(); recovered != pdfaNilContextPanic {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_ = PreflightUA2(nil, file) //nolint:staticcheck // nil context is the case under test
}

// validationUA2MetadataFile builds the tagged fixture with one metadata stream
// shape. The filter is written as given, and an empty filter writes none.
func validationUA2MetadataFile(t *testing.T, filter string, body []byte) *pdf.File {
	t.Helper()
	fix := defaultUA2Fixture()
	objects := []string{
		ua2CatalogBody(fix),
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << >> /StructParents 0 >>",
		ua2Stream("/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC"),
		"<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 8 0 R /ParentTreeNextKey 1 >>",
		ua2DocumentBody(fix),
		"<< /Type /StructElem /S /Figure /Alt (A figure) /P 6 0 R /Pg 3 0 R /K 0 >>",
		"<< /Nums [" + fix.parentNums + "] >>",
		validationMetadataStream(filter, body),
		"<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>",
	}
	src := buildClassicPDF(t, objects)
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// validationMetadataStream writes one object 9 stream body.
func validationMetadataStream(filter string, body []byte) string {
	head := fmt.Sprintf("<< /Length %d", len(body))
	if filter != "" {
		head += " /Filter " + filter
	}
	return head + " >>\nstream\n" + string(body) + "\nendstream"
}

// validationFlate compresses one payload with zlib.
func validationFlate(t *testing.T, plain []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zlib.NewWriter(&body)
	if _, err := writer.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
