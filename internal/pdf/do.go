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
// content stream. Any other subtype is undefined in Do.
func (run *runner) do(ctx context.Context) error {
	name, err := run.popName(opDo)
	if err != nil {
		return err
	}
	val, ok := run.xobjects[name]
	if !ok {
		return NewError(opDo, errUndefined)
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
// run. The caller has already checked the image subtype.
func (run *runner) image(name string, val Value) (image.Image, error) {
	if pic, ok := run.images[name]; ok {
		return pic, nil
	}
	if hasSMask(val) {
		return nil, NewError(opDo, errUndefined)
	}
	pic, err := DecodeImageValue(val)
	if err != nil {
		return nil, NewError(opDo, errUndefined)
	}
	if run.images == nil {
		run.images = map[string]image.Image{}
	}
	run.images[name] = pic
	return pic, nil
}

// hasSMask reports whether an image carries an /SMask. The alpha mask is not
// in this subset, so the image is rejected rather than painted opaque.
func hasSMask(val Value) bool {
	entry, ok := val.ValueEntry(keySMask)
	return ok && entry.Kind != KindNull
}
