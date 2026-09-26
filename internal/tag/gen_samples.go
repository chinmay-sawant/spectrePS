//go:build ignore

// Command gen_samples writes the generated PDF/UA-2 samples under
// sampledata/pdfua2. Run it from the module root:
//
//	go run internal/tag/gen_samples.go
//
// The files are deterministic: no dates are written, and a second run of the
// generator returns equal bytes. generated/report.pdf is a Spectre tagged
// write that passes pdfa.PreflightUA2 and veraPDF UA-2; negative/no-title.pdf
// is the refused claim case, which keeps the tree and writes no claim.
// veraPDF is never a build or runtime dependency.
package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// sampleDir is the repo-relative output directory. The refused claim case is
// a deliberate negative fixture and goes under negative/, which the make
// check excludes from the veraPDF run.
const sampleDir = "sampledata/pdfua2"

// reportTitle is the dc:title of the positive sample.
const reportTitle = "Annual report"

func main() {
	root := rootDir()
	src := sourcePDF()
	opt := spectreps.RewriteOptions{Tag: true, Claim: true, Title: reportTitle, Lang: "en-US"}
	pos, err := tagWrite(src, opt)
	if err != nil {
		fail("generated report: %v", err)
	}
	again, err := tagWrite(src, opt)
	if err != nil {
		fail("generated report, second run: %v", err)
	}
	if !bytes.Equal(pos, again) {
		fail("generated report is not stable")
	}
	refused, err := tagWrite(src, spectreps.RewriteOptions{Tag: true, Claim: true, Lang: "en-US"})
	if err == nil {
		fail("no-title write did not refuse the claim")
	}
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != "ua2-title" {
		fail("no-title refusal = %v, want ua2-title", err)
	}
	if refused == nil {
		fail("no-title refusal dropped the tree")
	}
	writeSample(filepath.Join(root, sampleDir, "generated", "report.pdf"), pos)
	writeSample(filepath.Join(root, sampleDir, "negative", "no-title.pdf"), refused)
	report("generated/report.pdf", pos)
	report("negative/no-title.pdf", refused)
}

// tagWrite runs the public tagged write over one source.
func tagWrite(src []byte, opt spectreps.RewriteOptions) ([]byte, error) {
	ctx := context.Background()
	in, err := spectreps.New()
	if err != nil {
		return nil, err
	}
	defer in.Close()
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return nil, err
	}
	return in.RewritePDF(ctx, doc, opt)
}

// rootDir is the module root, from this file's path.
func rootDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fail("cannot locate the generator source")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// report opens one sample and prints its UA-2 preflight outcome.
func report(name string, src []byte) {
	file, err := pdf.Open(context.Background(), src)
	if err != nil {
		fail("open %s: %v", name, err)
	}
	if err := pdfa.PreflightUA2(context.Background(), file); err != nil {
		fmt.Printf("%s: %v\n", name, err)
		return
	}
	fmt.Printf("%s: passes PreflightUA2\n", name)
}

// sourcePDF is the untagged input: a heading, two paragraphs, a two-item
// list, a two-by-two table, a rule, and an image with /Alt. The font is one
// synthetic embedded TrueType program whose glyphs cover printable ASCII.
func sourcePDF() []byte {
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 260] /Contents 4 0 R " +
			"/Resources << /Font << /F1 5 0 R >> /XObject << /Im0 6 0 R >> >> >>"),
		streamBody(reportContent()),
		fontDict(),
		imageBody(),
		descriptorDict(),
		fontProgramBody(),
		streamBody(toUnicodeCMap()),
	}
	return classicPDF(objects)
}

// reportContent is the sample page content stream.
func reportContent() string {
	return "BT /F1 18 Tf 20 230 Td (Annual report) Tj ET\n" +
		"BT /F1 12 Tf 20 200 Td (First paragraph line one) Tj ET\n" +
		"BT /F1 12 Tf 20 188 Td (First paragraph line two) Tj ET\n" +
		"BT /F1 12 Tf 20 155 Td (Second paragraph.) Tj ET\n" +
		"BT /F1 12 Tf 20 120 Td (-) Tj ET\n" +
		"BT /F1 12 Tf 32 120 Td (First item) Tj ET\n" +
		"BT /F1 12 Tf 20 108 Td (-) Tj ET\n" +
		"BT /F1 12 Tf 32 108 Td (Second item) Tj ET\n" +
		"BT /F1 12 Tf 20 90 Td (Name) Tj ET\n" +
		"BT /F1 12 Tf 130 90 Td (Value) Tj ET\n" +
		"BT /F1 12 Tf 20 78 Td (Alpha) Tj ET\n" +
		"BT /F1 12 Tf 130 78 Td (One) Tj ET\n" +
		"20 60 m 280 60 l S\n" +
		"q 40 0 0 40 20 10 cm /Im0 Do Q\n"
}

