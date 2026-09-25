package spectreps_test

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestCompareRaster(t *testing.T) {
	checkEqualRaster(t)
	checkWidthHeight(t)
	checkPixelOffset(t)
	checkStridePadding(t)
}

func checkEqualRaster(t *testing.T) {
	t.Helper()
	left := imageWith(2, 2, 2*3)
	right := imageWith(2, 2, 2*3)
	eq := spectreps.CompareRaster(left, right)
	if !eq.Equal || eq.Offset != -1 || eq.Reason != "" {
		t.Fatalf("equal %+v", eq)
	}
}

func checkWidthHeight(t *testing.T) {
	t.Helper()
	left := imageWith(2, 2, 2*3)
	wide := imageWith(3, 2, 3*3)
	if got := spectreps.CompareRaster(left, wide); got.Reason != "width" || got.Offset != -1 || got.Equal {
		t.Fatalf("width %+v", got)
	}
	tall := imageWith(2, 3, 2*3)
	if got := spectreps.CompareRaster(left, tall); got.Reason != "height" || got.Offset != -1 || got.Equal {
		t.Fatalf("height %+v", got)
	}
}

func checkPixelOffset(t *testing.T) {
	t.Helper()
	left := imageWith(2, 2, 2*3)
	changed := imageWith(2, 2, 2*3)
	changed.Pixels[1] = 9
	if got := spectreps.CompareRaster(left, changed); got.Reason != "pixel" || got.Offset != 1 || got.Equal {
		t.Fatalf("pixel %+v", got)
	}
}

func checkStridePadding(t *testing.T) {
	t.Helper()
	padA := imageWith(2, 1, 8)
	padB := imageWith(2, 1, 8)
	padB.Pixels[6] = 7
	padB.Pixels[7] = 9
	if got := spectreps.CompareRaster(padA, padB); !got.Equal {
		t.Fatalf("padding %+v", got)
	}
	padB.Pixels[4] = 1
	if got := spectreps.CompareRaster(padA, padB); got.Reason != "pixel" || got.Offset != 4 {
		t.Fatalf("packed offset %+v", got)
	}
}

// TestValidationCompareEdges locks the nil-against-empty file compare and the
// width-before-height precedence.
func TestValidationCompareEdges(t *testing.T) {
	t.Run("nil against empty", func(t *testing.T) {
		got := spectreps.CompareFiles(nil, []byte{})
		want := spectreps.CompareResult{Equal: true, Offset: -1, Reason: ""}
		if got != want {
			t.Fatalf("CompareFiles(nil, empty) = %+v, want %+v", got, want)
		}
		empty := spectreps.CompareFiles([]byte{}, nil)
		if empty != want {
			t.Fatalf("CompareFiles(empty, nil) = %+v, want %+v", empty, want)
		}
	})
	t.Run("length at zero", func(t *testing.T) {
		got := spectreps.CompareFiles([]byte("a"), nil)
		want := spectreps.CompareResult{Equal: false, Offset: 0, Reason: "length"}
		if got != want {
			t.Fatalf("CompareFiles(a, nil) = %+v, want %+v", got, want)
		}
	})
	t.Run("width before height", func(t *testing.T) {
		left := imageWith(2, 2, 6)
		both := imageWith(3, 4, 9)
		got := spectreps.CompareRaster(left, both)
		if got.Reason != "width" || got.Offset != -1 || got.Equal {
			t.Fatalf("CompareRaster(width and height differ) = %+v, want width", got)
		}
		tall := imageWith(2, 4, 6)
		got = spectreps.CompareRaster(left, tall)
		if got.Reason != "height" || got.Offset != -1 || got.Equal {
			t.Fatalf("CompareRaster(height differs) = %+v, want height", got)
		}
	})
}

func imageWith(width, height, stride int) spectreps.PageImage {
	return spectreps.PageImage{Width: width, Height: height, Stride: stride, Pixels: make([]byte, height*stride)}
}
