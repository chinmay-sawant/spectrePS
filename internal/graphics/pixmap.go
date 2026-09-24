package graphics

import "math"

const (
	bytesPerPixel = 3
	whiteByte     = 255
	colorScale    = 255
	pixelCenter   = 0.5
	halfWidth     = 2
	minFillPoints = 2
)

// Point is one device-space point. Y grows up. Move starts a subpath.
type Point struct {
	X    float64
	Y    float64
	Move bool
}

// Image is one page. Row 0 is the top. Stride is Width * 3.
type Image struct {
	Width  int
	Height int
	Stride int
	Pixels []byte
}

// Pixmap is the RGB device. It flips Y when it stores a pixel.
type Pixmap struct {
	w     int
	h     int
	pix   []byte
	pages []Image
}

// NewPixmap allocates a white page. The caller has already applied the pixel caps.
func NewPixmap(width, height int) *Pixmap {
	stride := width * bytesPerPixel
	pix := make([]byte, height*stride)
	for i := range pix {
		pix[i] = whiteByte
	}
	return &Pixmap{w: width, h: height, pix: pix, pages: nil}
}

// Stroke paints a centered stroke. width is already in device pixels.
func (p *Pixmap) Stroke(pts []Point, width, red, green, blue float64) {
	if width < 0 {
		width = -width
	}
	half := width / halfWidth
	redByte, greenByte, blueByte := colorByte(red), colorByte(green), colorByte(blue)
	for _, seg := range segments(pts) {
		p.strokeSegment(seg[0], seg[1], half, redByte, greenByte, blueByte)
	}
}

// Fill paints the interior. evenOdd selects the even-odd rule.
func (p *Pixmap) Fill(pts []Point, red, green, blue float64, evenOdd bool) {
	subs := subpaths(pts)
	if len(subs) == 0 {
		return
	}
	redByte, greenByte, blueByte := colorByte(red), colorByte(green), colorByte(blue)
	for row := range p.h {
		centerY := float64(p.h-1-row) + pixelCenter
		for col := range p.w {
			centerX := float64(col) + pixelCenter
			if inside(subs, centerX, centerY, evenOdd) {
				p.set(col, row, redByte, greenByte, blueByte)
			}
		}
	}
}

// ShowPage keeps the current pixels as a page and clears the buffer to white.
func (p *Pixmap) ShowPage() {
	copied := make([]byte, len(p.pix))
	copy(copied, p.pix)
	p.pages = append(p.pages, Image{
		Width:  p.w,
		Height: p.h,
		Stride: p.w * bytesPerPixel,
		Pixels: copied,
	})
	for i := range p.pix {
		p.pix[i] = whiteByte
	}
}

// Pages returns the pages collected by ShowPage.
func (p *Pixmap) Pages() []Image {
	out := make([]Image, len(p.pages))
	copy(out, p.pages)
	return out
}

func (p *Pixmap) set(col, row int, red, green, blue byte) {
	if col < 0 || row < 0 || col >= p.w || row >= p.h {
		return
	}
	i := row*p.w*bytesPerPixel + col*bytesPerPixel
	p.pix[i] = red
	p.pix[i+1] = green
	p.pix[i+2] = blue
}

func (p *Pixmap) strokeSegment(a, b Point, half float64, red, green, blue byte) {
	minX := math.Min(a.X, b.X) - half
	maxX := math.Max(a.X, b.X) + half
	minY := math.Min(a.Y, b.Y) - half
	maxY := math.Max(a.Y, b.Y) + half
	col0 := clampInt(int(math.Floor(minX)), p.w)
	col1 := clampInt(int(math.Ceil(maxX)), p.w)
	row0 := p.h - clampInt(int(math.Ceil(maxY)), p.h)
	row1 := p.h - clampInt(int(math.Floor(minY)), p.h)
	if row0 > row1 {
		row0, row1 = row1, row0
	}
	for row := row0; row < row1; row++ {
		centerY := float64(p.h-1-row) + pixelCenter
		for col := col0; col < col1; col++ {
			centerX := float64(col) + pixelCenter
			if distToSeg(centerX, centerY, a.X, a.Y, b.X, b.Y) <= half {
				p.set(col, row, red, green, blue)
			}
		}
	}
}

func colorByte(value float64) byte {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return byte(math.Round(value * colorScale))
}

func clampInt(value, hi int) int {
	if value < 0 {
		return 0
	}
	if value > hi {
		return hi
	}
	return value
}

func distToSeg(pointX, pointY, startX, startY, endX, endY float64) float64 {
	deltaX := endX - startX
	deltaY := endY - startY
	len2 := deltaX*deltaX + deltaY*deltaY
	if len2 == 0 {
		return math.Hypot(pointX-startX, pointY-startY)
	}
	param := ((pointX-startX)*deltaX + (pointY-startY)*deltaY) / len2
	if param < 0 {
		param = 0
	}
	if param > 1 {
		param = 1
	}
	return math.Hypot(pointX-(startX+param*deltaX), pointY-(startY+param*deltaY))
}

func segments(pts []Point) [][2]Point {
	out := make([][2]Point, 0, len(pts))
	var prev Point
	has := false
	for _, point := range pts {
		if point.Move || !has {
			prev = point
			has = true
			continue
		}
		out = append(out, [2]Point{prev, point})
		prev = point
	}
	return out
}

func subpaths(pts []Point) [][]Point {
	subs := make([][]Point, 0, len(pts))
	cur := make([]Point, 0, len(pts))
	for _, point := range pts {
		if point.Move && len(cur) > 0 {
			subs = append(subs, cur)
			cur = nil
		}
		cur = append(cur, point)
	}
	if len(cur) > 0 {
		subs = append(subs, cur)
	}
	return subs
}

func inside(subs [][]Point, pointX, pointY float64, evenOdd bool) bool {
	winding := 0
	for _, sub := range subs {
		if len(sub) < minFillPoints {
			continue
		}
		for i := range sub {
			start := sub[i]
			end := sub[(i+1)%len(sub)]
			if start.X == end.X && start.Y == end.Y {
				continue
			}
			hit, dir := cross(pointX, pointY, start.X, start.Y, end.X, end.Y)
			if hit {
				winding += dir
			}
		}
	}
	if evenOdd {
		return winding%2 != 0
	}
	return winding != 0
}

func cross(pointX, pointY, startX, startY, endX, endY float64) (bool, int) {
	if startY == endY || pointY < math.Min(startY, endY) || pointY >= math.Max(startY, endY) {
		return false, 0
	}
	atX := startX + (pointY-startY)/(endY-startY)*(endX-startX)
	if atX <= pointX {
		return false, 0
	}
	if endY > startY {
		return true, 1
	}
	return true, -1
}
