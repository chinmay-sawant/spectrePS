package spectreps_test

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

func TestMeasureBox(t *testing.T) {
	cases := []struct {
		name string
		img  spectreps.PageImage
		dpi  float64
		want spectreps.Box
		ok   bool
	}{
		{"bottom left pixel", withPixel(2, 2, 6, 0, 1, 0, 0, 0), 72, boxOf(0, 0, 1, 1), true},
		{"top left pixel", withPixel(2, 2, 6, 0, 0, 0, 0, 0), 72, boxOf(0, 1, 1, 2), true},
		{"two corners", withTwoPixels(3, 3), 72, boxOf(0, 0, 3, 3), true},
		{"144 dpi halves the box", withPixel(2, 2, 6, 0, 0, 0, 0, 0), 144, boxOf(0, 0.5, 0.5, 1), true},
		{"zero dpi selects 72", withPixel(2, 2, 6, 1, 1, 0, 0, 0), 0, boxOf(1, 0, 2, 1), true},
		{"negative dpi selects 72", withPixel(2, 2, 6, 1, 1, 0, 0, 0), -1, boxOf(1, 0, 2, 1), true},
		{"white page", blankImage(2, 2, 6), 72, boxOf(0, 0, 0, 0), false},
		{"stride padding is ignored", withPixel(2, 1, 8, 0, 0, 0, 0, 0), 72, boxOf(0, 0, 1, 1), true},
		{"padding alone does not count", blankImage(2, 1, 8), 72, boxOf(0, 0, 0, 0), false},
		{"empty image", spectreps.PageImage{}, 72, boxOf(0, 0, 0, 0), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := spectreps.MeasureBox(tt.img, tt.dpi)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("MeasureBox = %+v %v, want %+v %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestMeasureInk(t *testing.T) {
	cases := []struct {
		name string
		img  spectreps.PageImage
		want spectreps.Ink
	}{
		{
			name: "white page",
			img:  blankImage(2, 2, 6),
			want: spectreps.Ink{},
		},
		{
			name: "empty image",
			img:  spectreps.PageImage{},
			want: spectreps.Ink{},
		},
		{
			name: "one black pixel of four",
			img:  withPixel(2, 2, 6, 1, 1, 0, 0, 0),
			want: spectreps.Ink{R: 0.25, G: 0.25, B: 0.25},
		},
		{
			name: "cyan page",
			img:  withPixel(1, 1, 3, 0, 0, 0, 255, 255),
			want: spectreps.Ink{R: 1, G: 0, B: 0},
		},
		{
			name: "red page",
			img:  withPixel(1, 1, 3, 0, 0, 255, 0, 0),
			want: spectreps.Ink{R: 0, G: 1, B: 1},
		},
		{
			name: "stride padding is ignored",
			img:  blankImage(1, 1, 5),
			want: spectreps.Ink{},
		},
		{
			name: "half black half white",
			img:  withTwoBands(2, 1, 6),
			want: spectreps.Ink{R: 0.5, G: 0.5, B: 0.5},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := spectreps.MeasureInk(tt.img)
			if got != tt.want {
				t.Fatalf("MeasureInk = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMeasureInkAmount(t *testing.T) {
	gray := 127.0 / 255.0
	cases := []struct {
		name string
		img  spectreps.PageImage
		want spectreps.Ink
	}{
		{
			name: "white page",
			img:  blankImage(2, 2, 6),
			want: spectreps.Ink{},
		},
		{
			name: "empty image",
			img:  spectreps.PageImage{},
			want: spectreps.Ink{},
		},
		{
			name: "cyan page",
			img:  withPixel(1, 1, 3, 0, 0, 0, 255, 255),
			want: spectreps.Ink{R: 1, G: 0, B: 0},
		},
		{
			name: "red page",
			img:  withPixel(1, 1, 3, 0, 0, 255, 0, 0),
			want: spectreps.Ink{R: 0, G: 1, B: 1},
		},
		{
			name: "one black pixel of four",
			img:  withPixel(2, 2, 6, 1, 1, 0, 0, 0),
			want: spectreps.Ink{R: 0.25, G: 0.25, B: 0.25},
		},
		{
			name: "byte-128 gray page",
			img:  withGray(2, 1, 6, 128),
			want: spectreps.Ink{R: gray, G: gray, B: gray},
		},
		{
			name: "stride padding is ignored",
			img:  blankImage(1, 1, 5),
			want: spectreps.Ink{},
		},
		{
			name: "one black pixel of four with stride padding",
			img:  withPixel(2, 2, 8, 1, 1, 0, 0, 0),
			want: spectreps.Ink{R: 0.25, G: 0.25, B: 0.25},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := spectreps.MeasureInkAmount(tt.img)
			if got != tt.want {
				t.Fatalf("MeasureInkAmount = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidationMeasureEdges(t *testing.T) {
	t.Run("black page amount", func(t *testing.T) {
		got := spectreps.MeasureInkAmount(withGray(2, 2, 6, 0))
		want := spectreps.Ink{R: 1, G: 1, B: 1}
		if got != want {
			t.Fatalf("MeasureInkAmount(black) = %+v, want %+v", got, want)
		}
	})
	t.Run("one channel marks box", func(t *testing.T) {
		img := withPixel(2, 2, 6, 0, 0, 255, 255, 254)
		got, ok := spectreps.MeasureBox(img, 72)
		if !ok || got != boxOf(0, 1, 1, 2) {
			t.Fatalf("MeasureBox(one low channel) = %+v %v, want %+v true", got, ok, boxOf(0, 1, 1, 2))
		}
	})
	t.Run("zero size amount", func(t *testing.T) {
		got := spectreps.MeasureInkAmount(spectreps.PageImage{})
		if got != (spectreps.Ink{}) {
			t.Fatalf("MeasureInkAmount(zero image) = %+v, want zero Ink", got)
		}
	})
}

// blankImage is a white image with stride padding left at zero, so a scan that
// reads padding as pixels reports a marked page.
func blankImage(width, height, stride int) spectreps.PageImage {
	img := spectreps.PageImage{
		Width:  width,
		Height: height,
		Stride: stride,
		Pixels: make([]byte, height*stride),
	}
	for row := range height {
		base := row * stride
		for col := range width * 3 {
			img.Pixels[base+col] = 255
		}
	}
	return img
}

func boxOf(minX, minY, maxX, maxY float64) spectreps.Box {
	return spectreps.Box{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
}

// withGray fills the width by height corner with one gray byte per channel.
func withGray(width, height, stride int, value byte) spectreps.PageImage {
	img := blankImage(width, height, stride)
	for row := range height {
		base := row * stride
		for col := range width {
			i := base + col*3
			img.Pixels[i] = value
			img.Pixels[i+1] = value
			img.Pixels[i+2] = value
		}
	}
	return img
}

func withPixel(width, height, stride, x, y int, red, green, blue byte) spectreps.PageImage {
	img := blankImage(width, height, stride)
	i := y*stride + x*3
	img.Pixels[i] = red
	img.Pixels[i+1] = green
	img.Pixels[i+2] = blue
	return img
}

func withTwoPixels(width, height int) spectreps.PageImage {
	img := blankImage(width, height, width*3)
	first := 0
	last := (height-1)*img.Stride + (width-1)*3
	for i := range 3 {
		img.Pixels[first+i] = 0
		img.Pixels[last+i] = 0
	}
	return img
}

func withTwoBands(width, height, stride int) spectreps.PageImage {
	img := blankImage(width, height, stride)
	img.Pixels[0] = 0
	img.Pixels[1] = 0
	img.Pixels[2] = 0
	return img
}
