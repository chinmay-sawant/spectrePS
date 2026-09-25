package pdf

import (
	"bytes"
	"fmt"
	"testing"
)

// TestFlatePredictor applies predictors 2 and 10 through 15 on a Flate
// stream through Decode, on a content stream, an object stream, and an xref
// stream, and refuses a bad tag, a short row, and an unsupported value.
func TestFlatePredictor(t *testing.T) {
	t.Parallel()
	checkPNGPredictors(t)
	checkPNGPerRowTags(t)
	checkTIFFPredictor(t)
	checkTIFFSubByte(t)
	checkPredictorMalformed(t)
	checkPredictorStreamKinds(t)
}

func checkPNGPredictors(t *testing.T) {
	t.Helper()
	raw := []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
	}
	rowBytes := 4
	bpp := 2
	for predictor := predictorPNG; predictor <= predictorPNGHi; predictor++ {
		tag := byte((predictor - predictorPNG) % 5)
		encoded := pngPredictRows(raw, rowBytes, bpp, []byte{tag, tag, tag, tag})
		params := DictVal(map[string]Value{
			"Predictor":        IntVal(int64(predictor)),
			"Colors":           IntVal(2),
			"BitsPerComponent": IntVal(8),
			"Columns":          IntVal(2),
		})
		got, err := Decode(opFlate, params, flateRaw(t, encoded))
		if err != nil {
			t.Fatalf("predictor %d: %v", predictor, err)
		}
		if !bytes.Equal(got, raw) {
			t.Fatalf("predictor %d: got %v", predictor, got)
		}
	}
}

// checkPNGPerRowTags proves the tag byte selects the algorithm for its own
// row, whatever /Predictor says.
func checkPNGPerRowTags(t *testing.T) {
	t.Helper()
	raw := []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120}
	encoded := pngPredictRows(raw, 4, 1, []byte{1, 3, 4})
	for _, predictor := range []int{predictorPNG + 2, predictorPNGHi} {
		params := DictVal(map[string]Value{
			"Predictor": IntVal(int64(predictor)),
			"Columns":   IntVal(4),
		})
		got, err := Decode(opFlate, params, flateRaw(t, encoded))
		if err != nil {
			t.Fatalf("predictor %d: %v", predictor, err)
		}
		if !bytes.Equal(got, raw) {
			t.Fatalf("predictor %d: got %v", predictor, got)
		}
	}
}

func checkTIFFPredictor(t *testing.T) {
	t.Helper()
	raw := []byte{
		10, 20, 30, 40, 50, 60,
		70, 80, 90, 100, 110, 120,
	}
	encoded := tiffPredictRows(raw, 3, 6)
	params := DictVal(map[string]Value{
		"Predictor": IntVal(predictorTIFF),
		"Colors":    IntVal(3),
		"Columns":   IntVal(2),
	})
	got, err := Decode(opFlate, params, flateRaw(t, encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("tiff got %v", got)
	}
}

// checkTIFFSubByte exercises the sample-level arithmetic at 4 bits. Samples
// 1, 2, 4, 8 become differences 1, 1, 2, 4.
func checkTIFFSubByte(t *testing.T) {
	t.Helper()
	encoded := []byte{0x11, 0x24}
	params := DictVal(map[string]Value{
		"Predictor":        IntVal(predictorTIFF),
		"BitsPerComponent": IntVal(4),
		"Columns":          IntVal(4),
	})
	got, err := Decode(opFlate, params, flateRaw(t, encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte{0x12, 0x48}) {
		t.Fatalf("4-bit samples % x", got)
	}
}

func checkPredictorMalformed(t *testing.T) {
	t.Helper()
	cases := []struct {
		name   string
		params map[string]Value
		raw    []byte
	}{
		{name: "bad tag", params: map[string]Value{"Predictor": IntVal(15), "Columns": IntVal(4)},
			raw: []byte{9, 1, 2, 3, 4}},
		{name: "short png row", params: map[string]Value{"Predictor": IntVal(12), "Columns": IntVal(4)},
			raw: []byte{2, 1, 2}},
		{name: "short tiff row", params: map[string]Value{"Predictor": IntVal(2), "Columns": IntVal(4)},
			raw: []byte{1, 2, 3}},
		{name: "bad bits", params: map[string]Value{"Predictor": IntVal(2), "BitsPerComponent": IntVal(3)},
			raw: []byte{1}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(opFlate, DictVal(testCase.params), flateRaw(t, testCase.raw))
			wantJob(t, err, opPredictor, errSyntax)
		})
	}
}

// checkPredictorStreamKinds opens a PDF whose xref stream is PNG Up, whose
// object stream is TIFF Predictor 2, and whose content stream is PNG None.
func checkPredictorStreamKinds(t *testing.T) {
	t.Helper()
	file := mustOpen(t, predictedXRefPDF(t))
	if file.PageCount() != 1 {
		t.Fatalf("pages %d", file.PageCount())
	}
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(lineMarks)) {
		t.Fatalf("content %q", content)
	}
}

