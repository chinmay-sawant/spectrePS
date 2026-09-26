package pdf

import (
	"context"
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// maxFormDepth caps nested form XObject execution. The 33rd nested form is
// limitcheck in Do.
const (
	nameForm     = "Form"
	keyFormType  = "FormType"
	keyBBox      = "BBox"
	keyMatrix    = "Matrix"
	maxFormDepth = 32
	bboxLen      = 4
	formTypeOne  = 1

	// Transparency group keys and the two soft mask subtypes this subset
	// builds. A group /CA of 1 is opaque, and /I and /K are read and validated
	// but every supported group composites as an isolated group. keyS and keyK
	// are shared with the structure tree reader.
	keyGroup         = "Group"
	keyG             = "G"
	keyI             = "I"
	keyCA            = "CA"
	keyTR            = "TR"
	nameTransparency = "Transparency"
	nameAlpha        = "Alpha"
	nameLuminosity   = "Luminosity"
	nameNone         = "None"
	nameIdentity     = "Identity"

	// The scratch page caps match the page caps in documentation/language.md.
	maxScratchSide   = 20000
	maxScratchPixels = 40000000

	// luminosityRed, luminosityGreen, and luminosityBlue are the ISO 32000-1
	// luminosity weights for a /Luminosity soft mask.
	luminosityRed   = 0.3
	luminosityGreen = 0.59
	luminosityBlue  = 0.11
)

// isFormXObject reports whether a resolved XObject value is a form.
func isFormXObject(val Value) bool {
	if val.Kind != KindStream && val.Kind != KindDict {
		return false
	}
	name, ok := val.NameEntry(keySubtype)
	return ok && name == nameForm
}

// formPlan is one validated form XObject ready to execute.
type formPlan struct {
	content []byte
	matrix  graphics.Matrix
	box     [bboxLen]float64
	res     Resources
	local   bool
	group   *formGroup
}

// formGroup is one validated /Group /S /Transparency entry. alpha is the
// group constant alpha; isolated and knockout are read so a /I or /K value
// cannot slip through unvalidated. This subset composites every group once
// into a scratch page, which is the isolated shape.
type formGroup struct {
	alpha    float64
	isolated bool
	knockout bool
}

// runForm executes one form XObject. The form /Matrix concatenates with the
// current CTM, /BBox becomes a clip on the form's marks, and a form /Resources
// subdictionary replaces the current resources. A form without /Resources
// inherits the page resources. The form runs inside an implicit q/Q, so its
// state changes do not reach the caller.
func (run *runner) runForm(ctx context.Context, val Value) error {
	if run.formDepth >= maxFormDepth {
		return NewError(opDo, errLimit)
	}
	plan, err := run.formPlan(val)
	if err != nil {
		return err
	}
	return run.execForm(ctx, plan)
}

// formPlan validates one form XObject and reads its content, matrix, box, and
// local resources. A marker without graphics.ClipMarker refuses with undefined
// in Do because the /BBox clip cannot be expressed.
func (run *runner) formPlan(val Value) (formPlan, error) {
	var plan formPlan
	content, err := formContent(val)
	if err != nil {
		return plan, err
	}
	if err = formType(val); err != nil {
		return plan, err
	}
	matrix, err := run.formMatrix(val)
	if err != nil {
		return plan, err
	}
	box, err := run.formBBox(val)
	if err != nil {
		return plan, err
	}
	res, local, err := run.formResources(val)
	if err != nil {
		return plan, err
	}
	group, err := run.formGroup(val)
	if err != nil {
		return plan, err
	}
	if err := run.formSeams(group); err != nil {
		return plan, err
	}
	plan.content = content
	plan.matrix = matrix
	plan.box = box
	plan.res = res
	plan.local = local
	plan.group = group
	return plan, nil
}

// formSeams requires the marker to implement the clip seam every form needs
// and the group seam a transparency group needs. A marker without them keeps
// its named refusal.
func (run *runner) formSeams(group *formGroup) error {
	if run.marker == nil {
		return nil
	}
	if _, ok := run.marker.(graphics.ClipMarker); !ok {
		return NewError(opDo, errUndefined)
	}
	if group != nil {
		if _, ok := run.marker.(graphics.GroupMarker); !ok {
			return NewError(opDo, errUndefined)
		}
	}
	return nil
}

// formGroup validates one /Group entry. An absent or null entry returns nil.
// /S must be Transparency, /I and /K must be booleans, /CA must be a number
// that clamps, and the /CS space must resolve. Any other key refuses with
// undefined in Do instead of ignoring a group property.
func (run *runner) formGroup(val Value) (*formGroup, error) {
	entry, ok := val.ValueEntry(keyGroup)
	if !ok || entry.Kind == KindNull {
		return nil, nil //nolint:nilnil // an absent /Group is not a group
	}
	node, err := run.derefValue(entry, opDo)
	if err != nil {
		return nil, err
	}
	if node.Kind != KindDict {
		return nil, NewError(opDo, errUndefined)
	}
	subtype, ok := node.NameEntry(keyS)
	if !ok || subtype != nameTransparency {
		return nil, NewError(opDo, errUndefined)
	}
	group := &formGroup{alpha: 1, isolated: false, knockout: false}
	for key, item := range node.Dict {
		if err := group.readEntry(run, key, item); err != nil {
			return nil, err
		}
	}
	return group, nil
}

// readEntry reads one /Group dictionary entry.
func (group *formGroup) readEntry(run *runner, key string, item Value) error { //nolint:cyclop // one case per group key
	switch key {
	case keyType, keyS:
		return nil
	case "CS":
		if _, err := run.file.resolveColorSpace(item, opDo); err != nil {
			return err
		}
		return nil
	case "I":
		if item.Kind != KindBool {
			return NewError(opDo, errUndefined)
		}
		group.isolated = item.Bool
		return nil
	case "K":
		if item.Kind != KindBool {
			return NewError(opDo, errUndefined)
		}
		group.knockout = item.Bool
		return nil
	case "CA":
		number, ok := valueNum(item)
		if !ok {
			return NewError(opDo, errUndefined)
		}
		group.alpha = clampNumber(number, 0, 1)
		return nil
	default:
		return NewError(opDo, errUndefined)
	}
}

// execForm runs one planned form and restores the state around it. A
// transparency group renders into a scratch page and composites once.
func (run *runner) execForm(ctx context.Context, plan formPlan) error {
	if plan.group == nil {
		return run.execFormPlain(ctx, plan)
	}
	return run.execFormGroup(ctx, plan)
}

// execFormPlain runs one planned form in the current state. The form runs
// inside an implicit q/Q, so its state changes do not reach the caller.
func (run *runner) execFormPlain(ctx context.Context, plan formPlan) error {
	saved := run.snap()
	stack := run.stack
	skipDepth := run.skipDepth
	xobjects, fonts := run.xobjects, run.fonts
	extgstates, properties := run.extgstates, run.properties
	colors := run.colors
	images, masks, mcStack := run.images, run.masks, run.mcStack
	run.formDepth++
	run.stack = nil
	run.mcStack = nil
	run.ctm = graphics.Concat(plan.matrix, run.ctm)
	run.clips = append(run.clips, run.boxClip(plan.box))
	if plan.local {
		run.xobjects = plan.res.XObjects
		run.fonts = plan.res.Fonts
		run.extgstates = plan.res.ExtGStates
		run.properties = plan.res.Properties
		run.colors = plan.res.Colors
		run.images = nil
		run.masks = nil
	}
	playErr := run.play(ctx, &scanner{src: plan.content, pos: 0})
	run.stack = stack
	run.skipDepth = skipDepth
	run.xobjects, run.fonts = xobjects, fonts
	run.extgstates, run.properties = extgstates, properties
	run.colors = colors
	run.images, run.masks, run.mcStack = images, masks, mcStack
	run.apply(saved)
	run.formDepth--
	return playErr
}

// execFormGroup renders a transparency group into a scratch page the size of
// the target device and composites it once with the group alpha, the current
// blend mode, and the current fill alpha. The scratch obeys the page pixel
// and side caps.
func (run *runner) execFormGroup(ctx context.Context, plan formPlan) error {
	if run.marker == nil {
		return nil
	}
	host, ok := run.marker.(graphics.GroupMarker)
	if !ok {
		return NewError(opDo, errUndefined)
	}
	width, height := host.PageSize()
	scratch, err := newScratch(width, height)
	if err != nil {
		return err
	}
	if err := run.renderPlanned(ctx, plan, scratch); err != nil {
		return err
	}
	alpha := plan.group.alpha * run.fillAlpha
	if len(run.clips) == 0 {
		host.CompositeGroup(scratch, alpha, run.blendMode)
		return nil
	}
	host.CompositeGroupClipped(run.clips, scratch, alpha, run.blendMode)
	return nil
}

// renderPlanned executes one validated form onto a fresh marker through a
// sub-runner that shares the file and resources of the caller.
func (run *runner) renderPlanned(ctx context.Context, plan formPlan, marker graphics.Marker) error {
	sub := run.subRunner(marker)
	sub.formDepth = run.formDepth
	return sub.execFormPlain(ctx, plan)
}

// subRunner builds a runner that shares the caller's file, resources, and
// output seams but starts with a fresh graphics state. A group or soft mask
// render starts transparent.
func (run *runner) subRunner(marker graphics.Marker) *runner {
	sub := newRunner(marker, run.scale)
	sub.file = run.file
	sub.xobjects = run.xobjects
	sub.fonts = run.fonts
	sub.extgstates = run.extgstates
	sub.properties = run.properties
	sub.colors = run.colors
	sub.ocOff = run.ocOff
	sub.ctm = run.ctm
	sub.sink = run.sink
	sub.runs = run.runs
	sub.mcSink = run.mcSink
	return sub
}

// newScratch allocates one transparent group page under the page caps.
func newScratch(width, height int) (*graphics.Pixmap, error) {
	if !scratchFits(width, height) {
		return nil, NewError(opDo, errLimit)
	}
	return graphics.NewGroupPixmap(width, height), nil
}

// scratchFits reports whether a scratch page is inside the side and pixel
// caps. The product runs in int64 so a large side cannot overflow.
func scratchFits(width, height int) bool {
	if width <= 0 || height <= 0 || width > maxScratchSide || height > maxScratchSide {
		return false
	}
	return int64(width)*int64(height) <= int64(maxScratchPixels)
}

// formContent decodes the form content stream. A form that is not a stream is
// undefined in Do.
func formContent(val Value) ([]byte, error) {
	if val.Kind != KindStream {
		return nil, NewError(opDo, errUndefined)
	}
	content, err := decodeStream(val)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// formType accepts /FormType 1 and the absent default. Any other value is
// undefined in Do.
func formType(val Value) error {
	entry, ok := val.ValueEntry(keyFormType)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	if number, ok := valueNum(entry); ok && number == formTypeOne {
		return nil
	}
	return NewError(opDo, errUndefined)
}

// formMatrix returns the form /Matrix, or the identity when absent. A missing
// or malformed matrix is undefined in Do.
func (run *runner) formMatrix(val Value) (graphics.Matrix, error) {
	entry, ok := val.ValueEntry(keyMatrix)
	if !ok || entry.Kind == KindNull {
		return graphics.Identity(), nil
	}
	entry, err := run.derefValue(entry, opDo)
	if err != nil {
		return graphics.Identity(), err
	}
	vals, ok := numberArray(entry, matrixLen)
	if !ok {
		return graphics.Identity(), NewError(opDo, errUndefined)
	}
	return graphics.Matrix{
		A: vals[0],
		B: vals[1],
		C: vals[2],
		D: vals[3],
		E: vals[4],
		F: vals[5],
	}, nil
}

// formBBox returns the /BBox corners as x0 y0 x1 y1, normalized so a reversed
// box still clips. A missing or malformed box is undefined in Do.
func (run *runner) formBBox(val Value) ([bboxLen]float64, error) {
	entry, ok := val.ValueEntry(keyBBox)
	if !ok || entry.Kind == KindNull {
		return [bboxLen]float64{}, NewError(opDo, errUndefined)
	}
	entry, err := run.derefValue(entry, opDo)
	if err != nil {
		return [bboxLen]float64{}, err
	}
	vals, ok := numberArray(entry, bboxLen)
	if !ok {
		return [bboxLen]float64{}, NewError(opDo, errUndefined)
	}
	return [bboxLen]float64{
		math.Min(vals[0], vals[2]), math.Min(vals[1], vals[3]),
		math.Max(vals[0], vals[2]), math.Max(vals[1], vals[3]),
	}, nil
}

// formResources resolves a form's own /Resources. The bool is false when the
// form has none, so the caller keeps the current resources. A form that names
// resources without a file to resolve them refuses with undefined in Do.
func (run *runner) formResources(val Value) (Resources, bool, error) {
	entry, ok := val.ValueEntry(keyResources)
	if !ok || entry.Kind == KindNull {
		return emptyResources(), false, nil
	}
	if run.file == nil {
		return emptyResources(),
			false, NewError(opDo, errUndefined)
	}
	res, err := run.file.resolveResources(entry)
	if err != nil {
		return emptyResources(), false, err
	}
	return res, true, nil
}

// boxClip turns a /BBox into a device-space clip for the form's marks. The
// runner CTM already carries the form /Matrix when this runs.
func (run *runner) boxClip(box [bboxLen]float64) graphics.Clip {
	path := []point{
		{posX: box[0], posY: box[1], move: true},
		{posX: box[2], posY: box[1], move: false},
		{posX: box[2], posY: box[3], move: false},
		{posX: box[0], posY: box[3], move: false},
	}
	return graphics.Clip{Pts: run.devicePath(path), EvenOdd: false}
}

// stateSoftMask builds the coverage plane for one /ExtGState /SMask entry.
// /S /Alpha takes the group coverage and /S /Luminosity the ISO 32000-1
// luminosity of the group composited on black. /None clears the mask, and a
// /TR other than Identity refuses. The /G form is a Form XObject rendered
// into a scratch page with the page pixel and side caps. A marker the group
// cannot reach, such as the rewrite recorder, refuses with undefined in gs.
func (run *runner) stateSoftMask(ctx context.Context, entry Value, opName string) ([]byte, error) {
	if entry.Kind == KindName && entry.Name == nameNone {
		return nil, nil
	}
	node, err := run.derefValue(entry, opName)
	if err != nil {
		return nil, err
	}
	subtype, group, err := softMaskParams(node, opName)
	if err != nil {
		return nil, err
	}
	if run.marker == nil {
		return nil, nil
	}
	return run.renderSoftMask(ctx, subtype, group, opName)
}

// renderSoftMask renders the /G form of one soft mask into a scratch page and
// returns the coverage plane.
func (run *runner) renderSoftMask(
	ctx context.Context, subtype string, group Value, opName string,
) ([]byte, error) {
	host, ok := run.marker.(graphics.GroupMarker)
	if !ok {
		return nil, NewError(opName, errUndefined)
	}
	width, height := host.PageSize()
	scratch, err := newScratch(width, height)
	if err != nil {
		return nil, NewError(opName, errLimit)
	}
	form, err := run.derefValue(group, opName)
	if err != nil {
		return nil, err
	}
	sub := run.subRunner(scratch)
	plan, err := sub.formPlan(form)
	if err != nil {
		return nil, err
	}
	sub.formDepth = run.formDepth
	if err := sub.execFormPlain(ctx, plan); err != nil {
		return nil, err
	}
	return maskPlane(scratch, subtype), nil
}

// softMaskParams validates one /SMask dictionary and returns its subtype and
// /G form value. /S must be Alpha or Luminosity, /TR must be identity, and
// any other key refuses with undefined in opName.
func softMaskParams(node Value, opName string) (string, Value, error) {
	if node.Kind != KindDict {
		return "", NullVal(), NewError(opName, errUndefined)
	}
	subtype, ok := node.NameEntry(keyS)
	if !ok || (subtype != nameAlpha && subtype != nameLuminosity) {
		return "", NullVal(), NewError(opName, errUndefined)
	}
	if err := softMaskKeys(node, opName); err != nil {
		return "", NullVal(), err
	}
	group, ok := node.ValueEntry(keyG)
	if !ok || group.Kind == KindNull {
		return "", NullVal(), NewError(opName, errUndefined)
	}
	return subtype, group, nil
}

// softMaskKeys accepts the four SMask keys and requires /TR to be identity.
// Any other key refuses with undefined in opName.
func softMaskKeys(node Value, opName string) error {
	for key := range node.Dict {
		switch key {
		case keyType, keyS, keyG, keyTR:
		default:
			return NewError(opName, errUndefined)
		}
	}
	if transfer, present := node.ValueEntry(keyTR); present {
		return identityTransfer(transfer, opName)
	}
	return nil
}

// maskPlane turns one rendered soft mask group into the coverage plane later
// marks sample. /Alpha is the group coverage. /Luminosity is the ISO 32000-1
// weighted channel sum times the coverage, so an untouched pixel stays 0.
func maskPlane(scratch *graphics.Pixmap, subtype string) []byte {
	alpha := scratch.AlphaPlane()
	if subtype == nameAlpha {
		return alpha
	}
	pixels := scratch.PixelPlane()
	mask := make([]byte, len(alpha))
	for idx, coverage := range alpha {
		if coverage == 0 {
			continue
		}
		at := idx * rgbComponents
		luminosity := float64(pixels[at])*luminosityRed +
			float64(pixels[at+1])*luminosityGreen +
			float64(pixels[at+2])*luminosityBlue
		mask[idx] = previewByte(luminosity / colorSampleScale * float64(coverage) / colorSampleScale)
	}
	return mask
}

// numberArray returns n numbers from an array value.
func numberArray(val Value, count int) ([]float64, bool) {
	if val.Kind != KindArray || len(val.Array) != count {
		return nil, false
	}
	nums := make([]float64, count)
	for idx, item := range val.Array {
		number, ok := valueNum(item)
		if !ok {
			return nil, false
		}
		nums[idx] = number
	}
	return nums, true
}
