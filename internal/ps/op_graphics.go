package ps

import (
	"context"
	"math"
	"sync"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	defaultLineWidth = 1
	maxGSaveDepth    = 32
	halfTurnDegrees  = 180
	fullTurnDegrees  = 360
	maxLineCap       = 2
	opSetRGB         = "setrgbcolor"
	opSetLineCap     = "setlinecap"
	opRectFill       = "rectfill"
	opRectStroke     = "rectstroke"
	opDTransform     = "dtransform"
	paintStroke      = "stroke"
	paintFill        = "fill"
	paintEOFill      = "eofill"
)

// gsByInterp stores graphics state for each interpreter.
// Phase 04 replaces this recorder with the pixmap device and must keep user-space currentpoint.
//
//nolint:gochecknoglobals // Interp has no graphics field until phase 04
var (
	gsMu       sync.Mutex
	gsByInterp = map[*Interp]*gstate{}
)

// matrix is the CTM, user space to device space.
// x' = a*x + c*y + e. y' = b*x + d*y + f.
type matrix struct {
	a float64
	b float64
	c float64
	d float64
	e float64
	f float64
}

// devPt is one device-space path point.
type devPt struct {
	x    float64
	y    float64
	move bool
}

// paintMark records one of stroke, fill, or eofill.
// eofill is stored separately from fill.
type paintMark struct {
	kind    string
	evenOdd bool
}

// gstateSnap is the graphics state saved by gsave, without the save stack.
type gstateSnap struct {
	ctm      matrix
	path     []devPt
	hasPt    bool
	devX     float64
	devY     float64
	subX     float64
	subY     float64
	subOpen  bool
	width    float64
	lineCap  int
	gray     float64
	red      float64
	green    float64
	blue     float64
	fontName string
	fontSize float64
	clips    []clipPath
}

// gstate is the phase 03 path recorder.
// Path points are device space. currentpoint applies the inverse CTM.
type gstate struct {
	ctm         matrix
	path        []devPt
	hasPt       bool
	devX        float64
	devY        float64
	subX        float64
	subY        float64
	subOpen     bool
	width       float64
	lineCap     int
	gray        float64
	red         float64
	green       float64
	blue        float64
	fontName    string
	fontSize    float64
	clips       []clipPath
	saves       []*gstateSnap
	pages       int
	pix         *graphics.Pixmap
	scale       float64
	pageW       float64
	pageH       float64
	strokeCount int
	fillCount   int
	eoFillCount int
	marks       []paintMark
}

func registerGraphicsOps(interp *Interp) {
	interp.Install("moveto", opMoveto)
	interp.Install("rmoveto", opRmoveto)
	interp.Install("lineto", opLineto)
	interp.Install("rlineto", opRlineto)
	interp.Install("curveto", opCurveto)
	interp.Install("rcurveto", opRcurveto)
	interp.Install("closepath", opClosepath)
	interp.Install("newpath", opNewpath)
	interp.Install("currentpoint", opCurrentPoint)
	interp.Install("stroke", opStroke)
	interp.Install("fill", opFill)
	interp.Install("eofill", opEOFill)
	interp.Install("setlinewidth", opSetLineWidth)
	interp.Install(opSetLineCap, opSetLineCapRun)
	interp.Install("setrgbcolor", opSetRGBColor)
	interp.Install("setgray", opSetGray)
	interp.Install(opRectFill, opRectFillRun)
	interp.Install(opRectStroke, opRectStrokeRun)
	interp.Install(opDTransform, opDTransformRun)
	interp.Install("gsave", opGSave)
	interp.Install("grestore", opGRestore)
	interp.Install("showpage", opShowPage)
	interp.Install("translate", opTranslate)
	interp.Install("scale", opScale)
	interp.Install("rotate", opRotate)
	interp.Install("concat", opConcat)
	interp.Install("setmatrix", opSetMatrix)
	interp.Install("currentmatrix", opCurrentMatrix)
	registerArcOps(interp)
	registerClipOps(interp)
	registerInfoOps(interp)
	registerTextOps(interp)
}

func opMoveto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	userX, userY, err := popXY(interp, "moveto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	devX, devY := state.ctm.apply(userX, userY)
	return state.moveTo(devX, devY)
}

