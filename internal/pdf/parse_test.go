package pdf

import (
	"bytes"
	"strconv"
	"testing"
)

func TestParse(t *testing.T) {
	parseDict(t)
	parseArray(t)
	parseRef(t)
	parseIndirectInt(t)
	parseDictObject(t)
	parseHelloStream(t)
	parseEmbeddedEndstream(t)
	parseValueStops(t)
	parseTruncated(t)
	parseIndirectLength(t)
}

func parseDict(t *testing.T) {
	t.Helper()
	src := []byte("<< /Type /Page /Count 2 >>")
	val, next, err := ParseValue(src, 0)
	if err != nil {
		t.Fatalf("ParseValue: %v", err)
	}
	if val.Kind != KindDict || next != len(src) {
		t.Fatalf("dict kind %d next %d", val.Kind, next)
	}
	typ, ok := val.NameEntry("Type")
	if !ok || typ != "Page" {
		t.Fatalf("Type = %q ok %v", typ, ok)
	}
	count, ok := val.IntEntry("Count")
	if !ok || count != 2 {
		t.Fatalf("Count = %d ok %v", count, ok)
	}
}

func parseArray(t *testing.T) {
	t.Helper()
	src := []byte("[1 (hi) /N]")
	val, next, err := ParseValue(src, 0)
	if err != nil {
		t.Fatalf("ParseValue: %v", err)
	}
	if val.Kind != KindArray || next != len(src) || len(val.Array) != 3 {
		t.Fatalf("array kind %d len %d next %d", val.Kind, len(val.Array), next)
	}
	wantArrayItem(t, val.Array[0], KindInt, "", 1)
	wantArrayItem(t, val.Array[1], KindString, "hi", 0)
	wantArrayItem(t, val.Array[2], KindName, "N", 0)
}

func wantArrayItem(t *testing.T, item Value, kind Kind, text string, number int64) {
	t.Helper()
	if item.Kind != kind {
		t.Fatalf("kind %d want %d", item.Kind, kind)
	}
	if kind == KindInt && item.Int != number {
		t.Fatalf("int %d", item.Int)
	}
	if kind == KindString && item.String != text {
		t.Fatalf("string %q", item.String)
	}
	if kind == KindName && item.Name != text {
		t.Fatalf("name %q", item.Name)
	}
}

func parseRef(t *testing.T) {
	t.Helper()
	src := []byte("12 0 R tail")
	val, next, err := ParseValue(src, 0)
	if err != nil {
		t.Fatalf("ParseValue: %v", err)
	}
	if val.Kind != KindRef || val.RefNum != 12 || val.RefGen != 0 || next != len("12 0 R") {
		t.Fatalf("ref num %d gen %d kind %d next %d", val.RefNum, val.RefGen, val.Kind, next)
	}
	lone := []byte("12 0")
	val, next, err = ParseValue(lone, 0)
	if err != nil {
		t.Fatalf("ParseValue lone: %v", err)
	}
	if val.Kind != KindInt || val.Int != 12 || next != len("12") {
		t.Fatalf("lone kind %d int %d next %d", val.Kind, val.Int, next)
	}
}

func parseIndirectInt(t *testing.T) {
	t.Helper()
	const prefix = "%PDF-1.4\n"
	const body = "3 0 obj\n42\nendobj"
	src := []byte(prefix + body)
	num, gen, val, next, err := ParseIndirect(src, len(prefix))
	if err != nil {
		t.Fatalf("ParseIndirect: %v", err)
	}
	if num != 3 || gen != 0 || next != len(src) {
		t.Fatalf("num %d gen %d next %d", num, gen, next)
	}
	if val.Kind != KindInt || val.Int != 42 {
		t.Fatalf("value kind %d int %d", val.Kind, val.Int)
	}
}

func parseDictObject(t *testing.T) {
	t.Helper()
	src := []byte("1 0 obj\n<< /Type /X >>\nendobj")
	num, gen, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("ParseIndirect: %v", err)
	}
	if num != 1 || gen != 0 || next != len(src) || val.Kind != KindDict {
		t.Fatalf("dict object num %d gen %d kind %d next %d", num, gen, val.Kind, next)
	}
	typ, ok := val.NameEntry("Type")
	if !ok || typ != "X" {
		t.Fatalf("Type = %q ok %v", typ, ok)
	}
}

