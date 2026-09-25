package font

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/truetypesynth"
	"golang.org/x/image/font/sfnt"
)

// TestSubsetTrueTypeTables proves the table reader and writer round-trip a
// synthetic TrueType program: the directory sorts by tag, table checksums are
// recomputed, head carries a completing checkSumAdjustment, and sfnt.Parse
// accepts the result.
func TestSubsetTrueTypeTables(t *testing.T) {
	t.Parallel()
	program := truetypesynth.Program()
	checkSfntParse(t, program)
	checkTableDirectory(t, program)
	checkTableRoundTrip(t, program)
	checkWholeChecksum(t, program)
}

func checkSfntParse(t *testing.T, program []byte) {
	t.Helper()
	parsed, err := sfnt.Parse(program)
	if err != nil {
		t.Fatalf("sfnt.Parse(source) = %v", err)
	}
	if got := parsed.NumGlyphs(); got != truetypesynth.NumGlyphs {
		t.Fatalf("NumGlyphs = %d, want %d", got, truetypesynth.NumGlyphs)
	}
}

func checkTableDirectory(t *testing.T, program []byte) {
	t.Helper()
	tables, err := ParseTables(program)
	if err != nil {
		t.Fatalf("ParseTables = %v", err)
	}
	if len(tables) != 8 {
		t.Fatalf("tables = %d, want 8", len(tables))
	}
	for idx := 1; idx < len(tables); idx++ {
		if tables[idx-1].Tag >= tables[idx].Tag {
			t.Fatalf("directory not sorted at %d: %q then %q", idx, tables[idx-1].Tag, tables[idx].Tag)
		}
	}
	glyf, ok := TableData(tables, "glyf")
	if !ok || len(glyf) == 0 {
		t.Fatal("glyf table is missing")
	}
	if _, err := ParseTables(program[:10]); err == nil {
		t.Fatal("truncated program parsed")
	}
}

// checkTableRoundTrip writes and re-reads the tables and proves the writer
// leaves the caller's program alone.
func checkTableRoundTrip(t *testing.T, program []byte) {
	t.Helper()
	before := bytes.Clone(program)
	tables, err := ParseTables(program)
	if err != nil {
		t.Fatal(err)
	}
	out, err := WriteTables(tables)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(program, before) {
		t.Fatal("WriteTables changed the caller's program")
	}
	if _, err := sfnt.Parse(out); err != nil {
		t.Fatalf("sfnt.Parse(WriteTables) = %v", err)
	}
	again, err := ParseTables(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(tables) {
		t.Fatalf("tables = %d, want %d", len(again), len(tables))
	}
	for idx := range tables {
		if again[idx].Tag != tables[idx].Tag || !bytes.Equal(again[idx].Data, tables[idx].Data) {
			t.Fatalf("table %d round trip: %s", idx, tables[idx].Tag)
		}
	}
}

// checkWholeChecksum walks the written directory and proves every table
// checksum and the head adjustment are the sfnt values.
func checkWholeChecksum(t *testing.T, program []byte) {
	t.Helper()
	tables, err := ParseTables(program)
	if err != nil {
		t.Fatal(err)
	}
	out, err := WriteTables(tables)
	if err != nil {
		t.Fatal(err)
	}
	count := int(binary.BigEndian.Uint16(out[4:]))
	for idx := range count {
		entry := out[12+16*idx:]
		offset := int(binary.BigEndian.Uint32(entry[8:]))
		length := int(binary.BigEndian.Uint32(entry[12:]))
		data := out[offset : offset+length]
		checksum := binary.BigEndian.Uint32(entry[4:])
		if string(entry[0:4]) == "head" {
			clone := bytes.Clone(data)
			binary.BigEndian.PutUint32(clone[8:], 0)
			data = clone
		}
		if got := Checksum(data); got != checksum {
			t.Fatalf("table %s checksum = %#x, want %#x", entry[0:4], checksum, got)
		}
	}
	if got := Checksum(out); got != 0xB1B0AFBA {
		t.Fatalf("whole font checksum = %#x, want 0xB1B0AFBA", got)
	}
}

// TestEmbedSubsetTrueType proves the stable-GID subset zeroes an unused glyph
// and keeps the used outline, the cmap, the hmtx, and maxp valid.
func TestEmbedSubsetTrueType(t *testing.T) {
	t.Parallel()
	program := truetypesynth.Program()
	subset, err := Subset(program, []sfnt.GlyphIndex{truetypesynth.GIDA})
	if err != nil {
		t.Fatalf("Subset = %v", err)
	}
	sub, err := sfnt.Parse(subset)
	if err != nil {
		t.Fatalf("sfnt.Parse(subset) = %v", err)
	}
	if got := sub.NumGlyphs(); got != truetypesynth.NumGlyphs {
		t.Fatalf("NumGlyphs = %d, want %d", got, truetypesynth.NumGlyphs)
	}
	checkSameSegments(t, program, subset, truetypesynth.GIDA)
	checkEmptyGlyph(t, subset, truetypesynth.GIDB)
	checkEmptyGlyph(t, subset, truetypesynth.GIDC)
	checkCopiedTables(t, program, subset)
}

// checkCopiedTables proves cmap, hmtx, and maxp survive byte for byte, and
// head changes only its checkSumAdjustment because the short loca stays.
func checkCopiedTables(t *testing.T, program, subset []byte) {
	t.Helper()
	before, err := ParseTables(program)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseTables(subset)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"cmap", "hmtx", "maxp", "post"} {
		want, _ := TableData(before, tag)
		got, _ := TableData(after, tag)
		if !bytes.Equal(want, got) {
			t.Fatalf("%s changed", tag)
		}
	}
	wantHead, _ := TableData(before, "head")
	gotHead, _ := TableData(after, "head")
	if len(wantHead) != len(gotHead) {
		t.Fatalf("head length = %d, want %d", len(gotHead), len(wantHead))
	}
	if !bytes.Equal(wantHead[:8], gotHead[:8]) || !bytes.Equal(wantHead[12:], gotHead[12:]) {
		t.Fatal("head changed outside checkSumAdjustment")
	}
	glyf, _ := TableData(before, "glyf")
	subGlyf, _ := TableData(after, "glyf")
	if len(subGlyf) >= len(glyf) {
		t.Fatalf("glyf = %d bytes, want fewer than %d", len(subGlyf), len(glyf))
	}
}

