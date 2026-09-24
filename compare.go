package spectreps

// CompareResult is one byte or pixel comparison.
// Equal slices use Offset -1 and an empty Reason.
type CompareResult struct {
	Equal  bool
	Offset int64
	Reason string
}

// CompareFiles compares file bytes.
// Equal slices, including two empty slices, set Equal true, Offset -1, and Reason empty.
// The first differing index sets Equal false, Offset to that index, and Reason "byte".
// If one slice is a prefix of the other, Offset is the shorter length and Reason is "length".
// A difference before the end of the shorter slice is Reason "byte", not "length".
func CompareFiles(a, b []byte) CompareResult {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return CompareResult{Equal: false, Offset: int64(i), Reason: "byte"}
		}
	}
	if len(a) != len(b) {
		return CompareResult{Equal: false, Offset: int64(n), Reason: "length"}
	}
	return CompareResult{Equal: true, Offset: -1, Reason: ""}
}

// CompareRaster compares RGB pixels. The real compare arrives in a later tag.
// Phase 02 panics with ErrNotImplemented. The signature returns CompareResult and cannot return an error.
func CompareRaster(a, b PageImage) CompareResult {
	panic(ErrNotImplemented)
}
