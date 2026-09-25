package spectreps_test

import (
	"bytes"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
)

// TestType1Extract proves that File.ExtractText prints the built-in AGL names
// of a synthetic Type 1 font and falls back to the code point when the
// built-in encoding leaves a code unnamed. The fixture is built in Go, so no
// font bytes are checked in.
func TestType1Extract(t *testing.T) {
	in := newInst(t)
	doc, err := in.OpenPDF(t.Context(), type1PDF(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := in.ExtractText(t.Context(), doc, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ABZ\r\n" {
		t.Fatalf("ExtractText = %q, want %q", got, "ABZ\r\n")
	}
}

// type1PDF embeds the synthetic Type 1 font as /FontFile. The font is
// symbolic and carries no /ToUnicode, so extraction reads the built-in
// encoding: A and B through the Adobe Glyph List and Z from the code point.
func type1PDF(t *testing.T) []byte {
	t.Helper()
	program, lengths := type1synth.Program(type1synth.Options{LenIV: 4})
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> >> >>"),
		flateStream(t, "BT /F1 12 Tf 10 20 Td (ABZ) Tj ET"),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /SynthType1 /FontDescriptor 6 0 R >>"),
		[]byte("<< /Type /FontDescriptor /FontName /SynthType1 /Flags 4 /FontFile 7 0 R >>"),
		type1FontObject(t, program, lengths),
	}
	return classicXref(t, objects)
}

// type1FontObject wraps the program in a Flate stream that carries the
// /Length1-3 entries.
func type1FontObject(t *testing.T, program []byte, lengths [3]int) []byte {
	t.Helper()
	compressed := flateBytes(t, program)
	var body bytes.Buffer
	writef(t, &body, "<< /Length %d /Filter /FlateDecode /Length1 %d /Length2 %d /Length3 %d >>\nstream\n",
		len(compressed), lengths[0], lengths[1], lengths[2])
	writeAll(t, &body, compressed)
	writeString(t, &body, "\nendstream")
	return body.Bytes()
}
