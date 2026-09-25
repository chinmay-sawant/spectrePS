package pdf

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// TestEmbedSubsetToUnicode proves the synthesized /ToUnicode CMap for a
// simple font and a Type0 font reads back to the same strings through
// parseToUnicode, and that the map uses both bfchar and bfrange sections.
func TestEmbedSubsetToUnicode(t *testing.T) {
	t.Parallel()
	checkSimpleToUnicode(t)
	checkType0ToUnicode(t)
}

func checkSimpleToUnicode(t *testing.T) {
	t.Helper()
	content := "BT /F1 12 Tf 0 0 Td (ABC) Tj ET"
	file := mustOpen(t, embedPage(t, content,
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FirstChar 65 /Widths [600 400 0] "+
			"/FontDescriptor 6 0 R "+
			"/Encoding << /BaseEncoding /StandardEncoding /Differences [67 /Omega] >> >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
		synthFontStream(t, ""),
	))
	overrides, appended, first := embedParts(t, file)
	stream := toUnicodeOf(t, overrides, 5, first, appended)
	want := map[uint32]string{'A': "A", 'B': "B", 'C': "\u2126"}
	if got := parseToUnicode(stream.Stream); !reflect.DeepEqual(got, want) {
		t.Fatalf("ToUnicode = %v, want %v", got, want)
	}
	if !bytes.Contains(stream.Stream, []byte("beginbfrange")) {
		t.Fatalf("ToUnicode has no bfrange:\n%s", stream.Stream)
	}
	if !bytes.Contains(stream.Stream, []byte("beginbfchar")) {
		t.Fatalf("ToUnicode has no bfchar:\n%s", stream.Stream)
	}
}

func checkType0ToUnicode(t *testing.T) {
	t.Helper()
	file := mustOpen(t, embedPage(t,
		"BT /F1 12 Tf 0 0 Td <00010002> Tj ET",
		type0FontBody(), type0KidBody("/CIDToGIDMap 9 0 R"), type0DescBody(),
		synthFontStream(t, ""), streamBody("", []byte{0, 0, 0, 2, 0, 1}),
	))
	overrides, appended, first := embedParts(t, file)
	stream := toUnicodeOf(t, overrides, 5, first, appended)
	want := map[uint32]string{1: "B", 2: "A"}
	if got := parseToUnicode(stream.Stream); !reflect.DeepEqual(got, want) {
		t.Fatalf("ToUnicode = %v, want %v", got, want)
	}
}

// TestEmbedSubsetWidths proves /Widths, /FirstChar, /LastChar, /W, and /DW
// writing for a font with a source width array, a font without one, and a
// Type0 font trimmed to the used CIDs.
func TestEmbedSubsetWidths(t *testing.T) {
	t.Parallel()
	checkSimpleWidthsSource(t)
	checkSimpleWidthsProgram(t)
	checkCIDWidthTrim(t)
	checkCIDWidthDefault(t)
}

func checkSimpleWidthsSource(t *testing.T) {
	t.Helper()
	file := mustOpen(t, embedPage(t, "BT /F1 12 Tf 0 0 Td (AB) Tj ET",
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FirstChar 65 /Widths [600 400] "+
			"/FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
		synthFontStream(t, ""),
	))
	overrides, _, _ := embedParts(t, file)
	first, last, widths := simpleWidths(t, dictBody(t, overrides[5]))
	if first != 65 || last != 66 {
		t.Fatalf("FirstChar %d LastChar %d, want 65 and 66", first, last)
	}
	want := []Value{IntVal(600), IntVal(400)}
	if !reflect.DeepEqual(widths, want) {
		t.Fatalf("Widths = %v, want %v", widths, want)
	}
}

