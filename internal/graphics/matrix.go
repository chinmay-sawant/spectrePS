package graphics

import "math"

const halfTurnDegrees = 180

// Matrix maps user space to the space above it.
// x' = A*x + C*y + E. y' = B*x + D*y + F.
type Matrix struct {
	A float64
	B float64
	C float64
	D float64
	E float64
	F float64
}

// Identity is one user unit to one unit, origin unchanged.
func Identity() Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}

// Apply transforms one point.
func (m Matrix) Apply(userX, userY float64) (float64, float64) {
	return m.A*userX + m.C*userY + m.E, m.B*userX + m.D*userY + m.F
}

// Concat returns left applied to the user point, then right.
func Concat(left, right Matrix) Matrix {
	return Matrix{
		A: left.A*right.A + left.B*right.C,
		B: left.A*right.B + left.B*right.D,
		C: left.C*right.A + left.D*right.C,
		D: left.C*right.B + left.D*right.D,
		E: left.E*right.A + left.F*right.C + right.E,
		F: left.E*right.B + left.F*right.D + right.F,
	}
}

// Translate returns m with a translation applied in user space first.
func (m Matrix) Translate(tx, ty float64) Matrix {
	return Concat(Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty}, m)
}

// Scale returns m with a scale applied in user space first.
func (m Matrix) Scale(sx, sy float64) Matrix {
	return Concat(Matrix{A: sx, B: 0, C: 0, D: sy, E: 0, F: 0}, m)
}

// Rotate returns m with a rotation of degrees applied in user space first.
func (m Matrix) Rotate(degrees float64) Matrix {
	rad := degrees * math.Pi / halfTurnDegrees
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	return Concat(Matrix{A: cos, B: sin, C: -sin, D: cos, E: 0, F: 0}, m)
}

// Invert returns the inverse. The second result is false when the matrix is singular.
func (m Matrix) Invert() (Matrix, bool) {
	det := m.A*m.D - m.B*m.C
	if det == 0 {
		return Matrix{A: 0, B: 0, C: 0, D: 0, E: 0, F: 0}, false
	}
	return Matrix{
		A: m.D / det,
		B: -m.B / det,
		C: -m.C / det,
		D: m.A / det,
		E: (m.C*m.F - m.D*m.E) / det,
		F: (m.B*m.E - m.A*m.F) / det,
	}, true
}