func opRmoveto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deltaX, deltaY, err := popXY(interp, "rmoveto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	userX, userY, err := state.userPoint("rmoveto")
	if err != nil {
		return err
	}
	devX, devY := state.ctm.apply(userX+deltaX, userY+deltaY)
	return state.moveTo(devX, devY)
}

func opLineto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	userX, userY, err := popXY(interp, "lineto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	if err := state.requirePoint("lineto"); err != nil {
		return err
	}
	devX, devY := state.ctm.apply(userX, userY)
	return state.addPoints([]devPt{{x: devX, y: devY, move: false}})
}

func opRlineto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deltaX, deltaY, err := popXY(interp, "rlineto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	userX, userY, err := state.userPoint("rlineto")
	if err != nil {
		return err
	}
	devX, devY := state.ctm.apply(userX+deltaX, userY+deltaY)
	return state.addPoints([]devPt{{x: devX, y: devY, move: false}})
}

func opCurveto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pointX, pointY, err := popXYZ(interp, "curveto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	if err := state.requirePoint("curveto"); err != nil {
		return err
	}
	return state.addUserCurve(pointX, pointY, 0, 0, false)
}

func opRcurveto(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pointX, pointY, err := popXYZ(interp, "rcurveto")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	userX, userY, err := state.userPoint("rcurveto")
	if err != nil {
		return err
	}
	return state.addUserCurve(pointX, pointY, userX, userY, true)
}

func opClosepath(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	if err := state.requirePoint("closepath"); err != nil {
		return err
	}
	if !state.subOpen {
		return errOf(errNoCurrentPt, "closepath")
	}
	return state.addPoints([]devPt{{x: state.subX, y: state.subY, move: false}})
}

func opNewpath(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gsFor(interp).clearPath()
	return nil
}

func opCurrentPoint(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	userX, userY, err := state.userPoint("currentpoint")
	if err != nil {
		return err
	}
	if err := interp.Push(RealObj(userX)); err != nil {
		return err
	}
	return interp.Push(RealObj(userY))
}

func opStroke(ctx context.Context, interp *Interp) error {
	return paintOp(ctx, interp, paintStroke, false)
}

func opFill(ctx context.Context, interp *Interp) error {
	return paintOp(ctx, interp, paintFill, false)
}

func opEOFill(ctx context.Context, interp *Interp) error {
	return paintOp(ctx, interp, paintEOFill, true)
}

func opSetLineWidth(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	width, err := popFloat(interp, "setlinewidth")
	if err != nil {
		return err
	}
	gsFor(interp).width = width
	return nil
}

// opSetLineCapRun stores the line cap. The stroke device draws a capsule,
// which is the round cap, so caps 0 and 2 keep the recorded deviation from
// documentation/devices.md.
func opSetLineCapRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, err := interp.PopInt()
	if err != nil {
		return err
	}
	if value < 0 || value > maxLineCap {
		return errOf(errRangecheck, opSetLineCap)
	}
	gsFor(interp).lineCap = int(value)
	return nil
}

// opRectStrokeRun strokes the rectangle x y width height and leaves the
// current path in place, per the Level 2 contract. A negative width or height
// extends the rectangle in the negative direction.
func opRectStrokeRun(ctx context.Context, interp *Interp) error {
	return rectPaint(ctx, interp, opRectStroke, paintStroke)
}

// opRectFillRun fills the rectangle x y width height and leaves the current
// path in place.
func opRectFillRun(ctx context.Context, interp *Interp) error {
	return rectPaint(ctx, interp, opRectFill, paintFill)
}

func rectPaint(ctx context.Context, interp *Interp, opName, kind string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	height, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	width, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	userY, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	userX, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	state := gsFor(interp)
	saved := state.takePath()
	if err := state.addRect(userX, userY, width, height); err != nil {
		return err
	}
	state.notePaint(paintMark{kind: kind, evenOdd: false})
	state.putPath(saved)
	return nil
}

// opDTransformRun transforms a distance vector dx dy by the CTM into dx' dy'.
func opDTransformRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deltaY, err := popFloat(interp, opDTransform)
	if err != nil {
		return err
	}
	deltaX, err := popFloat(interp, opDTransform)
	if err != nil {
		return err
	}
	mat := gsFor(interp).ctm
	devX := mat.a*deltaX + mat.c*deltaY
	devY := mat.b*deltaX + mat.d*deltaY
	if err := interp.Push(RealObj(devX)); err != nil {
		return err
	}
	return interp.Push(RealObj(devY))
}

