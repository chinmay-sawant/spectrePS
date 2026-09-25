package pdf

import "image"

// opDo is the content operator name for one XObject paint.
const opDo = "Do"

// takeDo dispatches the Do operator.
func (run *runner) takeDo(opName string) (bool, error) {
	if opName != opDo {
		return false, nil
	}
	return true, run.do()
}

// do pops one name, resolves it in the page /XObject resources, and stamps
// the decoded image through the current matrix.
// A missing name, a non-image subtype, and a decode error are undefined in Do.
func (run *runner) do() error {
	name, err := run.popName(opDo)
	if err != nil {
		return err
	}
	pic, err := run.image(name)
	if err != nil {
		return err
	}
	if run.marker != nil {
		run.marker.DrawImage(pic, run.ctm, run.scale)
	}
	return nil
}

// image decodes one /XObject name once and caches the pixels for the run.
func (run *runner) image(name string) (image.Image, error) {
	if pic, ok := run.images[name]; ok {
		return pic, nil
	}
	val, ok := run.xobjects[name]
	if !ok || hasSMask(val) {
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
