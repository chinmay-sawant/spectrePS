package font

import "testing"

func TestStandard14Widths(t *testing.T) {
	t.Parallel()
	checkStandard14Names(t)
	checkStandard14Anchors(t)
	checkCourierAdvances(t)
}

func checkStandard14Names(t *testing.T) {
	t.Helper()
	names := Standard14Names()
	if len(names) != 14 {
		t.Fatalf("Standard14Names has %d names, want 14", len(names))
	}
	for _, name := range names {
		metrics, ok := Standard14(name)
		if !ok {
			t.Fatalf("Standard14(%q) missing", name)
		}
		if metrics.Name() != name {
			t.Fatalf("Name()=%q, want %q", metrics.Name(), name)
		}
		if len(metrics.byName) == 0 || len(metrics.byCode) == 0 {
			t.Fatalf("%s has no advances", name)
		}
	}
	if _, ok := Standard14("Arial"); ok {
		t.Fatal("Arial is not a standard 14 name")
	}
	if _, ok := Standard14("helvetica"); ok {
		t.Fatal("standard 14 lookup must be case-sensitive")
	}
}

func checkStandard14Anchors(t *testing.T) {
	t.Helper()
	const (
		helveticaName = "Helvetica"
		spaceGlyph    = "space"
	)
	byName := []struct {
		font  string
		glyph string
		want  Width
	}{
		{helveticaName, "A", 667},
		{helveticaName, spaceGlyph, 278},
		{helveticaName, "Aring", 667},
		{"Helvetica-Bold", "A", 722},
		{"Times-Roman", "A", 722},
		{"Times-Italic", "A", 611},
		{"Courier", "A", 600},
		{"Courier-BoldOblique", "i", 600},
		{"Symbol", spaceGlyph, 250},
		{"Symbol", "alpha", 631},
		{"ZapfDingbats", "a1", 974},
	}
	for _, anchor := range byName {
		metrics, ok := Standard14(anchor.font)
		if !ok {
			t.Fatalf("Standard14(%q) missing", anchor.font)
		}
		got, ok := metrics.WidthByName(anchor.glyph)
		if !ok || got != anchor.want {
			t.Fatalf("%s %s = %d (present %t), want %d", anchor.font, anchor.glyph, got, ok, anchor.want)
		}
	}

	byCode := []struct {
		font string
		code byte
		want Width
	}{
		{helveticaName, 'A', 667},
		{helveticaName, ' ', 278},
		{helveticaName, 193, 333},
		{"Courier", 'W', 600},
		{"Times-Roman", ' ', 250},
		{"Symbol", 97, 631},
		{"ZapfDingbats", 33, 974},
	}
	for _, anchor := range byCode {
		metrics, ok := Standard14(anchor.font)
		if !ok {
			t.Fatalf("Standard14(%q) missing", anchor.font)
		}
		got, ok := metrics.WidthByCode(anchor.code)
		if !ok || got != anchor.want {
			t.Fatalf("%s code %d = %d (present %t), want %d", anchor.font, anchor.code, got, ok, anchor.want)
		}
	}
}

func checkCourierAdvances(t *testing.T) {
	t.Helper()
	courier, ok := Standard14("Courier")
	if !ok {
		t.Fatal("Standard14(Courier) missing")
	}
	for glyph, width := range courier.byName {
		if width != 600 {
			t.Fatalf("Courier %s = %d, want 600", glyph, width)
		}
	}
	for code, width := range courier.byCode {
		if width != 600 {
			t.Fatalf("Courier code %d = %d, want 600", code, width)
		}
	}
	if _, ok := courier.WidthByName("nosuchglyph"); ok {
		t.Fatal("Courier has a glyph named nosuchglyph")
	}
	if _, ok := courier.WidthByCode(0); ok {
		t.Fatal("Courier has a glyph at code 0")
	}
}

func TestEncodingTables(t *testing.T) {
	t.Parallel()
	checkEncodingAnchors(t)
	checkEncodingRoundTrips(t)
	checkUnknownEncoding(t)
}