func opSetGray(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gray, err := popFloat(interp, "setgray")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.gray = gray
	state.red = gray
	state.green = gray
	state.blue = gray
	return nil
}

func opSetRGBColor(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	blue, err := popFloat(interp, opSetRGB)
	if err != nil {
		return err
	}
	green, err := popFloat(interp, opSetRGB)
	if err != nil {
		return err
	}
	red, err := popFloat(interp, opSetRGB)
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.red = red
	state.green = green
	state.blue = blue
	return nil
}

func opGSave(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	if len(state.saves) >= maxGSaveDepth {
		return errOf(errLimitCheck, "gsave")
	}
	state.saves = append(state.saves, state.snapshot())
	return nil
}

func opGRestore(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	if len(state.saves) == 0 {
		return errOf(errLimitCheck, "grestore")
	}
	last := len(state.saves) - 1
	snap := state.saves[last]
	state.saves = state.saves[:last]
	state.restore(snap)
	return nil
}

func opShowPage(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	if state.pix != nil {
		state.pix.ShowPage()
	}
	state.clearPath()
	state.pages++
	return nil
}

func opTranslate(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, ty, err := popXY(interp, "translate")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.ctm = concatMatrix(translation(tx, ty), state.ctm)
	return nil
}

func opScale(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sx, sy, err := popXY(interp, "scale")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.ctm = concatMatrix(scaling(sx, sy), state.ctm)
	return nil
}

func opRotate(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	degrees, err := popFloat(interp, "rotate")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.ctm = concatMatrix(rotation(degrees), state.ctm)
	return nil
}

func opConcat(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mat, err := popMatrix(interp, "concat")
	if err != nil {
		return err
	}
	state := gsFor(interp)
	state.ctm = concatMatrix(mat, state.ctm)
	return nil
}

func opSetMatrix(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mat, err := popMatrix(interp, "setmatrix")
	if err != nil {
		return err
	}
	gsFor(interp).ctm = mat
	return nil
}

func opCurrentMatrix(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mat := gsFor(interp).ctm
	values := []float64{mat.a, mat.b, mat.c, mat.d, mat.e, mat.f}
	for _, val := range values {
		if err := interp.Push(RealObj(val)); err != nil {
			return err
		}
	}
	return nil
}

func paintOp(ctx context.Context, interp *Interp, kind string, evenOdd bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gsFor(interp).notePaint(paintMark{kind: kind, evenOdd: evenOdd})
	return nil
}

func gsFor(interp *Interp) *gstate {
	gsMu.Lock()
	defer gsMu.Unlock()
	state := gsByInterp[interp]
	if state == nil {
		state = newGState()
		gsByInterp[interp] = state
	}
	return state
}

func newGState() *gstate {
	return &gstate{
		ctm:         identityMatrix(),
		path:        nil,
		hasPt:       false,
		devX:        0,
		devY:        0,
		subX:        0,
		subY:        0,
		subOpen:     false,
		width:       defaultLineWidth,
		lineCap:     0,
		gray:        0,
		red:         0,
		green:       0,
		blue:        0,
		fontName:    "",
		fontSize:    baseFontSize,
		clips:       nil,
		saves:       nil,
		pages:       0,
		strokeCount: 0,
		fillCount:   0,
		eoFillCount: 0,
		marks:       nil,
		pix:         nil,
		scale:       1,
		pageW:       defaultPageWidth,
		pageH:       defaultPageHeight,
	}
}

func (state *gstate) moveTo(devX, devY float64) error {
	if err := state.addPoints([]devPt{{x: devX, y: devY, move: true}}); err != nil {
		return err
	}
	state.subX = state.devX
	state.subY = state.devY
	state.subOpen = true
	return nil
}

func (state *gstate) addUserCurve(xs, ys [3]float64, originX, originY float64, relative bool) error {
	pts := make([]devPt, len(xs))
	for i := range xs {
		userX := xs[i]
		userY := ys[i]
		if relative {
			userX += originX
			userY += originY
		}
		pts[i].x, pts[i].y = state.ctm.apply(userX, userY)
	}
	return state.addPoints(pts)
}