// fontDict is the embedded TrueType font dictionary. The /Widths array, the
// font program, and the /ToUnicode map agree on one advance and one box
// glyph for every printable ASCII code, so the PDF/UA-2 glyph and width
// rules pass.
func fontDict() []byte {
	widths := make([]string, 0, asciiLast-asciiFirst+1)
	for code := asciiFirst; code <= asciiLast; code++ {
		widths = append(widths, fmt.Sprint(boxAdvance))
	}
	body := fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /SynthBox "+
		"/FirstChar %d /LastChar %d /Widths [%s] /Encoding /WinAnsiEncoding "+
		"/ToUnicode 9 0 R /FontDescriptor 7 0 R >>",
		asciiFirst, asciiLast, join(widths))
	return []byte(body)
}

// descriptorDict names the embedded font program at object 8.
func descriptorDict() []byte {
	body := fmt.Sprintf("<< /Type /FontDescriptor /FontName /SynthBox /Flags 32 "+
		"/FontBBox [%d 0 %d %d] /ItalicAngle 0 /Ascent 800 /Descent -200 "+
		"/CapHeight %d /StemV 80 /FontFile2 8 0 R >>",
		boxMinX, boxMaxX, boxMaxY, boxMaxY)
	return []byte(body)
}

// fontProgramBody wraps the synthetic TrueType program in a Flate stream.
func fontProgramBody() []byte {
	program := boxFont()
	compressed := flateBytes(program)
	head := fmt.Sprintf("<< /Length %d /Filter /FlateDecode /Length1 %d >>\nstream\n",
		len(compressed), len(program))
	out := append([]byte(head), compressed...)
	return append(out, []byte("\nendstream")...)
}

// toUnicodeCMap maps the printable ASCII codes to the same Unicode code
// points, so the text carries a Unicode mapping.
func toUnicodeCMap() string {
	return "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\n" +
		"begincmap\n" +
		"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n" +
		"/CMapName /Adobe-Identity-UCS def\n" +
		"/CMapType 2 def\n" +
		"1 begincodespacerange\n" +
		"<20> <7E>\n" +
		"endcodespacerange\n" +
		"1 beginbfrange\n" +
		"<20> <7E> <0020>\n" +
		"endbfrange\n" +
		"endcmap\n" +
		"CMapName currentdict /CMap defineresource pop\n" +
		"end\n" +
		"end"
}

// imageBody is a four by four RGB Flate image with an /Alt entry, which the
// tag builder reads before it decodes.
func imageBody() []byte {
	pic := image.NewRGBA(image.Rect(0, 0, 4, 4))
	raw := make([]byte, 0, 4*4*3)
	for y := range 4 {
		for x := range 4 {
			at := pic.PixOffset(x, y)
			pic.Pix[at] = byte(x * 60)
			pic.Pix[at+1] = byte(y * 60)
			pic.Pix[at+2] = byte((x + y) * 30)
			pic.Pix[at+3] = 255
			raw = append(raw, pic.Pix[at], pic.Pix[at+1], pic.Pix[at+2])
		}
	}
	compressed := flateBytes(raw)
	head := fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 4 /Height 4 "+
		"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode "+
		"/Alt (A four by four chart) /Length %d >>\nstream\n", len(compressed))
	out := append([]byte(head), compressed...)
	return append(out, []byte("\nendstream")...)
}

// The synthetic TrueType metrics. One box glyph per printable ASCII code,
// and one advance width, keep the font program glyphs present and the widths
// consistent with /Widths.
const (
	asciiFirst     = 0x20
	asciiLast      = 0x7E
	boxAdvance     = 500
	boxUpem        = 1000
	boxGlyphLength = 34
	boxGlyphCount  = asciiLast - asciiFirst + 1
	u16Mask        = 0xFFFF
	u32Mask        = 0xFFFFFFFF
)

// boxFont returns one minimal TrueType program: an empty .notdef and one box
// glyph per printable ASCII code. x/image/font/sfnt reads it, so Spectre
// rasterizes the sample.
func boxFont() []byte {
	builder := &sfntBuilder{}
	builder.add("cmap", boxCmap())
	builder.add("glyf", boxGlyf())
	builder.add("head", boxHead())
	builder.add("hhea", boxHhea())
	builder.add("hmtx", boxHmtx())
	builder.add("loca", boxLoca())
	builder.add("maxp", boxMaxp())
	builder.add("post", boxPost())
	return builder.bytes()
}

