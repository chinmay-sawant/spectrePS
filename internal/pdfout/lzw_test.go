package pdfout

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	levelLZWNum    = 4
	levelBadLZWNum = 5
)

// TestLevelLZWImage proves level 2 re-encodes an LZW image as lossless Flate
// RGB, levels 3 through 5 as DCT, and an undecodable LZW stream copies
// through at every level.
func TestLevelLZWImage(t *testing.T) {
	t.Parallel()
	samples := levelSamples()
	good := lzwImageBody(t, samples)
	badDict := "/Type /XObject /Subtype /Image /Width 4 /Height 4 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /LZWDecode"
	file := levelImagePDF(t, good, levelStreamBody(badDict, []byte{0xff, 0xff, 0xff}))
	checkLZWLevels(t, file, samples)
}

func checkLZWLevels(t *testing.T, file *pdf.File, samples []byte) {
	t.Helper()
	for level := MinCompressionLevel; level <= MaxCompressionLevel; level++ {
		overrides, err := LevelOverrides(t.Context(), file, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if level == MinCompressionLevel {
			if len(overrides) != 0 {
				t.Fatalf("level %d overrides %v", level, overrides)
			}
			continue
		}
		checkLZWOverride(t, file, overrides, level, samples)
	}
}

func checkLZWOverride(t *testing.T, file *pdf.File, overrides map[int][]byte, level int, samples []byte) {
	t.Helper()
	if _, ok := overrides[levelLZWNum]; !ok {
		t.Fatalf("level %d: LZW image has no override", level)
	}
	if _, ok := overrides[levelBadLZWNum]; ok {
		t.Fatalf("level %d: undecodable LZW image has an override", level)
	}
	reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
	pic, err := reopened.DecodeImage(levelLZWNum)
	if err != nil {
		t.Fatalf("level %d: %v", level, err)
	}
	if level == levelFlateImages {
		checkLevelFilter(t, reopened, levelLZWNum, filterFlate)
		checkLevelPixels(t, pic, samples)
	} else {
		if pic.Bounds().Dx() != 4 || pic.Bounds().Dy() != 4 {
			t.Fatalf("level %d: bounds %v", level, pic.Bounds())
		}
		checkLevelFilter(t, reopened, levelLZWNum, filterDCT)
	}
	checkLevelFilter(t, reopened, levelBadLZWNum, filterLZW)
}

// lzwImageBody wraps 8-bit RGB samples in an LZW image stream.
func lzwImageBody(t *testing.T, samples []byte) string {
	t.Helper()
	dict := "/Type /XObject /Subtype /Image /Width 4 /Height 4 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /LZWDecode"
	return levelStreamBody(dict, lzwImageBytes(t, samples, 1))
}

// lzwImageBytes writes PDF LZW with the given EarlyChange setting. It is a
// test encoder, not a production one.
func lzwImageBytes(t *testing.T, plain []byte, early int) []byte {
	t.Helper()
	var out lzwTestWriter
	table := map[string]int{}
	reset := func() {
		clear(table)
		for i := range 256 {
			table[string([]byte{byte(i)})] = i
		}
	}
	reset()
	next := 258
	width := 9
	out.write(256, width)
	add := func(key string) {
		if next > 4095 {
			out.write(256, width)
			reset()
			next = 258
			width = 9
			return
		}
		table[key] = next
		next++
		if width < 12 && next >= (1<<width)-early+1 {
			width++
		}
	}
	prev := ""
	for _, cur := range plain {
		candidate := prev + string([]byte{cur})
		if _, ok := table[candidate]; ok {
			prev = candidate
			continue
		}
		out.write(table[prev], width)
		add(candidate)
		prev = string([]byte{cur})
	}
	if prev != "" {
		out.write(table[prev], width)
	}
	out.write(257, width)
	return out.bytes()
}

// lzwTestWriter packs LZW codes MSB first.
type lzwTestWriter struct {
	buf   []byte
	bits  byte
	count int
}

func (writer *lzwTestWriter) write(code, width int) {
	for i := width - 1; i >= 0; i-- {
		writer.bits <<= 1
		if code>>i&1 == 1 {
			writer.bits |= 1
		}
		writer.count++
		if writer.count == 8 {
			writer.buf = append(writer.buf, writer.bits)
			writer.bits = 0
			writer.count = 0
		}
	}
}

func (writer *lzwTestWriter) bytes() []byte {
	if writer.count > 0 {
		writer.buf = append(writer.buf, writer.bits<<(8-writer.count))
	}
	return writer.buf
}