func checkSimpleWidthsProgram(t *testing.T) {
	t.Helper()
	file := mustOpen(t, embedPage(t, "BT /F1 12 Tf 0 0 Td (AB) Tj ET",
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 7 0 R >>",
		synthFontStream(t, ""),
	))
	overrides, _, _ := embedParts(t, file)
	fnt, err := file.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	first, last, widths := simpleWidths(t, dictBody(t, overrides[5]))
	if first != 65 || last != 66 {
		t.Fatalf("FirstChar %d LastChar %d, want 65 and 66", first, last)
	}
	want := []Value{widthValue(fnt.Width('A')), widthValue(fnt.Width('B'))}
	if !reflect.DeepEqual(widths, want) {
		t.Fatalf("Widths = %v, want the program widths %v", widths, want)
	}
}

func checkCIDWidthTrim(t *testing.T) {
	t.Helper()
	file := mustOpen(t, embedPage(t, "BT /F1 12 Tf 0 0 Td <00010003> Tj ET",
		type0FontBody(), type0KidBody("/W [1 [500 600 700]]"), type0DescBody(),
		synthFontStream(t, ""),
	))
	overrides, _, _ := embedParts(t, file)
	kid := descendantOf(t, dictBody(t, overrides[5]))
	if dw, _ := kid.IntEntry(keyDW); dw != 1000 {
		t.Fatalf("DW = %d, want 1000", dw)
	}
	entries, ok := kid.ArrayEntry(keyW)
	if !ok {
		t.Fatal("/W is missing")
	}
	want := []Value{IntVal(1), ArrayVal([]Value{IntVal(500)}), IntVal(3), ArrayVal([]Value{IntVal(700)})}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("W = %v, want %v", entries, want)
	}
}

func checkCIDWidthDefault(t *testing.T) {
	t.Helper()
	file := mustOpen(t, embedPage(t, "BT /F1 12 Tf 0 0 Td <00010002> Tj ET",
		type0FontBody(), type0KidBody(""), type0DescBody(), synthFontStream(t, ""),
	))
	overrides, _, _ := embedParts(t, file)
	kid := descendantOf(t, dictBody(t, overrides[5]))
	if dw, _ := kid.IntEntry(keyDW); dw != defaultCIDWidth {
		t.Fatalf("DW = %d, want %d", dw, defaultCIDWidth)
	}
	entries, ok := kid.ArrayEntry(keyW)
	if !ok {
		t.Fatal("/W is missing")
	}
	want := []Value{IntVal(1), IntVal(2), IntVal(defaultCIDWidth)}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("W = %v, want %v", entries, want)
	}
}

// TestEmbedSubsetCID proves a Type0 Identity-H subset keeps the CIDs and the
// /CIDToGIDMap, trims /W, and repaints the same glyph outlines after a reader
// round trip.
func TestEmbedSubsetCID(t *testing.T) {
	t.Parallel()
	content := "BT /F1 12 Tf 0 0 Td <00010002> Tj ET"
	file := mustOpen(t, embedPage(t, content,
		type0FontBody(), type0KidBody("/CIDToGIDMap 9 0 R"), type0DescBody(),
		synthFontStream(t, ""), streamBody("", []byte{0, 0, 0, 2, 0, 1}),
	))
	overrides, appended, first := embedParts(t, file)
	dict := dictBody(t, overrides[5])
	kid := descendantOf(t, dict)
	entry, _ := kid.ValueEntry(keyCIDToGIDMap)
	if entry.Kind != KindRef || entry.RefNum != 9 {
		t.Fatalf("CIDToGIDMap = %+v, want object 9", entry)
	}
	checkTaggedName(t, kid, "Synth")
	if _, ok := dict.ValueEntry(keyToUnicode); !ok {
		t.Fatal("the Type0 font has no /ToUnicode")
	}
	rebuilt := rebuildWithBodies(t, file, overrides, first, appended)
	checkSameOutline(t, file, rebuilt, 1)
	checkSameOutline(t, file, rebuilt, 2)
}