// sfntBuilder collects the tables of one font.
type sfntBuilder struct {
	tables []sfntTable
}

// sfntTable is one font table.
type sfntTable struct {
	tag  string
	data []byte
}

// add appends one table.
func (builder *sfntBuilder) add(tag string, data []byte) {
	builder.tables = append(builder.tables, sfntTable{tag: tag, data: data})
}

// bytes writes the table directory, the tables, and the head checksum
// adjustment.
func (builder *sfntBuilder) bytes() []byte {
	count := len(builder.tables)
	searchRange := 16
	entrySelector := 0
	for searchRange*2 <= 16*count {
		searchRange *= 2
		entrySelector++
	}
	rangeShift := 16*count - searchRange
	total := 12 + 16*count
	offsets := make([]int, count)
	for idx, table := range builder.tables {
		offsets[idx] = total
		total += padded4(len(table.data))
	}
	out := make([]byte, total)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, count)
	putU16(out, 6, searchRange)
	putU16(out, 8, entrySelector)
	putU16(out, 10, rangeShift)
	for idx, table := range builder.tables {
		entry := out[12+16*idx:]
		copy(entry[0:4], table.tag)
		putU32(entry, 4, tableChecksum(table.data))
		putOffset(entry, 8, offsets[idx])
		putOffset(entry, 12, len(table.data))
		copy(out[offsets[idx]:], table.data)
	}
	adjust := uint32(0xB1B0AFBA) - tableChecksum(out)
	for idx, table := range builder.tables {
		if table.tag == "head" {
			putU32(out, offsets[idx]+8, adjust)
		}
	}
	return out
}

// boxHead is the font header with a short loca format.
func boxHead() []byte {
	out := make([]byte, 54)
	putU32(out, 0, 0x00010000)
	putU32(out, 4, 0x00010000)
	putU32(out, 12, 0x5F0F3CF5)
	putU16(out, 16, 0)
	putU16(out, 18, boxUpem)
	putI16(out, 36, boxMinX)
	putI16(out, 38, 0)
	putI16(out, 40, boxMaxX)
	putI16(out, 42, boxMaxY)
	putU16(out, 44, 0)
	putU16(out, 46, 8)
	putI16(out, 48, 2)
	putI16(out, 50, 0)
	putI16(out, 52, 0)
	return out
}

// boxHhea is the horizontal header with one metric per glyph.
func boxHhea() []byte {
	out := make([]byte, 36)
	putU32(out, 0, 0x00010000)
	putI16(out, 4, 800)
	putI16(out, 6, -200)
	putI16(out, 8, 0)
	putU16(out, 10, boxAdvance)
	putI16(out, 12, 0)
	putI16(out, 14, 0)
	putI16(out, 16, boxMaxX)
	putI16(out, 18, 1)
	putI16(out, 20, 0)
	putI16(out, 22, 0)
	putI16(out, 32, 0)
	putU16(out, 34, boxGlyphCount+1)
	return out
}

// boxMaxp is the glyph count.
func boxMaxp() []byte {
	out := make([]byte, 32)
	putU32(out, 0, 0x00010000)
	putU16(out, 4, boxGlyphCount+1)
	putU16(out, 6, 4)
	putU16(out, 8, 1)
	putU16(out, 10, 0)
	putU16(out, 12, 0)
	putU16(out, 14, 2)
	return out
}

// boxHmtx gives every glyph the same advance, which the /Widths array
// matches.
func boxHmtx() []byte {
	out := make([]byte, 4*(boxGlyphCount+1))
	for index := 0; index <= boxGlyphCount; index++ {
		putU16(out, index*4, boxAdvance)
		putI16(out, index*4+2, 0)
	}
	return out
}

// boxLoca gives the short offsets: an empty .notdef, then one equal glyph
// per code.
func boxLoca() []byte {
	out := make([]byte, 2*(boxGlyphCount+2))
	putU16(out, 0, 0)
	putU16(out, 2, 0)
	for index := 1; index <= boxGlyphCount; index++ {
		putU16(out, 2*(index+1), index*boxGlyphLength/2)
	}
	return out
}

// boxGlyf is one contour of four on-curve points per glyph.
func boxGlyf() []byte {
	out := make([]byte, 0, boxGlyphCount*boxGlyphLength)
	for range boxGlyphCount {
		out = append(out, boxGlyph()...)
	}
	return out
}

