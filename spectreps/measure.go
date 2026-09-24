package spectreps

import "math"

// whiteByte is an unmarked RGB channel.
const whiteByte = 255

// Box is the union of marked pixels in points, origin at the lower left.
type Box struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// Ink is the fraction of pixels that mark each RGB channel.
type Ink struct {
	R float64
	G float64
	B float64
}

// MeasureBox returns the bounding box of the marked pixels in points.
// dpi of 0 or less selects 72. The bool is false when no pixel is marked.
func MeasureBox(img PageImage, dpi float64) (Box, bool) {
	if dpi <= 0 {
		dpi = defaultDPI
	}
	pixel := float64(pointsPerInch) / dpi
	var box Box
	found := false
	for row := range img.Height {
		base := row * img.Stride
		for col := range img.Width {
			i := base + col*rgbBytes
			if !markedPixel(img.Pixels, i) {
				continue
			}
			left := float64(col) * pixel
			right := float64(col+1) * pixel
			bottom := float64(img.Height-row-1) * pixel
			top := float64(img.Height-row) * pixel
			if !found {
				box = Box{MinX: left, MinY: bottom, MaxX: right, MaxY: top}
				found = true
				continue
			}
			box.MinX = math.Min(box.MinX, left)
			box.MinY = math.Min(box.MinY, bottom)
			box.MaxX = math.Max(box.MaxX, right)
			box.MaxY = math.Max(box.MaxY, top)
		}
	}
	return box, found
}

func markedPixel(pixels []byte, i int) bool {
	if pixels[i] != whiteByte {
		return true
	}
	if pixels[i+1] != whiteByte {
		return true
	}
	return pixels[i+2] != whiteByte
}

// MeasureInk returns the fraction of pixels whose R, G, or B byte is not 255.
// The denominator is Width * Height. Stride padding is ignored.
func MeasureInk(img PageImage) Ink {
	total := img.Width * img.Height
	if total == 0 {
		return Ink{R: 0, G: 0, B: 0}
	}
	var red, green, blue int
	for row := range img.Height {
		base := row * img.Stride
		for col := range img.Width {
			i := base + col*rgbBytes
			if img.Pixels[i] != whiteByte {
				red++
			}
			if img.Pixels[i+1] != whiteByte {
				green++
			}
			if img.Pixels[i+2] != whiteByte {
				blue++
			}
		}
	}
	n := float64(total)
	return Ink{
		R: float64(red) / n,
		G: float64(green) / n,
		B: float64(blue) / n,
	}
}
