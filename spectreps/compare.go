package spectreps

import "github.com/chinmay-sawant/spectrePS/internal/engine"

// CompareFiles compares file bytes.
func CompareFiles(a, b []byte) CompareResult {
	equal, offset, reason := engine.CompareBytes(a, b)
	return CompareResult{Equal: equal, Offset: offset, Reason: reason}
}

// CompareRaster compares RGB pixels. The real compare arrives in a later tag.
// This tag panics with ErrNotImplemented.
func CompareRaster(a, b PageImage) CompareResult {
	_ = a
	_ = b
	engine.CompareRaster()

	return CompareResult{Equal: false, Offset: -1, Reason: ""}
}