// TestEmbedSubsetComposite proves a kept composite glyph carries its
// components, transitively.
func TestEmbedSubsetComposite(t *testing.T) {
	t.Parallel()
	program := truetypesynth.Program()
	subset, err := Subset(program, []sfnt.GlyphIndex{truetypesynth.GIDC})
	if err != nil {
		t.Fatalf("Subset = %v", err)
	}
	if _, err := sfnt.Parse(subset); err != nil {
		t.Fatalf("sfnt.Parse(subset) = %v", err)
	}
	checkSameSegments(t, program, subset, truetypesynth.GIDC)
	checkSameSegments(t, program, subset, truetypesynth.GIDA)
	checkSameSegments(t, program, subset, truetypesynth.GIDB)

	empty, err := Subset(program, nil)
	if err != nil {
		t.Fatalf("Subset(empty) = %v", err)
	}
	checkEmptyGlyph(t, empty, truetypesynth.GIDA)
	checkEmptyGlyph(t, empty, truetypesynth.GIDB)
	checkEmptyGlyph(t, empty, truetypesynth.GIDC)

	if _, err := Subset(program, []sfnt.GlyphIndex{99}); err == nil {
		t.Fatal("Subset accepted an out-of-range glyph")
	}
}

// checkSameSegments proves one glyph loads to the same segments before and
// after the subset.
func checkSameSegments(t *testing.T, program, subset []byte, gid sfnt.GlyphIndex) {
	t.Helper()
	want := segmentsOf(t, program, gid)
	got := segmentsOf(t, subset, gid)
	if len(got) != len(want) {
		t.Fatalf("glyph %d has %d segments, want %d", gid, len(got), len(want))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("glyph %d segment %d = %+v, want %+v", gid, idx, got[idx], want[idx])
		}
	}
}

// checkEmptyGlyph proves a zeroed glyph carries no outline.
func checkEmptyGlyph(t *testing.T, program []byte, gid sfnt.GlyphIndex) {
	t.Helper()
	segments := segmentsOf(t, program, gid)
	if len(segments) != 0 {
		t.Fatalf("glyph %d has %d segments, want none", gid, len(segments))
	}
}

func segmentsOf(t *testing.T, program []byte, gid sfnt.GlyphIndex) sfnt.Segments {
	t.Helper()
	parsed, err := sfnt.Parse(program)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	segments, err := parsed.LoadGlyph(&buf, gid, 64, nil)
	if err != nil {
		return nil
	}
	return segments
}

// TestEmbedSubsetTag proves the tag is six uppercase letters from a digest of
// the subset bytes and follows the bytes, not a date.
func TestEmbedSubsetTag(t *testing.T) {
	t.Parallel()
	program := truetypesynth.Program()
	subset, err := Subset(program, []sfnt.GlyphIndex{truetypesynth.GIDA})
	if err != nil {
		t.Fatal(err)
	}
	tag := SubsetTag(subset)
	if len(tag) != 6 {
		t.Fatalf("tag %q has %d letters, want 6", tag, len(tag))
	}
	for idx := range len(tag) {
		if tag[idx] < 'A' || tag[idx] > 'Z' {
			t.Fatalf("tag %q has %q at %d, want A through Z", tag, tag[idx], idx)
		}
	}
	if got := SubsetTag(subset); got != tag {
		t.Fatalf("tag = %q, want %q", got, tag)
	}
	changed := bytes.Clone(subset)
	changed[len(changed)-1] ^= 0xFF
	if SubsetTag(changed) == tag {
		t.Fatal("tag did not follow the program bytes")
	}
	checkTagDigest(t, subset, tag)
}

// checkTagDigest locks the tag to the SHA-256 prefix and the A through Z
// mapping.
func checkTagDigest(t *testing.T, subset []byte, tag string) {
	t.Helper()
	sum := sha256.Sum256(subset)
	want := make([]byte, 6)
	for idx := range want {
		want[idx] = 'A' + sum[idx]%26
	}
	if tag != string(want) {
		t.Fatalf("tag = %q, want %q", tag, want)
	}
}

// TestEmbedSubsetStable proves two subset runs over one program return equal
// bytes and equal tags, so a rewrite stays byte-stable.
func TestEmbedSubsetStable(t *testing.T) {
	t.Parallel()
	program := truetypesynth.Program()
	before := bytes.Clone(program)
	first, err := Subset(program, []sfnt.GlyphIndex{truetypesynth.GIDC})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Subset(program, []sfnt.GlyphIndex{truetypesynth.GIDC})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two subset runs returned different bytes")
	}
	if !bytes.Equal(program, before) {
		t.Fatal("Subset changed the caller's program")
	}
	if SubsetTag(first) != SubsetTag(second) {
		t.Fatal("two subset runs returned different tags")
	}
}
