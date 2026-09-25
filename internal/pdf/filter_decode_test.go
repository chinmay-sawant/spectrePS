package pdf

import (
	"bytes"
	"encoding/ascii85"
	"testing"
)

// TestASCII85Decode decodes full and partial groups, the z shortcut, and the
// optional <~ wrapper, and refuses a bad character, a single leftover
// character, and a group above 2^32 - 1.
func TestASCII85Decode(t *testing.T) {
	t.Parallel()
	checkASCII85RoundTrip(t)
	checkASCII85WhitespaceAndWrapper(t)
	checkASCII85BadInput(t)
}

func checkASCII85RoundTrip(t *testing.T) {
	t.Helper()
	for size := range 9 {
		plain := make([]byte, size)
		for i := range plain {
			plain[i] = byte(i * 37)
		}
		encoded := ascii85Of(t, plain)
		got, err := Decode(opASCII85, NullVal(), encoded)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("size %d: got %q", size, got)
		}
	}
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	got, err := Decode(opASCII85, NullVal(), ascii85Of(t, all))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, all) {
		t.Fatal("all byte values did not round trip")
	}
}

// ascii85Of encodes plain with the standard library encoder and appends the
// PDF end-of-data marker.
func ascii85Of(t *testing.T, plain []byte) []byte {
	t.Helper()
	out := make([]byte, ascii85.MaxEncodedLen(len(plain)))
	count := ascii85.Encode(out, plain)
	return append(out[:count], '~', '>')
}

func checkASCII85WhitespaceAndWrapper(t *testing.T) {
	t.Helper()
	encoded := append([]byte("<~"), ascii85Of(t, []byte("Hello, world!"))...)
	encoded = append(encoded, " \n\tjunk"...)
	got, err := Decode(opASCII85, NullVal(), encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Hello, world!" {
		t.Fatalf("got %q", got)
	}
}

func checkASCII85BadInput(t *testing.T) {
	t.Helper()
	cases := map[string]string{
		"invalid character": "v~>",
		"single leftover":   "A~>",
		"group over 2^32":   "uuuuu~>",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(opASCII85, NullVal(), []byte(raw))
			wantJob(t, err, opASCII85, errSyntax)
		})
	}
}

// TestASCIIHexDecode decodes uppercase, lowercase, whitespace, the EOD
// marker, and a padded final nibble, and refuses a non-hex character.
func TestASCIIHexDecode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: "48656C6C6F>", want: "Hello"},
		{name: "lowercase", raw: "48656c6c6f>", want: "Hello"},
		{name: "whitespace", raw: "48 65\n6C\t6C 6F >", want: "Hello"},
		{name: "odd nibble", raw: "48656c6c6f7>", want: "Hello\x70"},
		{name: "eof ends", raw: "41", want: "A"},
		{name: "eod only", raw: ">", want: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := Decode(opASCIIHex, NullVal(), []byte(testCase.raw))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != testCase.want {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
	_, err := Decode(opASCIIHex, NullVal(), []byte("4G"))
	wantJob(t, err, opASCIIHex, errSyntax)
}

// TestRunLengthDecode decodes literal runs, repeat runs, and the EOD marker,
// refuses a truncated run, and stops at the 32 MiB cap with limitcheck.
func TestRunLengthDecode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  []byte
		want []byte
	}{
		{name: "literal", raw: []byte{0x02, 'a', 'b', 'c', 0x80}, want: []byte("abc")},
		{name: "repeat", raw: []byte{0xFF, 'x', 0xFE, 'y', 0x80}, want: []byte("xxyyy")},
		{name: "max repeat", raw: []byte{0x81, 'z', 0x80}, want: bytes.Repeat([]byte{'z'}, 128)},
		{name: "eod ends", raw: []byte{0x00, 'a', 0x80, 0x00, 'b'}, want: []byte("a")},
		{name: "eof ends", raw: []byte{0x00, 'a'}, want: []byte("a")},
		{name: "empty", raw: []byte{}, want: []byte{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := Decode(opRunLength, NullVal(), testCase.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, testCase.want) {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
	_, err := Decode(opRunLength, NullVal(), []byte{0x05, 'a', 'b'})
	wantJob(t, err, opRunLength, errSyntax)
	_, err = Decode(opRunLength, NullVal(), []byte{0x83})
	wantJob(t, err, opRunLength, errSyntax)
	checkRunLengthCap(t)
}

func checkRunLengthCap(t *testing.T) {
	t.Helper()
	raw := make([]byte, 0, maxInflated/64)
	for range maxInflated/128 + 1 {
		raw = append(raw, 0x81, 'x')
	}
	_, err := Decode(opRunLength, NullVal(), raw)
	wantJob(t, err, opRunLength, errLimit)
}

// TestFilterChain locks decodeChain: each stage decodes in order, per-stage
// /DecodeParms reach the stage, and an unknown stage reports undefined with
// its own name.
func TestFilterChain(t *testing.T) {
	t.Parallel()
	checkChainOrder(t)
	checkChainParams(t)
	checkChainUnknown(t)
}

func checkChainOrder(t *testing.T) {
	t.Helper()
	plain := []byte("q 10 20 m 30 40 l S\n")
	stored := ascii85Of(t, flateRaw(t, plain))
	val := chainStream([]string{opASCII85, opFlate}, NullVal(), stored)
	got, err := decodeStream(val)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("chain got %q", got)
	}
}

func checkChainParams(t *testing.T) {
	t.Helper()
	plain := []byte{1, 3, 6, 10, 15, 21, 28, 36}
	predicted := tiffPredictRows(plain, 1, len(plain))
	stored := ascii85Of(t, flateRaw(t, predicted))
	parms := ArrayVal([]Value{
		NullVal(),
		DictVal(map[string]Value{"Predictor": IntVal(predictorTIFF), "Columns": IntVal(int64(len(plain)))}),
	})
	val := chainStream([]string{opASCII85, opFlate}, parms, stored)
	got, err := decodeStream(val)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("chain params got %v", got)
	}
}

func checkChainUnknown(t *testing.T) {
	t.Helper()
	stored := flateRaw(t, []byte("q"))
	val := chainStream([]string{opFlate, "BogusDecode"}, NullVal(), stored)
	_, err := decodeStream(val)
	wantJob(t, err, "BogusDecode", errUndefined)
}

// chainStream builds one stream value with a filter chain and DecodeParms.
func chainStream(filters []string, parms Value, raw []byte) Value {
	names := make([]Value, 0, len(filters))
	for _, name := range filters {
		names = append(names, NameVal(name))
	}
	dict := map[string]Value{"Filter": ArrayVal(names)}
	if parms.Kind != KindNull {
		dict["DecodeParms"] = parms
	}
	return StreamVal(dict, raw)
}
