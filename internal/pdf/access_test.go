package pdf

import (
	"bytes"
	"testing"
)

const missingNum = 999

func TestObjectAccess(t *testing.T) {
	checkClassicAccess(t)
	checkPackedAccess(t)
}

func checkClassicAccess(t *testing.T) {
	t.Helper()
	file := mustOpen(t, classicLine(t, lineMarks))
	if file.ObjectCount() != idContent {
		t.Fatalf("ObjectCount = %d, want %d", file.ObjectCount(), idContent)
	}
	if file.RootNum() != idCatalog {
		t.Fatalf("RootNum = %d, want %d", file.RootNum(), idCatalog)
	}
	wantRawObject(t, file, idCatalog, "<< /Type /Catalog /Pages 2 0 R >>")
	wantRawObject(t, file, idPages, "<< /Type /Pages /Kids [3 0 R] /Count 2 >>")
	body, ok := file.RawObject(idContent)
	if !ok || !bytes.HasPrefix(body, []byte("<< /Filter /FlateDecode /Length ")) {
		t.Fatalf("RawObject(%d) = %q ok %v", idContent, body, ok)
	}
	checkCatalogValue(t, file, idCatalog)
	checkContentValue(t, file)
	checkMissingAccess(t, file)
}

func checkPackedAccess(t *testing.T) {
	t.Helper()
	file := mustOpen(t, buildXRefStream(t))
	if file.ObjectCount() != streamXRef {
		t.Fatalf("ObjectCount = %d, want %d", file.ObjectCount(), streamXRef)
	}
	if file.RootNum() != idCatalog {
		t.Fatalf("RootNum = %d, want %d", file.RootNum(), idCatalog)
	}
	if _, ok := file.RawObject(idCatalog); ok {
		t.Fatal("compressed object returned raw bytes")
	}
	checkCatalogValue(t, file, idCatalog)
	wantRawObject(t, file, streamPage, "<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>")
	checkMissingAccess(t, file)
}

func wantRawObject(t *testing.T, file *File, num int, want string) {
	t.Helper()
	body, ok := file.RawObject(num)
	if !ok || string(body) != want {
		t.Fatalf("RawObject(%d) = %q ok %v, want %q", num, body, ok, want)
	}
}

func checkCatalogValue(t *testing.T, file *File, num int) {
	t.Helper()
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok || val.Kind != KindDict {
		t.Fatalf("ObjectValue(%d) kind %d ok %v err %v", num, val.Kind, ok, err)
	}
	typ, found := val.NameEntry(keyType)
	if !found || typ != "Catalog" {
		t.Fatalf("Type = %q found %v", typ, found)
	}
}

func checkContentValue(t *testing.T, file *File) {
	t.Helper()
	val, ok, err := file.ObjectValue(idContent)
	if err != nil || !ok || val.Kind != KindStream {
		t.Fatalf("ObjectValue(%d) kind %d ok %v err %v", idContent, val.Kind, ok, err)
	}
	decoded, err := decodeStream(val)
	if err != nil || !bytes.Equal(decoded, []byte(lineMarks)) {
		t.Fatalf("decoded %q err %v", decoded, err)
	}
}

func checkMissingAccess(t *testing.T, file *File) {
	t.Helper()
	if _, ok := file.RawObject(missingNum); ok {
		t.Fatalf("RawObject(%d) reported bytes", missingNum)
	}
	if _, ok, err := file.ObjectValue(missingNum); ok || err != nil {
		t.Fatalf("ObjectValue(%d) ok %v err %v", missingNum, ok, err)
	}
}

func TestSerializeValue(t *testing.T) {
	checkScalarSyntax(t)
	checkContainerSyntax(t)
	checkStreamSyntax(t)
}

func checkScalarSyntax(t *testing.T) {
	t.Helper()
	wantInt(t, string(SerializeValue(IntVal(7))), 7)
	wantReal(t, string(SerializeValue(RealVal(2.5))), 2.5)
	wantReal(t, string(SerializeValue(RealVal(1))), 1)
	wantBool(t, string(SerializeValue(BoolVal(true))), true)
	wantBool(t, string(SerializeValue(BoolVal(false))), false)
	wantNull(t, string(SerializeValue(NullVal())))
	wantName(t, string(SerializeValue(NameVal("A B"))), "A B")
	wantName(t, string(SerializeValue(NameVal("A#B"))), "A#B")
	wantName(t, string(SerializeValue(NameVal("caf\xc3\xa9"))), "caf\xc3\xa9")
	wantString(t, string(SerializeValue(StringVal("a(b)\\c"))), "a(b)\\c")
	wantString(t, string(SerializeValue(StringVal("line\ncr\rtab\t"))), "line\ncr\rtab\t")
	wantString(t, string(SerializeValue(StringVal("\x00\xff"))), "\x00\xff")
}

func checkContainerSyntax(t *testing.T) {
	t.Helper()
	dict := DictVal(map[string]Value{
		"Type":  NameVal("Catalog"),
		"Pages": RefVal(2, 0),
	})
	if got := string(SerializeValue(dict)); got != "<< /Pages 2 0 R /Type /Catalog >>" {
		t.Fatalf("dict = %q", got)
	}
	if got := string(SerializeValue(ArrayVal(nil))); got != "[]" {
		t.Fatalf("empty array = %q", got)
	}
	if got := string(SerializeValue(DictVal(nil))); got != "<<>>" {
		t.Fatalf("empty dict = %q", got)
	}
	items := []Value{
		IntVal(1), RealVal(2), NameVal("N"), StringVal("x"),
		BoolVal(false), NullVal(), RefVal(3, 0),
	}
	want := "[1 2.0 /N (x) false null 3 0 R]"
	if got := string(SerializeValue(ArrayVal(items))); got != want {
		t.Fatalf("array = %q, want %q", got, want)
	}
	parsed, next := mustValue(t, want)
	if parsed.Kind != KindArray || next != len(want) || len(parsed.Array) != len(items) {
		t.Fatalf("parsed kind %d len %d next %d", parsed.Kind, len(parsed.Array), next)
	}
}

func checkStreamSyntax(t *testing.T) {
	t.Helper()
	val := StreamVal(map[string]Value{
		"Length": IntVal(99),
		"Filter": NameVal("FlateDecode"),
	}, []byte("abc"))
	body := string(SerializeValue(val))
	want := "<< /Filter /FlateDecode /Length 3 >>\nstream\nabc\nendstream"
	if body != want {
		t.Fatalf("stream = %q, want %q", body, want)
	}
	src := []byte("1 0 obj\n" + body + "\nendobj")
	num, gen, parsed, _, err := ParseIndirect(src, 0)
	if err != nil || num != 1 || gen != 0 || parsed.Kind != KindStream {
		t.Fatalf("ParseIndirect num %d gen %d kind %d err %v", num, gen, parsed.Kind, err)
	}
	if !bytes.Equal(parsed.Stream, []byte("abc")) {
		t.Fatalf("stream bytes %q", parsed.Stream)
	}
	if length, ok := parsed.IntEntry(wordLength); !ok || length != 3 {
		t.Fatalf("Length %d ok %v", length, ok)
	}
}
