package pdf

import (
	"context"
	"image"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// opDo is the content operator name for one XObject paint.
const opDo = "Do"

// takeDo dispatches the Do operator.
func (run *runner) takeDo(ctx context.Context, opName string) (bool, error) {
	if opName != opDo {
		return false, nil
	}
	return true, run.do(ctx)
}

// do pops one name and resolves it in the page /XObject resources. An image
// XObject stamps through the current matrix. A /Subtype /Form XObject runs its
// content stream. Any other subtype is undefined in Do. An XObject whose /OC
// names an OFF group is skipped, as if it were absent from the stream.
func (run *runner) do(ctx context.Context) error {
	name, err := run.popName(opDo)
	if err != nil {
		return err
	}
	val, ok := run.xobjects[name]
	if !ok {
		return NewError(opDo, errUndefined)
	}
	hidden, err := run.xobjectHidden(val)
	if err != nil {
		return err
	}
	if hidden {
		return nil
	}
	if isFormXObject(val) {
		return run.runForm(ctx, val)
	}
	if !hasImageSubtype(val) {
		return NewError(opDo, errUndefined)
	}
	run.nameImage(name, val)
	pic, err := run.image(name, val)
	if err != nil {
		return err
	}
	if run.marker != nil {
		run.drawImage(pic)
	}
	return nil
}

// nameImage tells a marker the resource name and the resolved XObject
// dictionary before decode. A marker without the seam keeps today's behavior.
func (run *runner) nameImage(name string, val Value) {
	if !activeSink(run.marker) {
		return
	}
	recorder, ok := run.marker.(ImageNameMarker)
	if !ok {
		return
	}
	recorder.ImageName(name, val)
}

// drawImage stamps one decoded image through every active clip.
func (run *runner) drawImage(pic image.Image) {
	if len(run.clips) == 0 {
		run.marker.DrawImage(pic, run.ctm, run.scale)
		return
	}
	target, ok := run.marker.(graphics.ClipMarker)
	if !ok {
		return
	}
	target.DrawImageClipped(run.clips, pic, run.ctm, run.scale)
}

// image decodes one resolved image XObject once and caches the pixels for the
// run. The caller has already checked the image subtype. An /ImageMask true
// image decodes once to its coverage plane and is tinted with the current
// fill color at every Do, because the color can change between paints.
func (run *runner) image(name string, val Value) (image.Image, error) {
	if imageMaskFlag(val) {
		mask, err := run.imageMask(name, val)
		if err != nil {
			return nil, NewError(opDo, errUndefined)
		}
		return tintMask(mask, run.red, run.green, run.blue), nil
	}
	if pic, ok := run.images[name]; ok {
		return pic, nil
	}
	pic, err := run.decodeImage(val)
	if err != nil {
		return nil, NewError(opDo, errUndefined)
	}
	if run.images == nil {
		run.images = map[string]image.Image{}
	}
	run.images[name] = pic
	return pic, nil
}

// decodeImage decodes one value through the file when the caller has one, so
// an indirect /SMask, /Mask, or /ColorSpace reference resolves.
func (run *runner) decodeImage(val Value) (image.Image, error) {
	if run.file == nil {
		return DecodeImageValue(val)
	}
	return run.file.DecodeImageValueOp(val, opDo)
}

// imageMask decodes and caches one /ImageMask coverage plane.
func (run *runner) imageMask(name string, val Value) (*image.Alpha, error) {
	if mask, ok := run.masks[name]; ok {
		return mask, nil
	}
	var (
		mask *image.Alpha
		err  error
	)
	if run.file == nil {
		mask, err = DecodeImageMaskValue(val)
	} else {
		mask, err = run.file.DecodeImageMaskValueOp(val, opDo)
	}
	if err != nil {
		return nil, err
	}
	if run.masks == nil {
		run.masks = map[string]*image.Alpha{}
	}
	run.masks[name] = mask
	return mask, nil
}

// tintMask builds the RGBA image an image mask paints: the current fill color
// where the coverage plane is set, transparent elsewhere. DrawImage composites
// the alpha, so a 0 pixel leaves the page.
func tintMask(mask *image.Alpha, red, green, blue float64) *image.RGBA {
	bounds := mask.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	fill := [rgbaComponents]byte{previewByte(red), previewByte(green), previewByte(blue), 0}
	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			at := row*out.Stride + col*rgbaComponents
			copy(out.Pix[at:at+rgbaComponents], fill[:])
			out.Pix[at+3] = mask.AlphaAt(bounds.Min.X+col, bounds.Min.Y+row).A
		}
	}
	return out
}