func parseHelloStream(t *testing.T) {
	t.Helper()
	seps := []string{"\n", "\r", "\r\n"}
	for _, sep := range seps {
		parseOneHello(t, sep)
	}
}

func parseOneHello(t *testing.T, sep string) {
	t.Helper()
	raw := "hello"
	obj := "7 0 obj" + sep + "<< /Length 5 >>" + sep + "stream" + sep + raw + sep + "endstream" + sep + "endobj"
	tail := sep + "trailer"
	src := []byte(obj + tail)
	num, gen, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("sep %q: %v", sep, err)
	}
	if num != 7 || gen != 0 || next != len(obj) {
		t.Fatalf("sep %q num %d gen %d next %d want %d", sep, num, gen, next, len(obj))
	}
	if string(src[next:]) != tail {
		t.Fatalf("sep %q tail %q", sep, src[next:])
	}
	if val.Kind != KindStream || !bytes.Equal(val.Stream, []byte(raw)) {
		t.Fatalf("sep %q stream %q kind %d", sep, val.Stream, val.Kind)
	}
	length, ok := val.IntEntry(wordLength)
	if !ok || length != len(raw) {
		t.Fatalf("sep %q Length %d ok %v", sep, length, ok)
	}
}

func parseEmbeddedEndstream(t *testing.T) {
	t.Helper()
	raw := "helloendstream"
	src := indirectStream(8, raw)
	_, _, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("ParseIndirect: %v", err)
	}
	if next != len(src) || val.Kind != KindStream || !bytes.Equal(val.Stream, []byte(raw)) {
		t.Fatalf("embedded stream %q next %d kind %d", val.Stream, next, val.Kind)
	}
}

func parseValueStops(t *testing.T) {
	t.Helper()
	src := []byte("<< /Length 5 >>\nstream\nhello")
	val, next, err := ParseValue(src, 0)
	if err != nil {
		t.Fatalf("ParseValue: %v", err)
	}
	stop := bytes.IndexByte(src, '\n')
	if val.Kind != KindDict || next != stop {
		t.Fatalf("kind %d next %d want %d", val.Kind, next, stop)
	}
	if _, ok := val.IntEntry(wordLength); !ok {
		t.Fatal("missing Length")
	}
}

func parseTruncated(t *testing.T) {
	t.Helper()
	job := mustFailIndirect(t, []byte("1 0 obj"))
	if job.Name != nameSyntax {
		t.Fatal(job)
	}
	short := []byte("1 0 obj\n<< /Length 5 >>\nstream\nhel")
	job = mustFailIndirect(t, short)
	if job.Name != nameSyntax {
		t.Fatal(job)
	}
	cut := []byte("1 0 obj\n<< /Length 5 >>\nstream\nhello\nendobj")
	job = mustFailIndirect(t, cut)
	if job.Op != wordEndStream {
		t.Fatalf("Op = %q, want %s", job.Op, wordEndStream)
	}
}

func mustFailIndirect(t *testing.T, src []byte) *Error {
	t.Helper()
	objNum, _, _, end, err := ParseIndirect(src, 0)
	if err == nil && objNum == 0 && end == 0 {
		t.Fatal("expected error")
	}
	return wantSyntax(t, err)
}

// parseIndirectLength locks the fallback for an indirect /Length that has no
// resolver: the reader scans for endstream instead of refusing the stream.
func parseIndirectLength(t *testing.T) {
	t.Helper()
	src := []byte("1 0 obj\n<< /Length 2 0 R >>\nstream\nhello\nendstream\nendobj")
	_, _, val, next, err := ParseIndirect(src, 0)
	if err != nil {
		t.Fatalf("ParseIndirect: %v", err)
	}
	if next != len(src) || val.Kind != KindStream || !bytes.Equal(val.Stream, []byte("hello")) {
		t.Fatalf("stream %q next %d kind %d", val.Stream, next, val.Kind)
	}
}

func indirectStream(num int, raw string) []byte {
	objNum := strconv.Itoa(num)
	length := strconv.Itoa(len(raw))
	body := objNum + " 0 obj\n<< /Length " + length + " >>\nstream\n" + raw + "\nendstream\nendobj"
	return []byte(body)
}