func (state *gstate) addPoints(pts []devPt) error {
	if len(pts) > maxPathPoints || len(state.path) > maxPathPoints-len(pts) {
		return errOf(errLimitCheck, pathLimitOp)
	}
	state.path = append(state.path, pts...)
	if len(state.path) == 0 {
		return nil
	}
	last := state.path[len(state.path)-1]
	state.devX = last.x
	state.devY = last.y
	state.hasPt = true
	return nil
}

// pathSnap is the current path and current point, saved around a rectangle
// paint so rectfill and rectstroke leave the path untouched.
type pathSnap struct {
	path    []devPt
	hasPt   bool
	devX    float64
	devY    float64
	subX    float64
	subY    float64
	subOpen bool
}

func (state *gstate) takePath() pathSnap {
	return pathSnap{
		path:    append([]devPt(nil), state.path...),
		hasPt:   state.hasPt,
		devX:    state.devX,
		devY:    state.devY,
		subX:    state.subX,
		subY:    state.subY,
		subOpen: state.subOpen,
	}
}

func (state *gstate) putPath(snap pathSnap) {
	state.path = snap.path
	state.hasPt = snap.hasPt
	state.devX = snap.devX
	state.devY = snap.devY
	state.subX = snap.subX
	state.subY = snap.subY
	state.subOpen = snap.subOpen
}

// addRect appends a closed rectangle subpath in user space. A negative width
// or height extends the rectangle in the negative direction.
func (state *gstate) addRect(userX, userY, width, height float64) error {
	corners := [...][2]float64{
		{userX, userY},
		{userX + width, userY},
		{userX + width, userY + height},
		{userX, userY + height},
		{userX, userY},
	}
	pts := make([]devPt, len(corners))
	for i, corner := range corners {
		pts[i].x, pts[i].y = state.ctm.apply(corner[0], corner[1])
	}
	pts[0].move = true
	if err := state.addPoints(pts); err != nil {
		return err
	}
	state.subX = pts[0].x
	state.subY = pts[0].y
	state.subOpen = true
	return nil
}

func (state *gstate) requirePoint(opName string) error {
	if state.hasPt {
		return nil
	}
	return errOf(errNoCurrentPt, opName)
}

func (state *gstate) userPoint(opName string) (float64, float64, error) {
	if err := state.requirePoint(opName); err != nil {
		return 0, 0, err
	}
	inv, ok := state.ctm.invert()
	if !ok {
		return 0, 0, errOf(errUndefinedRes, opName)
	}
	userX, userY := inv.apply(state.devX, state.devY)
	return userX, userY, nil
}

func (state *gstate) clearPath() {
	state.path = nil
	state.hasPt = false
	state.subOpen = false
	state.devX = 0
	state.devY = 0
	state.subX = 0
	state.subY = 0
}

func (state *gstate) notePaint(mark paintMark) {
	if state.pix != nil {
		state.emit(mark)
	}
	state.marks = append(state.marks, mark)
	last := state.marks[len(state.marks)-1]
	switch {
	case last.evenOdd:
		state.eoFillCount++
	case last.kind == paintStroke:
		state.strokeCount++
	default:
		state.fillCount++
	}
	state.clearPath()
}

func (state *gstate) emit(mark paintMark) {
	pts := make([]graphics.Point, len(state.path))
	for i, pt := range state.path {
		pts[i] = graphics.Point{X: pt.x * state.scale, Y: pt.y * state.scale, Move: pt.move}
	}
	width := state.width * state.scale * math.Hypot(state.ctm.a, state.ctm.b)
	clips := state.deviceClips()
	if mark.kind == paintStroke {
		if len(clips) == 0 {
			state.pix.Stroke(pts, width, state.red, state.green, state.blue)
			return
		}
		state.pix.StrokeClipped(clips, pts, width, state.red, state.green, state.blue)
		return
	}
	if len(clips) == 0 {
		state.pix.Fill(pts, state.red, state.green, state.blue, mark.evenOdd)
		return
	}
	state.pix.FillClipped(clips, pts, state.red, state.green, state.blue, mark.evenOdd)
}