// TestEmbedSubsetOpenType proves a /FontFile3 /OpenType program is copied
// whole, its descriptor keeps its reference, and the dictionary still loads
// the program and the widths.
func TestEmbedSubsetOpenType(t *testing.T) {
	t.Parallel()
	file := mustOpen(t, embedPage(t, "BT /F1 12 Tf 0 0 Td (AB) Tj ET",
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synth /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile3 7 0 R >>",
		synthFontStream(t, "/Subtype /OpenType"),
	))
	overrides, appended, first := embedParts(t, file)
	if _, ok := overrides[7]; ok {
		t.Fatal("the /FontFile3 program was replaced")
	}
	dict := dictBody(t, overrides[5])
	ref, _ := dict.ValueEntry(keyFontDescriptor)
	if ref.Kind != KindRef || ref.RefNum != 6 {
		t.Fatalf("FontDescriptor = %+v, want object 6", ref)
	}
	if _, ok := dict.ValueEntry(keyToUnicode); !ok {
		t.Fatal("the OpenType font has no /ToUnicode")
	}
	rebuilt := rebuildWithBodies(t, file, overrides, first, appended)
	if got := programOf(t, rebuilt, 7); !bytes.Equal(got, synthFont()) {
		t.Fatal("the /FontFile3 program changed")
	}
	checkSameOutline(t, file, rebuilt, 'A')
	checkSameOutline(t, file, rebuilt, 'B')
}

// embedParts runs a subsetting pass over one file.
func embedParts(t *testing.T, file *File) (map[int][]byte, [][]byte, int) {
	t.Helper()
	first := file.ObjectCount() + 1
	overrides, appended, err := file.SubsetFontObjects(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	return overrides, appended, first
}

// embedPage builds a one-page PDF whose /F1 is object 5, followed by the
// extra objects numbered from 6.
func embedPage(t *testing.T, content, font string, extra ...string) []byte {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(streamBody("", []byte(content)))
	doc.object(font)
	for _, body := range extra {
		doc.object(body)
	}
	return doc.classic("")
}

// type0FontBody is the Type0 font dictionary at object 5.
func type0FontBody() string {
	return "<< /Type /Font /Subtype /Type0 /BaseFont /Synth /Encoding /Identity-H " +
		"/DescendantFonts [6 0 R] >>"
}

// type0KidBody is the CIDFontType2 descendant at object 6.
func type0KidBody(extra string) string {
	return "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Synth /FontDescriptor 7 0 R " +
		"/DW 1000 " + extra + " >>"
}

// type0DescBody is the descriptor at object 7.
func type0DescBody() string {
	return "<< /Type /FontDescriptor /FontName /Synth /Flags 32 /FontFile2 8 0 R >>"
}

// dictBody parses one bare serialized dictionary or stream body by wrapping
// it in an indirect object header, so ParseIndirect can attach the stream.
func dictBody(t *testing.T, body []byte) Value {
	t.Helper()
	src := make([]byte, 0, len(body)+32)
	src = append(src, "1 0 obj\n"...)
	src = append(src, body...)
	src = append(src, "\nendobj"...)
	num, gen, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("ParseIndirect(%q) = %v", body, err)
	}
	if num != 1 || gen != 0 || next == 0 {
		t.Fatalf("ParseIndirect header = %d %d %d", num, gen, next)
	}
	return val
}

// descendantOf returns the direct descendant font dictionary.
func descendantOf(t *testing.T, dict Value) Value {
	t.Helper()
	kids, ok := dict.ArrayEntry(keyDescendantFonts)
	if !ok || len(kids) != 1 || kids[0].Kind != KindDict {
		t.Fatalf("DescendantFonts = %v", kids)
	}
	return kids[0]
}

// toUnicodeOf resolves the /ToUnicode stream of a font override.
func toUnicodeOf(t *testing.T, overrides map[int][]byte, num, first int, appended [][]byte) Value {
	t.Helper()
	dict := dictBody(t, overrides[num])
	ref, ok := dict.ValueEntry(keyToUnicode)
	if !ok || ref.Kind != KindRef {
		t.Fatalf("font %d has no /ToUnicode", num)
	}
	idx := ref.RefNum - first
	if idx < 0 || idx >= len(appended) {
		t.Fatalf("/ToUnicode reference %d is outside the appended bodies", ref.RefNum)
	}
	stream := dictBody(t, appended[idx])
	if stream.Kind != KindStream {
		t.Fatalf("/ToUnicode body is %v, want a stream", stream.Kind)
	}
	return stream
}

