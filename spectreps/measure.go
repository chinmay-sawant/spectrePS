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

// Ink is one RGB triple. MeasureInk fills it with occupancy, the fraction of
// pixels that mark each channel. MeasureInkAmount fills it with the weighted
// amount, the mean complement of each channel.
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

// MeasureInkAmount returns the mean complement of each RGB channel, the
// weighted ink amount. A channel byte c contributes (255 - c) / 255, and the
// mean runs over Width * Height. A white page is zero and a black page is one
// on every channel. Stride padding is ignored. A zero-size image returns the
// zero Ink.
func MeasureInkAmount(img PageImage) Ink {
	total := img.Width * img.Height
	if total == 0 {
		return Ink{R: 0, G: 0, B: 0}
	}
	var red, green, blue int64
	for row := range img.Height {
		base := row * img.Stride
		for col := range img.Width {
			i := base + col*rgbBytes
			red += int64(whiteByte - int(img.Pixels[i]))
			green += int64(whiteByte - int(img.Pixels[i+1]))
			blue += int64(whiteByte - int(img.Pixels[i+2]))
		}
	}
	n := float64(total) * whiteByte
	return Ink{
		R: float64(red) / n,
		G: float64(green) / n,
		B: float64(blue) / n,
	}
}
