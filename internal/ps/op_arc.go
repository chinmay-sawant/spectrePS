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
// The operands are x y r ang1 ang2.
func opArcRun(ctx context.Context, interp *Interp) error {
	return arcRun(ctx, interp, false)
}

// opArcRunN appends a clockwise arc to the current path.
// The operands are x y r ang1 ang2.
func opArcRunN(ctx context.Context, interp *Interp) error {
	return arcRun(ctx, interp, true)
}

// arcArgs are the x y r ang1 ang2 operands of arc and arcn.
type arcArgs struct {
	centerX float64
	centerY float64
	radius  float64
	angle1  float64
	angle2  float64
}

// popArcArgs pops x y r ang1 ang2, the arc operand order.
func popArcArgs(interp *Interp, opName string) (arcArgs, error) {
	var args arcArgs
	var err error
	args.angle2, err = popFloat(interp, opName)
	if err != nil {
		return args, err
	}
	args.angle1, err = popFloat(interp, opName)
	if err != nil {
		return args, err
	}
	args.radius, err = popFloat(interp, opName)
	if err != nil {
		return args, err
	}
	args.centerY, err = popFloat(interp, opName)
	if err != nil {
		return args, err
	}
	args.centerX, err = popFloat(interp, opName)
	return args, err
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
	args, err := popArcArgs(interp, opName)
	if err != nil {
		return err
	}
	if args.radius < 0 {
		return errOf(errRangecheck, opName)
	}
	state := gsFor(interp)
	pts := arcPoints(state, args, arcSweep(args.angle1, args.angle2, clockwise))
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

// arcPoints samples the arc chord endpoints in device space.
func arcPoints(state *gstate, args arcArgs, sweep float64) []devPt {
	steps := int(math.Ceil(math.Abs(sweep) / arcStepDegrees))
	pts := make([]devPt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		angle := args.angle1
		if steps > 0 {
			angle = args.angle1 + sweep*float64(i)/float64(steps)
		}
		rad := angle * math.Pi / halfTurnDegrees
		userX := args.centerX + args.radius*math.Cos(rad)
		userY := args.centerY + args.radius*math.Sin(rad)
		devX, devY := state.ctm.apply(userX, userY)
		pt := devPt{x: devX, y: devY, move: i == 0 && !state.hasPt}
		pts = append(pts, pt)
	}
	return pts
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
