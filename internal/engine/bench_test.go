package engine

import "testing"

// The package had no benchmark before this file. CompareBytes is already
// measured indirectly by BenchmarkCompareFiles in spectreps, so this is the
// direct number and the mismatch path beside it.

// benchCompareBytes builds two buffers that differ only in the last byte when
// differ is true.
func benchCompareBytes(differ bool) ([]byte, []byte) {
	left := make([]byte, 1<<20)
	for i := range left {
		left[i] = byte(i)
	}
	right := make([]byte, len(left))
	copy(right, left)
	if differ {
		right[len(right)-1]++
	}
	return left, right
}

// BenchmarkCompareBytes compares equal buffers and buffers whose only
// difference is the final byte, so the full scan and the early return are
// both timed.
func BenchmarkCompareBytes(b *testing.B) {
	for _, testCase := range []struct {
		name   string
		differ bool
	}{
		{name: "equal", differ: false},
		{name: "last-byte", differ: true},
	} {
		b.Run(testCase.name, func(b *testing.B) {
			left, right := benchCompareBytes(testCase.differ)
			b.SetBytes(int64(len(left)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				equal, _, _ := CompareBytes(left, right)
				if equal == testCase.differ {
					b.Fatalf("equal = %v, want %v", equal, !testCase.differ)
				}
			}
		})
	}
}
