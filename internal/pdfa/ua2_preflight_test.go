package pdfa

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

func TestUA2Preflight(t *testing.T) {
	t.Run("conforming", checkUA2Conforming)
	t.Run("rules", checkUA2Rules)
	t.Run("requests", checkUA2Requests)
}

func checkUA2Conforming(t *testing.T) {
	t.Helper()
	file := ua2File(t, defaultUA2Fixture())
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
}

// checkUA2Rules proves one failing fixture per rule.
func checkUA2Rules(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		fix  ua2Fixture
		rule string
	}{
		{name: "marked", fix: ua2With(func(fix *ua2Fixture) { fix.marked = false }), rule: ruleUA2Marked},
		{name: "struct tree", fix: ua2With(func(fix *ua2Fixture) { fix.tree = false }), rule: ruleUA2StructTree},
		{name: "document", fix: ua2With(func(fix *ua2Fixture) { fix.documentType = "P" }), rule: ruleUA2Document},
		{name: "language", fix: ua2With(func(fix *ua2Fixture) { fix.lang = "english_us" }), rule: ruleUA2Lang},
		{
			name: "display doc title",
			fix:  ua2With(func(fix *ua2Fixture) { fix.displayDocTitle = false }),
			rule: ruleUA2DisplayDocTitle,
		},
		{
			name: "pdfuaid",
			fix:  ua2With(func(fix *ua2Fixture) { fix.part = "1"; fix.rev = "2014" }),
			rule: ruleUA2PDFUAID,
		},
		{name: "title", fix: ua2With(func(fix *ua2Fixture) { fix.title = "" }), rule: ruleUA2Title},
		{name: "role map", fix: ua2With(func(fix *ua2Fixture) { fix.documentType = "Custom" }), rule: ruleUA2RoleMap},
		{name: "mcid", fix: ua2With(func(fix *ua2Fixture) { fix.parentNums = "" }), rule: ruleUA2MCID},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			file := ua2File(t, testCase.fix)
			wantUA2Rule(t, PreflightUA2(t.Context(), file), testCase.rule)
		})
	}
}

// checkUA2Requests proves the two preflights do not fight: a PDF/A-only
// problem passes the UA-2 request, and the UA-2 request never reports a
// PDF/A rule.
func checkUA2Requests(t *testing.T) {
	t.Helper()
	lzw := "<< /Filter /LZWDecode /Length 0 >>\nstream\n\nendstream"
	file := ua2File(t, ua2With(func(fix *ua2Fixture) { fix.extra = []string{lzw} }))
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleLZWDecode)

	font := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	file = ua2File(t, ua2With(func(fix *ua2Fixture) { fix.extra = []string{font} }))
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
	wantRule(t, Preflight(t.Context(), file, Mode4), ruleFontNotEmbedded)
}

func TestUA2PreflightNilContext(t *testing.T) {
	file := ua2File(t, defaultUA2Fixture())
	defer func() {
		if recovered := recover(); recovered != pdfaNilContextPanic {
			t.Fatalf("panic %v", recovered)
		}
	}()
	_ = PreflightUA2(nil, file) //nolint:staticcheck // nil context is the case under test
}

// ua2With returns the default fixture with one mutation.
func ua2With(mutate func(*ua2Fixture)) ua2Fixture {
	fix := defaultUA2Fixture()
	mutate(&fix)
	return fix
}

func wantUA2Rule(t *testing.T, err error, rule string) {
	t.Helper()
	var job *pdf.Error
	if !errors.As(err, &job) {
		t.Fatalf("error %v, want *pdf.Error", err)
	}
	if job.Op != opPDFUA || job.Name != rule {
		t.Fatalf("error %s in %s, want %s in %s", job.Name, job.Op, rule, opPDFUA)
	}
}

func wantNoUA2Rule(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("error %v, want nil", err)
	}
}
