package pdf

import "bytes"

// binaryHighByte is the first byte above the ASCII range. A PDF 2.0 header
// carries one in its binary marker comment.
const binaryHighByte = 127

// Header returns the source header block: the "%PDF-x.y" line and, when the
// next line is a binary marker comment, that line too. A file without a header
// returns nil. The copy writer keeps a tagged source's header version, so a
// PDF 2.0 file with tags does not leave as a 1.4 shell.
func (file *File) Header() []byte {
	if file == nil {
		return nil
	}
	start := bytes.Index(file.src, []byte(pdfHeader))
	if start < 0 {
		return nil
	}
	end := lineEnd(file.src, start)
	if next := end; next < len(file.src) && file.src[next] == '%' {
		marker := lineEnd(file.src, next)
		if binaryMarker(file.src[next:marker]) {
			end = marker
		}
	}
	return cloneBytes(file.src[start:end])
}

// lineEnd returns the index just past the line that starts at start.
func lineEnd(src []byte, start int) int {
	found := bytes.IndexByte(src[start:], '\n')
	if found < 0 {
		return len(src)
	}
	return start + found + 1
}

// binaryMarker reports whether one comment line carries a byte above 127, the
// PDF 2.0 binary marker rule.
func binaryMarker(line []byte) bool {
	for _, cur := range line {
		if cur > binaryHighByte {
			return true
		}
	}
	return false
}
