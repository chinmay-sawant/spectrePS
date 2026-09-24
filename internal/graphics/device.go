package graphics

// Marker receives device-space marks.
// The pixmap and the PDF rewrite device both implement it.
// documentation/devices.md names this seam.
type Marker interface {
	// Stroke receives one stroke. width is already in device pixels.
	Stroke(pts []Point, width, red, green, blue float64)
	// Fill receives one interior. evenOdd selects the even-odd rule.
	Fill(pts []Point, red, green, blue float64, evenOdd bool)
}
