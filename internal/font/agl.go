package font

// AGLUnicode returns the Unicode string for a glyph name from the Adobe Glyph
// List 2.0, with the ZapfDingbats names merged in. Some names map to more than
// one code point, so the result can contain several runes. The second result
// is false when the list does not name the glyph.
func AGLUnicode(name string) (string, bool) {
	value, ok := aglNames[name]
	return value, ok
}
