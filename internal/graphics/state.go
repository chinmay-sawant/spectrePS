package graphics

const (
	defaultLineWidth = 1
	maxGSaveDepth    = 32
)

// State is the graphics state the path operators share.
// The default matrix is the identity. The default line width is 1. The default color is gray 0.
type State struct {
	CTM   Matrix
	Width float64
	Red   float64
	Green float64
	Blue  float64
	saves []snap
}

type snap struct {
	ctm   Matrix
	width float64
	red   float64
	green float64
	blue  float64
}

// NewState returns the defaults from the language file.
func NewState() *State {
	return &State{CTM: Identity(), Width: defaultLineWidth, Red: 0, Green: 0, Blue: 0, saves: nil}
}

// SetGray sets a gray level and the matching RGB color.
func (s *State) SetGray(gray float64) {
	s.Red = gray
	s.Green = gray
	s.Blue = gray
}

// SetRGB sets an RGB color.
func (s *State) SetRGB(red, green, blue float64) {
	s.Red = red
	s.Green = green
	s.Blue = blue
}

// SetLineWidth sets the stroke width in user space.
func (s *State) SetLineWidth(width float64) {
	s.Width = width
}

// GSave copies the matrix, width, and color. Depth 32 is the last legal save.
func (s *State) GSave() error {
	if len(s.saves) >= maxGSaveDepth {
		return errLimit
	}
	s.saves = append(s.saves, snap{
		ctm:   s.CTM,
		width: s.Width,
		red:   s.Red,
		green: s.Green,
		blue:  s.Blue,
	})
	return nil
}

// GRestore restores the last gsave. An empty save stack is limitcheck.
func (s *State) GRestore() error {
	if len(s.saves) == 0 {
		return errLimit
	}
	last := len(s.saves) - 1
	saved := s.saves[last]
	s.saves = s.saves[:last]
	s.CTM = saved.ctm
	s.Width = saved.width
	s.Red = saved.red
	s.Green = saved.green
	s.Blue = saved.blue
	return nil
}
