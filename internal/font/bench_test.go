package font

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/truetypesynth"
	"github.com/chinmay-sawant/spectrePS/internal/type1synth"
	"golang.org/x/image/font/sfnt"
)

// The package had no benchmark before this file. The lookups here are the ones
// a text job runs per shown code, and Subset is what RewritePDF pays with
// SubsetFonts set. A map literal with more than 25 entries compiles to a
// generated init loop, so the standard 14 tables in widths_data.go and
// codes_data.go are built before any timer in this file starts. That cost is
// not measurable from inside the package and documentation/performance.md
// records it from the application side instead.

// BenchmarkStandard14Lookup resolves every standard 14 name and then runs both advance
// lookups on each resolved metrics, so the font map hit and the two advance
// map hits are all in the timed loop. The advance lookups do not assert a
// result, because Symbol and ZapfDingbats have no A glyph and the benchmark
// measures the map hit either way.
func BenchmarkStandard14Lookup(b *testing.B) {
	names := Standard14Names()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, name := range names {
			metrics, ok := Standard14(name)
			if !ok {
				b.Fatalf("Standard14(%q) is absent", name)
			}
			_, _ = metrics.WidthByName("A")
			_, _ = metrics.WidthByCode('A')
		}
	}
}

// BenchmarkEncodingLookup walks all 256 codes of every encoding through
// GlyphName and back through GlyphCode, the pair the text job runs per code.
// The reverse lookup is exercised for its cost and its result is not asserted,
// because a code with no glyph in that encoding has no reverse entry.
func BenchmarkEncodingLookup(b *testing.B) {
	encodings := []Encoding{EncodingStandard, EncodingWinAnsi, EncodingMacRoman}
	reverse := make([][]byte, len(encodings))
	for idx := range reverse {
		reverse[idx] = make([]byte, 256)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for idx, encoding := range encodings {
			for code := range 256 {
				name := encoding.GlyphName(byte(code))
				if back, ok := encoding.GlyphCode(name); ok {
					reverse[idx][code] = back
				}
			}
		}
	}
}

// BenchmarkAGLUnicode resolves a spread of glyph names through the aglNames
// table in agl_data.go. GlyphCode builds the reverse map on first use, so the
// first iteration is excluded by the warm-up below.
func BenchmarkAGLUnicode(b *testing.B) {
	names := Standard14Names()
	probes := []string{"A", "a", "eacute", "adieresis", "bullet", "quoteright", "zero", "space"}
	warm := map[string]struct{}{}
	for _, name := range names {
		warm[name] = struct{}{}
	}
	for _, probe := range probes {
		if _, ok := AGLUnicode(probe); ok {
			warm[probe] = struct{}{}
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, probe := range probes {
			if _, ok := AGLUnicode(probe); !ok {
				b.Fatalf("AGLUnicode(%q) is absent", probe)
			}
		}
	}
}

// BenchmarkLoadType1 splits the Type 1 program parse from the charstring
// interpret, because a text job pays the parse once per font and the interpret
// once per glyph.
func BenchmarkLoadType1(b *testing.B) {
	data, lengths := type1synth.Program(type1synth.Options{})
	b.Run("parse", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := LoadType1(data, lengths); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("glyph", func(b *testing.B) {
		program, err := LoadType1(data, lengths)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := program.Glyph(type1synth.GlyphA); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSubset subsets the synthetic TrueType program, which is what
// RewritePDF pays with SubsetFonts set. The tag is a separate line because it
// only hashes the table directory.
func BenchmarkSubset(b *testing.B) {
	program := truetypesynth.Program()
	keep := []sfnt.GlyphIndex{0, 1, 2, 3}
	b.Run("subset", func(b *testing.B) {
		b.SetBytes(int64(len(program)))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := Subset(program, keep); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("tag", func(b *testing.B) {
		b.SetBytes(int64(len(program)))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = SubsetTag(program)
		}
	})
}
