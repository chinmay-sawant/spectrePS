package ps

import (
	"context"
	"math"
)

const (
	opArc  = "arc"
	opArcN = "arcn"

	// arcStepDegrees is the chord spacing of the arc approximation. The path
	// model stores line segments, and five degrees is under a fifth of a pixel
	// at the radii the corpus uses.
	arcStepDegrees = 5
)

func registerArcOps(interp *Interp) {
	interp.Install(opArc, opArcRun)
	interp.Install(opArcN, opArcRunN)
}

// opArcRun appends a counterclockwise arc to the current path.
// x y r ang1 ang2 arc -
func opArcRun(ctx context.Context, interp *Interp) error {
	return arcRun(ctx, interp, false)
}

// opArcRunN appends a clockwise arc to the current path.
// x y r ang1 ang2 arcn -
func opArcRunN(ctx context.Context, interp *Interp) error {
	return arcRun(ctx, interp, true)
}

// arcRun appends the arc as chords. A current point connects to the arc start
// with a line; without one the arc starts its own subpath.
func arcRun(ctx context.Context, interp *Interp, clockwise bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	opName := opArc
	if clockwise {
		opName = opArcN
	}
	angle2, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	angle1, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	radius, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	centerY, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	centerX, err := popFloat(interp, opName)
	if err != nil {
		return err
	}
	if radius < 0 {
		return errOf(errRangecheck, opName)
	}
	state := gsFor(interp)
	sweep := arcSweep(angle1, angle2, clockwise)
	steps := int(math.Ceil(math.Abs(sweep) / arcStepDegrees))
	pts := make([]devPt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		angle := angle1
		if steps > 0 {
			angle = angle1 + sweep*float64(i)/float64(steps)
		}
		rad := angle * math.Pi / halfTurnDegrees
		userX := centerX + radius*math.Cos(rad)
		userY := centerY + radius*math.Sin(rad)
		devX, devY := state.ctm.apply(userX, userY)
		pt := devPt{x: devX, y: devY}
		if i == 0 && !state.hasPt {
			pt.move = true
		}
		pts = append(pts, pt)
	}
	if err := state.addPoints(pts); err != nil {
		return err
	}
	if pts[0].move {
		state.subX = pts[0].x
		state.subY = pts[0].y
		state.subOpen = true
	}
	return nil
}

// arcSweep turns the two angles into a signed sweep. arc is positive
// counterclockwise and arcn negative clockwise, and neither turns more than
// one full circle.
func arcSweep(angle1, angle2 float64, clockwise bool) float64 {
	sweep := angle2 - angle1
	if clockwise {
		if sweep > 0 {
			sweep = -fullTurnDegrees + math.Mod(sweep, fullTurnDegrees)
		}
		if sweep < -fullTurnDegrees {
			sweep = -fullTurnDegrees
		}
		return sweep
	}
	if sweep < 0 {
		sweep = fullTurnDegrees + math.Mod(sweep, fullTurnDegrees)
	}
	if sweep > fullTurnDegrees {
		sweep = fullTurnDegrees
	}
	return sweep
}
