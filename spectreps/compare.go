package spectreps

import (
	"bytes"

	"github.com/chinmay-sawant/spectrePS/internal/engine"
)

const rgbBytes = 3

// CompareFiles compares file bytes.
func CompareFiles(a, b []byte) CompareResult {
	equal, offset, reason := engine.CompareBytes(a, b)
	return CompareResult{Equal: equal, Offset: offset, Reason: reason}
}

// CompareRaster compares RGB pixels. Stride padding is ignored.
// Width is checked before height. The first differing pixel byte sets Offset.
func CompareRaster(a, b PageImage) CompareResult {
	if a.Width != b.Width {
		return CompareResult{Equal: false, Offset: -1, Reason: "width"}
	}
	if a.Height != b.Height {
		return CompareResult{Equal: false, Offset: -1, Reason: "height"}
	}
	rowBytes := a.Width * rgbBytes
	// Two tight strides are one packed buffer per image, so bytes.Equal skips
	// the per-byte loop. A padded or mismatched stride keeps the loop.
	if a.Stride == rowBytes && b.Stride == rowBytes &&
		bytes.Equal(a.Pixels[:a.Height*rowBytes], b.Pixels[:b.Height*rowBytes]) {
		return CompareResult{Equal: true, Offset: -1, Reason: ""}
	}
	var offset int64
	for row := range a.Height {
		aBase := row * a.Stride
		bBase := row * b.Stride
		for col := range rowBytes {
			if a.Pixels[aBase+col] != b.Pixels[bBase+col] {
				return CompareResult{Equal: false, Offset: offset, Reason: "pixel"}
			}
			offset++
		}
	}
	return CompareResult{Equal: true, Offset: -1, Reason: ""}
}