func (state *gstate) snapshot() *gstateSnap {
	return &gstateSnap{
		ctm:      state.ctm,
		path:     append([]devPt(nil), state.path...),
		hasPt:    state.hasPt,
		devX:     state.devX,
		devY:     state.devY,
		subX:     state.subX,
		subY:     state.subY,
		subOpen:  state.subOpen,
		width:    state.width,
		lineCap:  state.lineCap,
		gray:     state.gray,
		red:      state.red,
		green:    state.green,
		blue:     state.blue,
		fontName: state.fontName,
		fontSize: state.fontSize,
		clips:    cloneClips(state.clips),
	}
}

func (state *gstate) restore(snap *gstateSnap) {
	state.ctm = snap.ctm
	state.path = append([]devPt(nil), snap.path...)
	state.hasPt = snap.hasPt
	state.devX = snap.devX
	state.devY = snap.devY
	state.subX = snap.subX
	state.subY = snap.subY
	state.subOpen = snap.subOpen
	state.width = snap.width
	state.lineCap = snap.lineCap
	state.gray = snap.gray
	state.red = snap.red
	state.green = snap.green
	state.blue = snap.blue
	state.fontName = snap.fontName
	state.fontSize = snap.fontSize
	state.clips = cloneClips(snap.clips)
}

func (mat matrix) apply(userX, userY float64) (float64, float64) {
	devX := mat.a*userX + mat.c*userY + mat.e
	devY := mat.b*userX + mat.d*userY + mat.f
	return devX, devY
}

func (mat matrix) invert() (matrix, bool) {
	det := mat.a*mat.d - mat.b*mat.c
	if det == 0 {
		var zero matrix
		return zero, false
	}
	return matrix{
		a: mat.d / det,
		b: -mat.b / det,
		c: -mat.c / det,
		d: mat.a / det,
		e: (mat.c*mat.f - mat.d*mat.e) / det,
		f: (mat.b*mat.e - mat.a*mat.f) / det,
	}, true
}

// concatMatrix applies left to the user point before right.
func concatMatrix(left, right matrix) matrix {
	return matrix{
		a: left.a*right.a + left.b*right.c,
		b: left.a*right.b + left.b*right.d,
		c: left.c*right.a + left.d*right.c,
		d: left.c*right.b + left.d*right.d,
		e: left.e*right.a + left.f*right.c + right.e,
		f: left.e*right.b + left.f*right.d + right.f,
	}
}

func identityMatrix() matrix {
	return matrix{a: 1, b: 0, c: 0, d: 1, e: 0, f: 0}
}

func translation(tx, ty float64) matrix {
	return matrix{a: 1, b: 0, c: 0, d: 1, e: tx, f: ty}
}

func scaling(sx, sy float64) matrix {
	return matrix{a: sx, b: 0, c: 0, d: sy, e: 0, f: 0}
}

func rotation(degrees float64) matrix {
	rad := degrees * math.Pi / halfTurnDegrees
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	return matrix{a: cos, b: sin, c: -sin, d: cos, e: 0, f: 0}
}

func zeroMatrix() matrix {
	var mat matrix
	return mat
}

func popFloat(interp *Interp, opName string) (float64, error) {
	val, _, err := popNumber(interp, opName)
	return val, err
}

func popXY(interp *Interp, opName string) (float64, float64, error) {
	userY, err := popFloat(interp, opName)
	if err != nil {
		return 0, 0, err
	}
	userX, err := popFloat(interp, opName)
	if err != nil {
		return 0, 0, err
	}
	return userX, userY, nil
}

func popXYZ(interp *Interp, opName string) ([3]float64, [3]float64, error) {
	var pointX, pointY [3]float64
	for i := len(pointX) - 1; i >= 0; i-- {
		userY, err := popFloat(interp, opName)
		if err != nil {
			return pointX, pointY, err
		}
		userX, err := popFloat(interp, opName)
		if err != nil {
			return pointX, pointY, err
		}
		pointX[i] = userX
		pointY[i] = userY
	}
	return pointX, pointY, nil
}

func popMatrix(interp *Interp, opName string) (matrix, error) {
	elemF, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	elemE, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	elemD, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	elemC, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	elemB, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	elemA, err := popFloat(interp, opName)
	if err != nil {
		return zeroMatrix(), err
	}
	return matrix{a: elemA, b: elemB, c: elemC, d: elemD, e: elemE, f: elemF}, nil
}
