package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// type1TextFixture is the checked-in PDF with a synthetic /FontFile Type 1
// program. The generator is internal/type1synth; the fixture is pinned so
// internal/cli keeps the import boundary that TestValidationNoProcess locks.
const type1TextFixture = "type1-text.pdf"

// type1TextSHA256 is the pinned digest of sampledata/fixtures/type1-text.pdf.
const type1TextSHA256 = "23749b69f83db2b1a646757edbcebb9e79e878adb25b8b8b446abdea569b089e"

// TestType1TextCommand proves `spectreps text` on a fixture that embeds the
// synthetic Type 1 font. The font is symbolic with no /ToUnicode, so the
// command prints the built-in AGL names and the code-point fallback.
func TestType1TextCommand(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "sampledata", "fixtures", type1TextFixture))
	if err != nil {
		t.Fatalf("read %s: %v", type1TextFixture, err)
	}
	if got := sha256Hex(raw); got != type1TextSHA256 {
		t.Fatalf("%s sha256 = %s, want %s", type1TextFixture, got, type1TextSHA256)
	}
	path := writeTemp(t, "type1.pdf", raw)
	want(t, []string{"text", path}, 0, "ABZ\r\n", "")
}

// sha256Hex returns the lowercase hex digest of raw.
func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
