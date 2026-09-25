package pdf

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestCMYKPaint proves K and k reach the RGB pixmap through the frozen
// cmykPreview rule r = (1-C)(1-K).
func TestCMYKPaint(t *testing.T) {
	t.Run("half black", func(t *testing.T) {
		img := paintImage(t, "0 0 0 0.5 k 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 128, 128, 128)
	})
	t.Run("magenta", func(t *testing.T) {
		img := paintImage(t, "0 1 0 0 k 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 255, 0, 255)
	})
	t.Run("stroke operator shares the color", func(t *testing.T) {
		img := paintImage(t, "4 w 1 0 0 0 K 0 2 m 20 2 l S")
		wantPixel(t, img, 10, 18, 0, 255, 255)
	})
	t.Run("component survives q Q", func(t *testing.T) {
		img := paintImage(t, "1 0 0 rg q 0 0 0 1 k Q 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
}

// TestGenericColorOps proves CS, cs, SC, sc, SCN, and scn select a space and
// components, resolve a /ColorSpace resource, and restore on Q.
func TestGenericColorOps(t *testing.T) {
	t.Run("device space and sc", func(t *testing.T) {
		img := paintImage(t, "/DeviceCMYK cs 0 1 0 0 sc 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 255, 0, 255)
	})
	t.Run("upper operators", func(t *testing.T) {
		img := paintImage(t, "/DeviceRGB CS 1 0 0 SC 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("scn", func(t *testing.T) {
		img := paintImage(t, "/DeviceGray cs 0.5 scn 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 128, 128, 128)
	})
	t.Run("resource space", func(t *testing.T) {
		file := separationPage(t, "/CS0 cs 0.5 sc 0 0 20 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 128, 128, 128)
	})
	t.Run("space restores", func(t *testing.T) {
		img := paintImage(t, "1 0 0 rg q /DeviceCMYK cs 0 0 0 1 sc Q 0 0 20 20 re f")
		wantPixel(t, img, 10, 10, 255, 0, 0)
	})
	t.Run("pattern name refuses", func(t *testing.T) {
		err := Paint(t.Context(), []byte("/DeviceCMYK cs /P1 scn"),
			graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "scn", nameUndefined)
	})
	t.Run("unknown space refuses", func(t *testing.T) {
		err := Paint(t.Context(), []byte("/Nope cs"), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "cs", nameUndefined)
	})
	t.Run("components typecheck", func(t *testing.T) {
		err := Paint(t.Context(), []byte("/DeviceCMYK cs (x) sc"),
			graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, "sc", nameType)
	})
}

// TestSeparationRaster proves a page whose /ColorSpace resource is a
// Separation or DeviceN space rasterizes through the preview conversion.
func TestSeparationRaster(t *testing.T) {
	t.Run("separation", func(t *testing.T) {
		file := separationPage(t, "/CS0 cs 0.5 sc 0 0 20 20 re f")
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 128, 128, 128)
	})
	t.Run("devicen", func(t *testing.T) {
		resources := "<< /ColorSpace << /CS0 5 0 R >> >>"
		bodies := []string{
			"[/DeviceN [/Cyan /Magenta /Yellow /Black] /DeviceCMYK 6 0 R]",
			streamBody("/FunctionType 4 /Domain [0 1 0 1 0 1 0 1] "+
				"/Range [0 1 0 1 0 1 0 1]", nil),
		}
		file := doPage(t, "/CS0 cs 0 0 0 1 sc 0 0 20 20 re f", resources, bodies...)
		img := paintPage(t, file)
		wantPixel(t, img, 10, 10, 0, 0, 0)
	})
}

// separationPage builds one 20 by 20 page whose /CS0 is a Separation over
// DeviceCMYK with a type 2 tint transform.
func separationPage(t *testing.T, content string) *File {
	t.Helper()
	resources := "<< /ColorSpace << /CS0 5 0 R >> >>"
	bodies := []string{
		"[/Separation /Spot /DeviceCMYK 6 0 R]",
		streamBody("/FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [1 1 1 0] /N 1", nil),
	}
	return doPage(t, content, resources, bodies...)
}