// predictedXRefPDF is buildXRefStream with a predictor on each stream kind.
func predictedXRefPDF(t *testing.T) []byte {
	t.Helper()
	plain, first := objectStreamPlain()
	objStm := streamBody(fmt.Sprintf(
		"/Type /ObjStm /N 2 /First %d /Filter /FlateDecode "+
			"/DecodeParms << /Predictor 2 /Columns %d >>", first, len(plain)),
		flateRaw(t, tiffPredictRows(plain, 1, len(plain))))
	page := "<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>"
	content := lineMarks
	contentBody := streamBody(fmt.Sprintf(
		"/Filter /FlateDecode /DecodeParms << /Predictor 10 /Columns %d >>", len(content)),
		flateRaw(t, pngPredictRows([]byte(content), len(content), 1, []byte{0})))

	doc := newDoc()
	doc.put(streamObjs, objStm)
	doc.put(streamPage, page)
	doc.put(streamContent, contentBody)
	xrefAt := doc.buf.Len()
	rows := streamRows(doc.offsets, xrefAt)
	dict := fmt.Sprintf(
		"/Type /XRef /Size %d /Root 1 0 R /W [1 2 2] /Filter /FlateDecode "+
			"/DecodeParms << /Predictor 12 /Columns 5 >>", streamSize)
	predicted := pngPredictRows(rows, 5, 1, bytes.Repeat([]byte{2}, len(rows)/5))
	doc.put(streamXRef, streamBody(dict, flateRaw(t, predicted)))
	if doc.offsets[streamXRef] != xrefAt {
		t.Fatalf("xref offset %d", xrefAt)
	}
	fmt.Fprintf(&doc.buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return doc.buf.Bytes()
}

// TestPredictorImage decodes image XObjects through PNG and TIFF predictors.
func TestPredictorImage(t *testing.T) {
	t.Parallel()
	checkPredictorImagePNG(t)
	checkPredictorImageTIFF(t)
}

func checkPredictorImagePNG(t *testing.T) {
	t.Helper()
	samples := rgbPixels()
	rows := pngPredictRows(samples, 6, 3, []byte{2, 2})
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode " +
		"/DecodeParms << /Predictor 12 /Colors 3 /Columns 2 >>"
	file, num := oneImageDoc(t, dict, flateRaw(t, rows))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, pic, samples)
}

func checkPredictorImageTIFF(t *testing.T) {
	t.Helper()
	samples := rgbPixels()
	encoded := tiffPredictRows(samples, 3, 6)
	dict := "/Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB " +
		"/BitsPerComponent 8 /Filter /FlateDecode " +
		"/DecodeParms << /Predictor 2 /Colors 3 /Columns 2 >>"
	file, num := oneImageDoc(t, dict, flateRaw(t, encoded))
	pic, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	checkRGBPixels(t, pic, samples)
}

// tiffPredictRows differences each sample from the sample colors positions
// to its left, modulo 256, per row. It is the inverse of Predictor 2.
func tiffPredictRows(raw []byte, colors, rowBytes int) []byte {
	out := make([]byte, len(raw))
	for row := 0; row < len(raw); row += rowBytes {
		end := row + rowBytes
		for i := row; i < end; i++ {
			if i-row < colors {
				out[i] = raw[i]
				continue
			}
			out[i] = raw[i] - raw[i-colors]
		}
	}
	return out
}

// pngPredictRows builds PNG-predicted rows with one tag per row. bpp is the
// byte distance to the left sample.
func pngPredictRows(raw []byte, rowBytes, bpp int, tags []byte) []byte {
	if rowBytes < 1 || len(raw)%rowBytes != 0 || len(tags) != len(raw)/rowBytes {
		panic("pdf: pngPredictRows shape")
	}
	out := make([]byte, 0, len(raw)+len(tags))
	prev := make([]byte, rowBytes)
	for row, tag := range tags {
		src := raw[row*rowBytes : (row+1)*rowBytes]
		out = append(out, tag)
		for i := range src {
			out = append(out, pngPredictByte(src, prev, i, bpp, int(tag)))
		}
		prev = src
	}
	return out
}

// pngPredictByte predicts one byte for the given PNG algorithm tag.
func pngPredictByte(src, prev []byte, index, bpp, tag int) byte {
	left := pngRawLeft(src, index, bpp)
	above := prev[index]
	aboveLeft := pngRawLeft(prev, index, bpp)
	switch tag {
	case 0:
		return src[index]
	case 1:
		return src[index] - left
	case 2:
		return src[index] - above
	case 3:
		return src[index] - byte((int(left)+int(above))/2)
	case 4:
		return src[index] - paeth(left, above, aboveLeft)
	default:
		panic("pdf: pngPredictRows tag")
	}
}

func pngRawLeft(row []byte, index, bpp int) byte {
	if index < bpp {
		return 0
	}
	return row[index-bpp]
}