// boxGlyph is one box contour.
func boxGlyph() []byte {
	out := make([]byte, boxGlyphLength)
	putI16(out, 0, 1)
	putI16(out, 2, boxMinX)
	putI16(out, 4, 0)
	putI16(out, 6, boxMaxX)
	putI16(out, 8, boxMaxY)
	putU16(out, 10, 3)
	putU16(out, 12, 0)
	out[14], out[15], out[16], out[17] = 0x01, 0x01, 0x01, 0x01
	putI16(out, 18, boxMinX)
	putI16(out, 20, boxMaxX-boxMinX)
	putI16(out, 22, 0)
	putI16(out, 24, boxMinX-boxMaxX)
	putI16(out, 26, 0)
	putI16(out, 28, 0)
	putI16(out, 30, boxMaxY)
	putI16(out, 32, 0)
	return out
}

// The box glyph box.
const (
	boxMinX = 50
	boxMaxX = 450
	boxMaxY = 700
)

// boxCmap wraps the format 4 subtable in the cmap table header: version 0,
// one subtable, platform 3 encoding 1.
func boxCmap() []byte {
	sub := boxCmapFormat4()
	out := make([]byte, 12+len(sub))
	putU16(out, 0, 0)
	putU16(out, 2, 1)
	putU16(out, 4, 3)
	putU16(out, 6, 1)
	putU32(out, 8, 12)
	copy(out[12:], sub)
	return out
}

// boxCmapFormat4 maps every printable ASCII code to glyph 1 and 0xFFFF to
// .notdef.
func boxCmapFormat4() []byte {
	type segment struct {
		start int
		end   int
		delta int
	}
	segments := []segment{
		{start: asciiFirst, end: asciiLast, delta: 1 - asciiFirst},
		{start: 0xFFFF, end: 0xFFFF, delta: 1},
	}
	count := len(segments)
	out := make([]byte, 16+8*count)
	putU16(out, 0, 4)
	putU16(out, 2, len(out))
	putU16(out, 4, 0)
	putU16(out, 6, count*2)
	putU16(out, 8, 4)
	putU16(out, 10, 1)
	putU16(out, 12, 2)
	offset := 14
	for _, seg := range segments {
		putU16(out, offset, seg.end)
		offset += 2
	}
	putU16(out, offset, 0)
	offset += 2
	for _, seg := range segments {
		putU16(out, offset, seg.start)
		offset += 2
	}
	for _, seg := range segments {
		putU16(out, offset, seg.delta)
		offset += 2
	}
	return out
}

// boxPost is post version 3.0, which carries no glyph names.
func boxPost() []byte {
	out := make([]byte, 32)
	putU32(out, 0, 0x00030000)
	return out
}

// streamBody wraps one uncompressed stream.
func streamBody(content string) []byte {
	return []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
}

// classicPDF writes a classic-xref file with a 1.4 header.
func classicPDF(objects [][]byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 1, len(objects)+1)
	for index, body := range objects {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n", index+1)
		buf.Write(body)
		buf.WriteString("\nendobj\n")
	}
	xrefAt := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets), xrefAt)
	return buf.Bytes()
}

// join joins the width strings with a space.
func join(items []string) string {
	var buf bytes.Buffer
	for index, item := range items {
		if index > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(item)
	}
	return buf.String()
}

// flateBytes compresses one plain byte slice with zlib.
func flateBytes(plain []byte) []byte {
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	if _, err := writer.Write(plain); err != nil {
		fail("flate: %v", err)
	}
	if err := writer.Close(); err != nil {
		fail("flate: %v", err)
	}
	return out.Bytes()
}

// padded4 rounds one size up to a four-byte boundary.
func padded4(size int) int {
	return (size + 3) &^ 3
}

// tableChecksum is the sum of one table's big-endian words.
func tableChecksum(data []byte) uint32 {
	var sum uint32
	for idx := 0; idx < len(data); idx += 4 {
		var word [4]byte
		copy(word[:], data[idx:min(idx+4, len(data))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}

// putU16 writes one 16-bit value.
func putU16(data []byte, offset, value int) {
	binary.BigEndian.PutUint16(data[offset:], uint16(value&u16Mask)) //nolint:gosec // the mask bounds the value
}

// putI16 writes one signed 16-bit value.
func putI16(data []byte, offset int, value int) {
	binary.BigEndian.PutUint16(data[offset:], uint16(value)) //nolint:gosec // int16 and uint16 are both 16 bits
}

// putU32 writes one 32-bit value.
func putU32(data []byte, offset int, value uint32) {
	binary.BigEndian.PutUint32(data[offset:], value)
}

// putOffset writes one file offset or length.
func putOffset(data []byte, offset, value int) {
	binary.BigEndian.PutUint32(data[offset:], uint32(value&u32Mask)) //nolint:gosec // the mask bounds the value
}

// writeSample writes one sample, creating the directory.
func writeSample(path string, src []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		fail("write %s: %v", path, err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen_samples: "+format+"\n", args...)
	os.Exit(1)
}