func checkEncodingAnchors(t *testing.T) {
	t.Helper()
	const spaceGlyph = "space"
	cases := []struct {
		encoding Encoding
		code     byte
		glyph    string
	}{
		{EncodingStandard, 0x20, spaceGlyph},
		{EncodingStandard, 0x27, "quoteright"},
		{EncodingStandard, 0x41, "A"},
		{EncodingStandard, 0xA1, "exclamdown"},
		{EncodingStandard, 0xFB, "germandbls"},
		{EncodingWinAnsi, 0x80, "Euro"},
		{EncodingWinAnsi, 0x93, "quotedblleft"},
		{EncodingWinAnsi, 0xA0, spaceGlyph},
		{EncodingWinAnsi, 0xAD, "hyphen"},
		{EncodingWinAnsi, 0xE9, "eacute"},
		{EncodingMacRoman, 0x80, "Adieresis"},
		{EncodingMacRoman, 0x8E, "eacute"},
		{EncodingMacRoman, 0xAD, "notequal"},
		{EncodingMacRoman, 0xCA, spaceGlyph},
		{EncodingMacRoman, 0xF0, "apple"},
	}
	for _, testCase := range cases {
		if got := testCase.encoding.GlyphName(testCase.code); got != testCase.glyph {
			t.Fatalf("%s code %#x = %q, want %q", testCase.encoding, testCase.code, got, testCase.glyph)
		}
	}
}

func checkEncodingRoundTrips(t *testing.T) {
	t.Helper()
	encodings := []Encoding{EncodingStandard, EncodingWinAnsi, EncodingMacRoman}
	for _, encoding := range encodings {
		for code := range 256 {
			glyph := encoding.GlyphName(byte(code))
			if glyph == "" {
				continue
			}
			got, ok := encoding.GlyphCode(glyph)
			if !ok || encoding.GlyphName(got) != glyph {
				t.Fatalf("%s code %d glyph %s does not round trip", encoding, code, glyph)
			}
		}
		if got := encoding.GlyphName(0); got != "" {
			t.Fatalf("%s code 0 = %q, want empty", encoding, got)
		}
	}
}

func checkUnknownEncoding(t *testing.T) {
	t.Helper()
	if got := Encoding(9).GlyphName(0x41); got != "" {
		t.Fatalf("unknown encoding code 0x41 = %q, want empty", got)
	}
	if _, ok := Encoding(9).GlyphCode("A"); ok {
		t.Fatal("unknown encoding has glyph A")
	}
	if got := Encoding(9).String(); got != "Encoding(9)" {
		t.Fatalf("unknown encoding String() = %q", got)
	}
	if got := EncodingWinAnsi.String(); got != "WinAnsiEncoding" {
		t.Fatalf("WinAnsi String() = %q", got)
	}
	names := EncodingWinAnsi.GlyphNames()
	if names[0x80] != "Euro" || names[0xE9] != "eacute" {
		t.Fatalf("GlyphNames copy = %q at 0x80, %q at 0xE9", names[0x80], names[0xE9])
	}
}

func TestAGLNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		glyph string
		want  string
	}{
		{"A", "A"},
		{"space", " "},
		{"eacute", "\u00e9"},
		{"Euro", "\u20ac"},
		{"notequal", "\u2260"},
		{"dalethatafpatah", "\u05d3\u05b2"},
		{"a1", "\u2701"},
		{"a202", "\u2703"},
	}
	for _, testCase := range cases {
		got, ok := AGLUnicode(testCase.glyph)
		if !ok || got != testCase.want {
			t.Fatalf("AGLUnicode(%q) = %q (present %t), want %q", testCase.glyph, got, ok, testCase.want)
		}
	}
	for _, glyph := range []string{"nosuchglyph", ""} {
		if got, ok := AGLUnicode(glyph); ok {
			t.Fatalf("AGLUnicode(%q) = %q, want absent", glyph, got)
		}
	}
}
