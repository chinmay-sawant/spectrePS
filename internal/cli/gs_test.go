package cli

import (
	"path/filepath"
	"testing"
)

// TestGSRejectsAll checks the reject paths of the gs scanner. Every command
// line here stays rejected after the switch families land.
func TestGSRejectsAll(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ps")
	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", []string{"gs"}},
		{"no device", []string{"gs", missing}},
		{"unknown debug switch", []string{"gs", "-Z", missing}},
		{"unknown parameter", []string{"gs", "-dFoo", missing}},
		{"unknown set name", []string{"gs", "-sFoo=1", missing}},
		{"inline code", []string{"gs", "-c", "1 2 add"}},
		{"unsafe", []string{"gs", "-dNOSAFER", missing}},
		{"delayed safer", []string{"gs", "-dDELAYSAFER", missing}},
		{"unknown device", []string{"gs", "-sDEVICE=ps2write", "-sOutputFile=out.pdf", missing}},
		{"stdin", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png", "-"}},
		{"argument file", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png", "@args"}},
		{"missing input", []string{"gs", "-sDEVICE=png16m", "-sOutputFile=out.png"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := callRun(t, tc.args...)
			if code != exitUsage {
				t.Fatalf("code = %d, want %d", code, exitUsage)
			}
			if stderr == "" {
				t.Fatal("stderr is empty, want a rejection message")
			}
		})
	}
}
