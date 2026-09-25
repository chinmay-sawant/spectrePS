package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
)

// TestType1TextCommand proves `spectreps text` on a fixture that embeds the
// synthetic Type 1 font. The font is symbolic with no /ToUnicode, so the
// command prints the built-in AGL names and the code-point fallback.
func TestType1TextCommand(t *testing.T) {
	path := writeTemp(t, "type1.pdf", type1Fixture(t))
	want(t, []string{"text", path}, 0, "ABZ\r\n", "")
}

// type1Fixture builds the one-page PDF with the synthetic /FontFile program.
func type1Fixture(t *testing.T) []byte {
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
		type1Stream(t, program, lengths),
	}
	return classicXref(t, objects)
}

// type1Stream wraps one program with the /Length1-3 entries.
func type1Stream(t *testing.T, program []byte, lengths [3]int) []byte {
	t.Helper()
	compressed := flateBytes(t, program)
	var body bytes.Buffer
	fmt.Fprintf(&body, "<< /Length %d /Filter /FlateDecode /Length1 %d /Length2 %d /Length3 %d >>\nstream\n",
		len(compressed), lengths[0], lengths[1], lengths[2])
	body.Write(compressed)
	body.WriteString("\nendstream")
	return body.Bytes()
}
