package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const notImplemented = "spectreps: not implemented\n"

func callRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func want(t *testing.T, args []string, code int, stdout, stderr string) {
	t.Helper()
	gotCode, gotOut, gotErr := callRun(t, args...)
	if gotCode != code || gotOut != stdout || gotErr != stderr {
		t.Fatalf("run %q: code=%d stdout=%q stderr=%q, want code=%d stdout=%q stderr=%q",
			args, gotCode, gotOut, gotErr, code, stdout, stderr)
	}
}

func wantCode(t *testing.T, args []string, code int) {
	t.Helper()
	got, _, _ := callRun(t, args...)
	if got != code {
		t.Fatalf("run %q: code=%d, want %d", args, got, code)
	}
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func writePair(t *testing.T, a, b []byte) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pa := filepath.Join(dir, "a")
	pb := filepath.Join(dir, "b")
	if err := os.WriteFile(pa, a, 0644); err != nil {
		t.Fatalf("write %s: %v", pa, err)
	}
	if err := os.WriteFile(pb, b, 0644); err != nil {
		t.Fatalf("write %s: %v", pb, err)
	}
	return pa, pb
}

func TestVersion(t *testing.T) {
	want(t, []string{"version"}, 0, "0.0.1\n", "")
	wantCode(t, []string{"version", "extra"}, 2)
}

func TestUsage(t *testing.T) {
	wantCode(t, nil, 2)
	wantCode(t, []string{"no-such-command"}, 2)
	wantCode(t, []string{"run", "-z"}, 2)
	wantCode(t, []string{"run"}, 2)
	wantCode(t, []string{"compare"}, 2)
	wantCode(t, []string{"compare", "bytes", writeTemp(t, "a", []byte("x"))}, 2)
	wantCode(t, []string{"validate", "-o", "out"}, 2)
}

func TestRun(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ps")
	wantCode(t, []string{"run", missing}, 2)
	wantCode(t, []string{"run", t.TempDir()}, 3)

	path := writeTemp(t, "in.ps", []byte("hi"))
	want(t, []string{"run", path}, 1, "", notImplemented)
	want(t, []string{"run", "-w", "200", "-h", "100", "-r", "72", path}, 1, "", notImplemented)
}

func TestRaster(t *testing.T) {
	path := writeTemp(t, "in.ps", []byte("hi"))
	wantCode(t, []string{"raster", path}, 2)
	want(t, []string{"raster", "-o", "out.ppm", path}, 1, "", notImplemented)
}

func TestRewrite(t *testing.T) {
	path := writeTemp(t, "in.pdf", []byte("%PDF"))
	wantCode(t, []string{"rewrite", path}, 2)
	want(t, []string{"rewrite", "-o", "out.pdf", path}, 1, "", notImplemented)
}

func TestValidate(t *testing.T) {
	wantCode(t, []string{"validate", filepath.Join(t.TempDir(), "missing.ps")}, 2)
	path := writeTemp(t, "in.ps", []byte("hi"))
	want(t, []string{"validate", path}, 1, "", notImplemented)
}

func TestCompareBytes(t *testing.T) {
	equalA, equalB := writePair(t, []byte("hello"), []byte("hello"))
	want(t, []string{"compare", "bytes", equalA, equalB}, 0, "", "")

	byteA, byteB := writePair(t, []byte("hello"), []byte("hallo"))
	want(t, []string{"compare", "bytes", byteA, byteB}, 1, "mismatch byte 1\n", "")

	lenA, lenB := writePair(t, []byte("ab"), []byte("abcd"))
	want(t, []string{"compare", "bytes", lenA, lenB}, 1, "mismatch length 2\n", "")

	file, _ := writePair(t, []byte("ab"), []byte("ab"))
	missing := filepath.Join(t.TempDir(), "missing")
	wantCode(t, []string{"compare", "bytes", file, missing}, 2)

	dir := t.TempDir()
	one := filepath.Join(dir, "a")
	if err := os.WriteFile(one, []byte("ab"), 0644); err != nil {
		t.Fatalf("write %s: %v", one, err)
	}
	wantCode(t, []string{"compare", "bytes", one, dir}, 3)
}

func TestCompareRaster(t *testing.T) {
	a, b := writePair(t, []byte("one"), []byte("two"))
	want(t, []string{"compare", "raster", a, b}, 1, "", notImplemented)
	want(t, []string{"compare", "raster", "-r", "72", "-w", "10", "-h", "10", a, b}, 1, "", notImplemented)

	file, _ := writePair(t, []byte("one"), []byte("two"))
	missing := filepath.Join(t.TempDir(), "missing")
	wantCode(t, []string{"compare", "raster", file, missing}, 2)
}
