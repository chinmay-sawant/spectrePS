package spectreps

import "github.com/chinmay-sawant/spectrePS/internal/engine"

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
	var offset int64
	rowBytes := a.Width * rgbBytes
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
