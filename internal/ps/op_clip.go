package ps

import (
	"context"
	"math"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

const (
	opClip     = "clip"
	opInitClip = "initclip"
	opClipPath = "clippath"
	opPathBBox = "pathbbox"

	// The device page defaults match the letter page in
	// documentation/language.md when the interpreter has no pixmap.
	defaultPageWidth  = 612
	defaultPageHeight = 792
)

// clipPath is one clip in the interpreter's device space. A paint mark
// survives when its pixel center falls inside every stored clip. evenOdd
// selects the even-odd rule; clip stores the nonzero rule.
type clipPath struct {
	pts     []devPt
	evenOdd bool
}

func registerClipOps(interp *Interp) {
	interp.Install(opClip, opClipRun)
	interp.Install(opInitClip, opInitClipRun)
	interp.Install(opClipPath, opClipPathRun)
	interp.Install(opPathBBox, opPathBBoxRun)
}

// opClipRun intersects the current clip with the current path. The path is
// left in place, as the Level 2 contract says.
func opClipRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	state.clips = append(state.clips, clipPath{
		pts:     append([]devPt(nil), state.path...),
		evenOdd: false,
	})
	return nil
}

// opInitClipRun resets the clip to the device page.
func opInitClipRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gsFor(interp).clips = nil
	return nil
}

// opClipPathRun replaces the current path with the clip boundary. With no
// clip the boundary is the device page rectangle. Stored clips are
// concatenated, which is exact for one clip and a union boundary for more
// than one.
func opClipPathRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	state.clearPath()
	pts := state.clipBoundary()
	if len(pts) == 0 {
		return nil
	}
	if err := state.addPoints(pts); err != nil {
		return err
	}
	state.subX = pts[0].x
	state.subY = pts[0].y
	state.subOpen = true
	return nil
}

// opPathBBoxRun pushes the bounding box of the current path in user space as
// llx lly urx ury. An empty path pushes 0 0 0 0.
func opPathBBoxRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := gsFor(interp)
	inv, ok := state.ctm.invert()
	if len(state.path) == 0 || !ok {
		return state.pushBBox(interp, 0, 0, 0, 0)
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, pt := range state.path {
		userX, userY := inv.apply(pt.x, pt.y)
		minX = math.Min(minX, userX)
		minY = math.Min(minY, userY)
		maxX = math.Max(maxX, userX)
		maxY = math.Max(maxY, userY)
	}
	return state.pushBBox(interp, minX, minY, maxX, maxY)
}

// pushBBox pushes one path bounding box, bottom left first.
func (state *gstate) pushBBox(interp *Interp, minX, minY, maxX, maxY float64) error {
	values := [...]float64{minX, minY, maxX, maxY}
	for _, value := range values {
		if err := interp.Push(RealObj(value)); err != nil {
			return err
		}
	}
	return nil
}

// clipBoundary returns the current clip boundary in the interpreter's device
// space. The page rectangle is the boundary when no clip is stored.
func (state *gstate) clipBoundary() []devPt {
	if len(state.clips) == 0 {
		width, height := state.pageRect()
		return []devPt{
			{x: 0, y: 0, move: true},
			{x: width, y: 0, move: false},
			{x: width, y: height, move: false},
			{x: 0, y: height, move: false},
			{x: 0, y: 0, move: false},
		}
	}
	var pts []devPt
	for _, clip := range state.clips {
		pts = append(pts, clip.pts...)
	}
	return pts
}

// pageRect returns the device page in the interpreter's device space. The
// pixmap page is in pixels and the device space is one scale unit per pixel.
func (state *gstate) pageRect() (float64, float64) {
	width, height := state.pageW, state.pageH
	if width <= 0 || height <= 0 {
		width, height = defaultPageWidth, defaultPageHeight
	}
	if state.scale > 0 {
		width /= state.scale
		height /= state.scale
	}
	return width, height
}

// deviceClips scales the stored clips into pixmap pixels for the ClipMarker
// seam. No stored clip returns nil so the plain device path stays unchanged.
func (state *gstate) deviceClips() []graphics.Clip {
	if len(state.clips) == 0 {
		return nil
	}
	clips := make([]graphics.Clip, len(state.clips))
	for i, clip := range state.clips {
		pts := make([]graphics.Point, len(clip.pts))
		for j, pt := range clip.pts {
			pts[j] = graphics.Point{X: pt.x * state.scale, Y: pt.y * state.scale, Move: pt.move}
		}
		clips[i] = graphics.Clip{Pts: pts, EvenOdd: clip.evenOdd}
	}
	return clips
}

// cloneClips deep copies the clip list for gsave and grestore.
func cloneClips(clips []clipPath) []clipPath {
	if len(clips) == 0 {
		return nil
	}
	out := make([]clipPath, len(clips))
	for i, clip := range clips {
		out[i] = clipPath{
			pts:     append([]devPt(nil), clip.pts...),
			evenOdd: clip.evenOdd,
		}
	}
	return out
}
