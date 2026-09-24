package graphics

import "testing"

func TestMatrix(t *testing.T) {
	checkIdentity(t)
	checkTranslate(t)
	checkScale(t)
	checkRotate(t)
	checkConcat(t)
	checkGSave(t)
	checkGSaveCap(t)
}

func checkIdentity(t *testing.T) {
	t.Helper()
	mat := Identity()
	xPos, yPos := mat.Apply(3, 4)
	if xPos != 3 || yPos != 4 {
		t.Fatalf("identity (%v, %v)", xPos, yPos)
	}
}

func checkTranslate(t *testing.T) {
	t.Helper()
	xPos, yPos := Identity().Translate(10, -2).Apply(1, 2)
	if xPos != 11 || yPos != 0 {
		t.Fatalf("translate (%v, %v)", xPos, yPos)
	}
}

func checkScale(t *testing.T) {
	t.Helper()
	xPos, yPos := Identity().Scale(2, 3).Apply(4, 5)
	if xPos != 8 || yPos != 15 {
		t.Fatalf("scale (%v, %v)", xPos, yPos)
	}
}

func checkRotate(t *testing.T) {
	t.Helper()
	xPos, yPos := Identity().Rotate(90).Apply(1, 0)
	if mathAbs(xPos) > 1e-9 || mathAbs(yPos-1) > 1e-9 {
		t.Fatalf("rotate (%v, %v)", xPos, yPos)
	}
}

func checkConcat(t *testing.T) {
	t.Helper()
	xPos, yPos := Identity().Translate(1, 0).Scale(2, 2).Apply(3, 4)
	if xPos != 7 || yPos != 8 {
		t.Fatalf("concat (%v, %v)", xPos, yPos)
	}
}

func checkGSave(t *testing.T) {
	t.Helper()
	state := NewState()
	if state.Width != defaultLineWidth || state.Red != 0 || state.CTM != Identity() {
		t.Fatalf("defaults %+v", state)
	}
	if err := state.GSave(); err != nil {
		t.Fatal(err)
	}
	state.SetLineWidth(4)
	state.SetGray(0.25)
	state.SetRGB(0.1, 0.2, 0.3)
	state.CTM = state.CTM.Translate(5, 6)
	if err := state.GRestore(); err != nil {
		t.Fatal(err)
	}
	if state.Width != defaultLineWidth || state.Red != 0 || state.CTM != Identity() {
		t.Fatalf("grestore %+v", state)
	}
	checkEmptyGRestore(t, state)
}

func checkEmptyGRestore(t *testing.T, state *State) {
	t.Helper()
	if err := state.GRestore(); err == nil || err.Error() != errLimit.Error() {
		t.Fatalf("empty grestore %v", err)
	}
}

func checkGSaveCap(t *testing.T) {
	t.Helper()
	state := NewState()
	for range maxGSaveDepth {
		if err := state.GSave(); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.GSave(); err == nil || err.Error() != errLimit.Error() {
		t.Fatalf("gsave cap %v", err)
	}
}

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
