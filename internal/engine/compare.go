package engine

// CompareBytes compares file bytes.
// Equal slices, including two empty slices, report offset -1 and an empty reason.
// The first differing index reports that index and reason "byte".
// A proper prefix reports the shorter length and reason "length".
func CompareBytes(a, b []byte) (bool, int64, string) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := range n {
		if a[i] != b[i] {
			return false, int64(i), "byte"
		}
	}
	if len(a) != len(b) {
		return false, int64(n), "length"
	}
	return true, -1, ""
}
