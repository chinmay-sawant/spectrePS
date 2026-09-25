package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/tiff"
)

func TestRasterTIFF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "solid.ps")
	prog := []byte("0 0.5 0 setrgbcolor 0 0 moveto 20 0 lineto 20 20 lineto 0 20 lineto closepath fill")
	if err := os.WriteFile(src, prog, 0o600); err != nil {
		t.Fatal(err)
	}
	ppm := filepath.Join(dir, "out.ppm")
	want(t, []string{"raster", "-o", ppm, "-w", "20", "-h", "20", "-r", "72", src}, 0, "", "")
	body := ppmBody(t, ppm)

	output := func(name string, flag []string) []byte {
		path := filepath.Join(dir, name)
		args := []string{"raster", "-o", path, "-w", "20", "-h", "20", "-r", "72"}
		args = append(args, flag...)
		args = append(args, src)
		want(t, args, 0, "", "")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	def := output("default.tiff", nil)
	deflate := output("deflate.tif", []string{"-tiffcompress", "deflate"})
	none := output("none.tif", []string{"-tiffcompress", "none"})

	if !bytes.Equal(def, deflate) {
		t.Fatal("default TIFF bytes differ from -tiffcompress deflate")
	}
	if bytes.Equal(none, deflate) {
		t.Fatal("none and deflate TIFF bytes match")
	}
	for _, tt := range []struct {
		name string
		raw  []byte
	}{
		{"default", def},
		{"deflate", deflate},
		{"none", none},
	} {
		checkTIFF(t, tt.name, tt.raw, body)
	}

	bad := filepath.Join(dir, "bad.tif")
	wantCode(t, []string{"raster", "-tiffcompress", "lzw", "-o", bad, "-w", "20", "-h", "20", "-r", "72", src}, 2)
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatalf("bad flag wrote %s", bad)
	}
	wantCode(t, []string{"run", "-tiffcompress", "none", src}, 2)
}

// checkTIFF decodes a raster output and compares its pixels with the PPM
// body of the same program. TIFF bytes are not an equality oracle.
func checkTIFF(t *testing.T, name string, raw, body []byte) {
	t.Helper()
	pic, err := tiff.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s decode: %v", name, err)
	}
	if pic.Bounds().Dx() != 20 || pic.Bounds().Dy() != 20 {
		t.Fatalf("%s bounds %v, want 20x20", name, pic.Bounds())
	}
	for y := range 20 {
		for x := range 20 {
			red, green, blue, _ := pic.At(x, y).RGBA()
			i := (y*20 + x) * 3
			if byte(red>>8) != body[i] || byte(green>>8) != body[i+1] || byte(blue>>8) != body[i+2] {
				t.Fatalf("%s pixel %d,%d", name, x, y)
			}
		}
	}
}
