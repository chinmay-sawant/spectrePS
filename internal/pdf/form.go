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
	if run.marker != nil {
		if _, ok := run.marker.(graphics.ClipMarker); !ok {
			return plan, NewError(opDo, errUndefined)
		}
	}
	plan.content = content
	plan.matrix = matrix
	plan.box = box
	plan.res = res
	plan.local = local
	return plan, nil
}

// execForm runs one planned form and restores the state around it.
func (run *runner) execForm(ctx context.Context, plan formPlan) error {
	saved := run.snap()
	stack := run.stack
	xobjects, fonts := run.xobjects, run.fonts
	extgstates, properties := run.extgstates, run.properties
	images, mcStack := run.images, run.mcStack
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
		run.images = nil
	}
	playErr := run.play(ctx, &scanner{src: plan.content, pos: 0})
	run.stack = stack
	run.xobjects, run.fonts = xobjects, fonts
	run.extgstates, run.properties = extgstates, properties
	run.images, run.mcStack = images, mcStack
	run.apply(saved)
	run.formDepth--
	return playErr
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