// simpleWidths reads /FirstChar, /LastChar, and /Widths.
func simpleWidths(t *testing.T, dict Value) (int, int, []Value) {
	t.Helper()
	first, ok := dict.IntEntry(keyFirstChar)
	if !ok {
		t.Fatal("/FirstChar is missing")
	}
	last, ok := dict.IntEntry(keyLastChar)
	if !ok {
		t.Fatal("/LastChar is missing")
	}
	widths, ok := dict.ArrayEntry(keyWidths)
	if !ok {
		t.Fatal("/Widths is missing")
	}
	return first, last, widths
}

// checkTaggedName proves a subset /BaseFont carries the six-letter tag.
func checkTaggedName(t *testing.T, dict Value, base string) {
	t.Helper()
	name, ok := dict.NameEntry(keyBaseFont)
	if !ok || len(name) != subsetTagLetters+1+len(base) || !strings.HasSuffix(name, "+"+base) {
		t.Fatalf("BaseFont = %q, want a tagged %s", name, base)
	}
	for idx := range subsetTagLetters {
		letter := name[idx]
		if letter < 'A' || letter > 'Z' {
			t.Fatalf("BaseFont tag %q is not uppercase letters", name)
		}
	}
}

// subsetTagLetters is the six-letter subset tag length.
const subsetTagLetters = 6

// rebuildWithBodies writes the overridden and appended bodies into a fresh
// classic file and opens it, which is the reader round trip without the
// pdfout writer.
func rebuildWithBodies(
	t *testing.T,
	file *File,
	overrides map[int][]byte,
	first int,
	appended [][]byte,
) *File {
	t.Helper()
	doc := newDoc()
	for num := 1; num <= file.ObjectCount(); num++ {
		body, ok := overrides[num]
		if !ok {
			val, found, err := file.ObjectValue(num)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				doc.object("null")
				continue
			}
			body = SerializeValue(val)
		}
		doc.object(string(body))
	}
	for idx, body := range appended {
		if first+idx != len(doc.offsets) {
			t.Fatalf("appended body %d lands at %d, want %d", idx, len(doc.offsets), first+idx)
		}
		doc.object(string(body))
	}
	return mustOpen(t, doc.classic(""))
}

// checkSameOutline proves one code loads the same glyph outline before and
// after the subset round trip.
func checkSameOutline(t *testing.T, before, after *File, code uint32) {
	t.Helper()
	wantFont, err := before.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	gotFont, err := after.PageFont(0, "F1")
	if err != nil {
		t.Fatal(err)
	}
	want, ok := wantFont.outline(code)
	if !ok || len(want) == 0 {
		t.Fatalf("before outline(%d) = %d segments, %v", code, len(want), ok)
	}
	got, ok := gotFont.outline(code)
	if !ok || len(got) != len(want) {
		t.Fatalf("after outline(%d) = %d segments, %v, want %d", code, len(got), ok, len(want))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("outline(%d) segment %d = %+v, want %+v", code, idx, got[idx], want[idx])
		}
	}
	checkSameGlyphIndex(t, wantFont, gotFont, code)
}

// checkSameGlyphIndex proves the code still resolves to the same glyph index.
func checkSameGlyphIndex(t *testing.T, wantFont, gotFont *Font, code uint32) {
	t.Helper()
	gid, ok := gotFont.glyphIndex(code)
	if !ok {
		t.Fatalf("after glyphIndex(%d) is missing", code)
	}
	wantGID, ok := wantFont.glyphIndex(code)
	if !ok {
		t.Fatalf("before glyphIndex(%d) is missing", code)
	}
	if gid != wantGID {
		t.Fatalf("glyphIndex(%d) = %d, want %d", code, gid, wantGID)
	}
}

// programOf decodes one font program object.
func programOf(t *testing.T, file *File, num int) []byte {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok || val.Kind != KindStream {
		t.Fatalf("object %d = %v, %v, %v", num, val.Kind, ok, err)
	}
	body, err := decodeStream(val)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
