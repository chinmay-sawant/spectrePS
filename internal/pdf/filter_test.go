package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"testing"
)

func TestDecodeFlate(t *testing.T) {
	t.Parallel()
	plain := []byte("q\n10 20 m\n30 40 l\nS\n")
	got, err := Decode(opFlate, NullVal(), zlibOf(t, plain))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q", got)
	}
	params := DictVal(map[string]Value{"Predictor": IntVal(1)})
	got, err = Decode(opFlate, params, zlibOf(t, plain))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("predictor 1 got %q", got)
	}
}

func TestDecodeEmpty(t *testing.T) {
	t.Parallel()
	raw := []byte("abc")
	params := DictVal(map[string]Value{"Predictor": IntVal(12)})
	got, err := Decode("", params, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(raw) || &got[0] != &raw[0] {
		t.Fatal("empty filter changed the buffer")
	}
}

func TestDecodeUnknownFilter(t *testing.T) {
	t.Parallel()
	_, err := Decode("BogusDecode", NullVal(), nil)
	wantErr(t, err, "BogusDecode", errUndefined)
}

// TestDecodeUnsupportedPredictor keeps the old refusal for a predictor the
// decoder does not implement. Predictors 2 and 10 through 15 now apply.
func TestDecodeUnsupportedPredictor(t *testing.T) {
	t.Parallel()
	params := DictVal(map[string]Value{"Predictor": IntVal(3)})
	_, err := Decode(opFlate, params, []byte{1, 2, 3})
	wantErr(t, err, opPredictor, errUndefined)
	bad := DictVal(map[string]Value{"Predictor": NameVal("yes")})
	_, err = Decode(opFlate, bad, []byte{1})
	wantErr(t, err, opPredictor, errSyntax)
}

func wantErr(t *testing.T, err error, opName, errName string) {
	t.Helper()
	var got *Error
	if !errors.As(err, &got) {
		t.Fatalf("err=%v want %s %s", err, opName, errName)
	}
	if got.Op != opName || got.Name != errName {
		t.Fatalf("op=%s name=%s", got.Op, got.Name)
	}
}

func zlibOf(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	if _, err := writer.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
