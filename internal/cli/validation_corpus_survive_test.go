package cli

// The survive-classification tests prove the robustness check is not vacuous.
// A corpus check that accepts everything measures nothing, so these lock the
// boundary between a refusal, which passes, and a fault, which does not.

import (
	"errors"
	"strings"
	"testing"
)

// surviveCase is one classification to lock: the process outcome and the stderr
// it produced, with the error the classifier must return. A nil wantErr means
// the classifier has to accept the run.
type surviveCase struct {
	name    string
	code    int
	stderr  string
	wantErr error
}

// surviveCases returns every classification the survive check has to get right.
// The pass half is a defect the reader handled, and the failure half is every
// way a Go process can die plus a non-zero exit that carries no spectreps
// error. Without the second list the corpus check would pass everything.
func surviveCases() []surviveCase {
	return []surviveCase{
		{name: "recovered page", code: exitOK},
		{name: "named refusal", code: 1, stderr: "Error: /syntaxerror in obj\n"},
		{name: "named refusal with the operator", code: 1,
			stderr: "Error: /invalidaccess in Encrypt\n"},
		{name: "killed by a signal", code: -11, wantErr: errSurviveSignal},
		{name: "go panic", code: 2, wantErr: errSurviveFault,
			stderr: "panic: runtime error: index out of range [3]\n\ngoroutine 1 [running]:\n"},
		{name: "runtime fault without a panic banner", code: 2, wantErr: errSurviveFault,
			stderr: "runtime error: slice bounds out of range\n"},
		{name: "fatal error", code: 2, wantErr: errSurviveFault,
			stderr: "fatal error: all goroutines are asleep - deadlock!\n"},
		{name: "stack overflow", code: 2, wantErr: errSurviveFault,
			stderr: "runtime: goroutine stack exceeds 1000000000-byte limit\nstack overflow\n"},
		{name: "killed by the kernel, which is not a named error", code: 137,
			wantErr: errSurviveFault, stderr: "unexpected signal: killed\n"},
		{name: "a non-zero exit with no spectreps error", code: 3,
			wantErr: errSurviveSilent, stderr: "something else went wrong\n"},
	}
}

// TestValidationCorpusSurviveVerdict requires the classification to accept a
// clean exit and a named refusal, and to reject every fault shape. A defect the
// reader handled is a pass; a Go panic is not.
func TestValidationCorpusSurviveVerdict(t *testing.T) {
	for _, testCase := range surviveCases() {
		t.Run(testCase.name, func(t *testing.T) {
			err := surviveVerdict(testCase.code, testCase.stderr)
			if testCase.wantErr == nil {
				if err != nil {
					t.Fatalf("surviveVerdict(%d, %q) = %v, want nil",
						testCase.code, testCase.stderr, err)
				}
				return
			}
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("surviveVerdict(%d, %q) = %v, want %v",
					testCase.code, testCase.stderr, err, testCase.wantErr)
			}
		})
	}
}

// TestValidationCorpusFaultSignalsKeepTheirShape requires every fault signal to
// be a phrase that cannot appear in a named refusal. A named refusal is
// "Error: /name in op", so a signal that overlapped that shape would reject a
// legitimate row.
func TestValidationCorpusFaultSignalsKeepTheirShape(t *testing.T) {
	signals := corpusFaultSignals()
	if len(signals) == 0 {
		t.Fatal("no fault signals, so a panic would pass as a refusal")
	}
	const refusal = "Error: /limitcheck in exec\n"
	for _, signal := range signals {
		if signal == "" {
			t.Error("an empty fault signal matches everything")
		}
		if strings.Contains(refusal, signal) {
			t.Errorf("fault signal %q appears in the named refusal %q", signal, refusal)
		}
	}
}
