package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestRasterOwnImagePDF runs the CLI round trip: raster the PostScript
// source, wrap it with pdfimage, raster the PDF, and compare the PPM bodies.
// The gray conversion is the identity on the black and white source.
func TestRasterOwnImagePDF(t *testing.T) {
	cases := []struct {
		name       string
		colorspace []string
	}{
		{name: "rgb"},
		{name: "gray", colorspace: []string{"-colorspace", "gray"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			program := filepath.Join(dir, "in.ps")
			if err := os.WriteFile(program, []byte("0 0 moveto 10 0 lineto stroke"), 0o600); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(dir, "source.ppm")
			want(t, rasterArgs(source, program), 0, "", "")
			imagePDF := filepath.Join(dir, "image.pdf")
			pdfArgs := []string{"pdfimage", "-o", imagePDF}
			pdfArgs = append(pdfArgs, tt.colorspace...)
			pdfArgs = append(pdfArgs, "-w", "20", "-h", "20", "-r", "72", program)
			want(t, pdfArgs, 0, "", "")
			roundTrip := filepath.Join(dir, "round.ppm")
			want(t, rasterArgs(roundTrip, imagePDF), 0, "", "")
			got := readPayload(t, roundTrip)
			expected := readPayload(t, source)
			if !bytes.Equal(got, expected) {
				t.Fatal("round trip PPM differs from the source raster")
			}
		})
	}
}

func rasterArgs(out, in string) []string {
	return []string{"raster", "-o", out, "-w", "20", "-h", "20", "-r", "72", in}
}
