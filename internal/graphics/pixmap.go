package graphics

import (
	"image"
	"math"
)

const (
	bytesPerPixel = 3
	whiteByte     = 255
	colorScale    = 255
	pixelCenter   = 0.5
	halfWidth     = 2
	minFillPoints = 2
	byteShift     = 8
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
	// Walk adjacent pairs directly. A Move point starts a new subpath and
	// breaks the pair, so no per-stroke segment slice is built.
	for i := 1; i < len(pts); i++ {
		if pts[i].Move {
			continue
		}
		p.strokeSegment(pts[i-1], pts[i], half, redByte, greenByte, blueByte)
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

// DrawImage stamps one image. The image unit square maps through ctm, then
// scales by scale to device pixels. Sampling is nearest neighbor with image
// row 0 at the top of the square, and the image alpha is ignored.
func (p *Pixmap) DrawImage(pic image.Image, ctm Matrix, scale float64) {
	if pic == nil {
		return
	}
	bounds := pic.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return
	}
	imageToDevice := Concat(ctm, Matrix{A: scale, B: 0, C: 0, D: scale, E: 0, F: 0})
	toImage, ok := imageToDevice.Invert()
	if !ok {
		return
	}
	col0, col1, row0, row1 := p.imageSpan(imageToDevice)
	for row := row0; row < row1; row++ {
		for col := col0; col < col1; col++ {
			srcX, srcY, ok := p.imagePixel(toImage, width, height, col, row)
			if !ok {
				continue
			}
			red, green, blue, _ := pic.At(bounds.Min.X+srcX, bounds.Min.Y+srcY).RGBA()
			p.set(col, row, byte(red>>byteShift), byte(green>>byteShift), byte(blue>>byteShift))
		}
	}
}

// imagePixel maps one device pixel center back to an image pixel. The bool is
// false when the center falls outside the image unit square.
func (p *Pixmap) imagePixel(toImage Matrix, width, height, col, row int) (int, int, bool) {
	posX, posY := toImage.Apply(float64(col)+pixelCenter, float64(p.h-1-row)+pixelCenter)
	if posX < 0 || posX >= 1 || posY < 0 || posY >= 1 {
		return 0, 0, false
	}
	return int(posX * float64(width)), height - 1 - int(posY*float64(height)), true
}

// DrawGlyph blends one glyph coverage mask. Mask row 0 is the top row, and
// the mask's bottom-left pixel sits at device pixel (originX, originY).
// Alpha 0 leaves the pixel and 255 replaces it with the color.
func (p *Pixmap) DrawGlyph(mask *image.Alpha, originX, originY int, red, green, blue float64) {
	if mask == nil {
		return
	}
	bounds := mask.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	redByte, greenByte, blueByte := colorByte(red), colorByte(green), colorByte(blue)
	baseRow := p.h - height - originY
	for row := range height {
		for col := range width {
			alpha := mask.AlphaAt(bounds.Min.X+col, bounds.Min.Y+row).A
			p.blend(originX+col, baseRow+row, redByte, greenByte, blueByte, alpha)
		}
	}
}

func (p *Pixmap) blend(col, row int, red, green, blue, alpha byte) {
	if alpha == 0 || col < 0 || row < 0 || col >= p.w || row >= p.h {
		return
	}
	offset := row*p.w*bytesPerPixel + col*bytesPerPixel
	p.pix[offset] = blendByte(p.pix[offset], red, int(alpha))
	p.pix[offset+1] = blendByte(p.pix[offset+1], green, int(alpha))
	p.pix[offset+2] = blendByte(p.pix[offset+2], blue, int(alpha))
}

// blendByte returns dst scaled by the coverage plus src, rounded.
func blendByte(dst, src byte, alpha int) byte {
	return byte((int(dst)*(colorScale-alpha) + int(src)*alpha + colorScale/halfWidth) / colorScale)
}

// imageSpan returns the half-open pixel ranges the mapped unit square covers.
func (p *Pixmap) imageSpan(mat Matrix) (int, int, int, int) {
	minX, minY, maxX, maxY := unitBox(mat)
	col0 := clampInt(int(math.Floor(minX)), p.w)
	col1 := clampInt(int(math.Ceil(maxX)), p.w)
	row0 := p.h - clampInt(int(math.Ceil(maxY)), p.h)
	row1 := p.h - clampInt(int(math.Floor(minY)), p.h)
	if row0 > row1 {
		row0, row1 = row1, row0
	}
	return col0, col1, row0, row1
}

// unitBox returns the device bounds of the unit square mapped through mat.
func unitBox(mat Matrix) (float64, float64, float64, float64) {
	minX, minY := mat.Apply(0, 0)
	maxX, maxY := minX, minY
	for _, corner := range [][2]float64{{1, 0}, {0, 1}, {1, 1}} {
		posX, posY := mat.Apply(corner[0], corner[1])
		minX = math.Min(minX, posX)
		maxX = math.Max(maxX, posX)
		minY = math.Min(minY, posY)
		maxY = math.Max(maxY, posY)
	}
	return minX, minY, maxX, maxY
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
