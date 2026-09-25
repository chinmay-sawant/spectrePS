package font

// Width is a glyph advance in 1/1000 em units.
type Width int

// Metrics holds one font's advances, keyed by character code and by glyph
// name. Build one with Standard14. The zero value holds no advances.
type Metrics struct {
	name   string
	byName map[string]Width
	byCode map[byte]Width
}

// Name returns the PostScript name, for example "Helvetica-Bold".
func (m *Metrics) Name() string {
	if m == nil {
		return ""
	}
	return m.name
}

// WidthByCode returns the advance for a character code in the font's own
// encoding. The second result is false when the font has no glyph at the code.
func (m *Metrics) WidthByCode(code byte) (Width, bool) {
	if m == nil {
		return 0, false
	}
	width, ok := m.byCode[code]
	return width, ok
}

// WidthByName returns the advance for a glyph name, for example "eacute".
// The second result is false when the font has no such glyph. A name can have
// no code and still have an advance, which is what a PDF /Differences array
// needs.
func (m *Metrics) WidthByName(name string) (Width, bool) {
	if m == nil {
		return 0, false
	}
	width, ok := m.byName[name]
	return width, ok
}

// Standard14 returns the metrics for one of the standard 14 font names. The
// second result is false for any other name. Names are case-sensitive, so
// "helvetica" is not "Helvetica". These metrics are the fallback for a simple
// font with no /Widths array and no embedded font program.
func Standard14(name string) (*Metrics, bool) {
	metrics, ok := standard14Data[name]
	return metrics, ok
}

// Standard14Names returns the PostScript names of the standard 14 fonts in
// AFM order: the Courier family, the Helvetica family, the Times family,
// then Symbol and ZapfDingbats.
func Standard14Names() []string {
	return append([]string(nil), standard14Order[:]...)
}
