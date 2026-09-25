package font

import "strconv"

// Encoding names one of the simple-font encodings in ISO 32000-1 Annex D.2.
type Encoding uint8

const (
	// EncodingStandard is Adobe StandardEncoding, the default for a Type 1
	// font without an /Encoding entry.
	EncodingStandard Encoding = iota
	// EncodingWinAnsi is WinAnsiEncoding, the Windows code page 1252 set.
	EncodingWinAnsi
	// EncodingMacRoman is MacRomanEncoding, the Mac OS Roman set.
	EncodingMacRoman
)

// String returns the PDF encoding name without the leading slash, or
// "Encoding(n)" for a value outside the three tables.
func (e Encoding) String() string {
	switch e {
	case EncodingStandard:
		return "StandardEncoding"
	case EncodingWinAnsi:
		return "WinAnsiEncoding"
	case EncodingMacRoman:
		return "MacRomanEncoding"
	default:
		return "Encoding(" + strconv.Itoa(int(e)) + ")"
	}
}

// table returns the package table for the encoding, or nil for an unknown
// value.
func (e Encoding) table() *[256]string {
	switch e {
	case EncodingStandard:
		return &standardEncodingNames
	case EncodingWinAnsi:
		return &winAnsiEncodingNames
	case EncodingMacRoman:
		return &macRomanEncodingNames
	default:
		return nil
	}
}

// GlyphName returns the glyph name at a character code, or "" when the
// encoding leaves the code undefined.
func (e Encoding) GlyphName(code byte) string {
	table := e.table()
	if table == nil {
		return ""
	}
	return table[code]
}

// GlyphCode returns the first code assigned to a glyph name. The second result
// is false when the encoding has no such name. Some encodings assign a name to
// more than one code, for example "space" in WinAnsiEncoding; GlyphName of the
// returned code gives the name back.
func (e Encoding) GlyphCode(name string) (byte, bool) {
	table := e.table()
	if table == nil || name == "" {
		return 0, false
	}
	for code, got := range table {
		if got == name {
			return byte(code), true
		}
	}
	return 0, false
}

// GlyphNames returns a copy of the whole 256-entry code-to-name table. An
// unknown encoding returns an empty table.
func (e Encoding) GlyphNames() [256]string {
	table := e.table()
	if table == nil {
		return [256]string{}
	}
	return *table
}
