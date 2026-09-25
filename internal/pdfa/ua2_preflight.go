package pdfa

import (
	"context"
	"errors"
	"strings"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The PDF/UA-2 failed rule names carried in the *pdf.Error Name field. They
// are distinct from the PDF/A rules, and a UA-2 check runs only for a UA-2
// request.
const (
	ruleUA2Marked          = "ua2-marked"
	ruleUA2StructTree      = "ua2-structtree"
	ruleUA2Document        = "ua2-document"
	ruleUA2Lang            = "ua2-lang"
	ruleUA2DisplayDocTitle = "ua2-displaydoctitle"
	ruleUA2PDFUAID         = "ua2-pdfuaid"
	ruleUA2Title           = "ua2-title"
	ruleUA2RoleMap         = "ua2-rolemap"
	ruleUA2MCID            = "ua2-mcid"

	opRoleMapUA2    = "RoleMap"
	opParentTreeUA2 = "ParentTree"
)

// PreflightUA2 returns a *pdf.Error with Op "PDFUA" and the failed rule when
// file cannot carry a PDF/UA-2 claim. The machine-checkable rules are:
//
//   - ua2-marked: /MarkInfo /Marked true.
//   - ua2-structtree: /StructTreeRoot parses.
//   - ua2-document: one Document in the PDF 2.0 namespace at the top.
//   - ua2-lang: /Lang has a valid syntax.
//   - ua2-displaydoctitle: /ViewerPreferences /DisplayDocTitle true.
//   - ua2-pdfuaid: a present pdfuaid claim is part 2 rev 2024.
//   - ua2-title: the XMP carries a dc:title.
//   - ua2-rolemap: every role resolves through /RoleMap or /RoleMapNS.
//   - ua2-mcid: every MCID claim has an agreeing parent tree entry.
//
// The PDF/A preflight is a separate request and never runs these checks, and
// this preflight ignores PDF/A-only problems. The result is preflight only,
// never certification.
// A canceled context returns ctx.Err(). A nil context panics with
// "pdfa: nil context".
func PreflightUA2(ctx context.Context, file *pdf.File) error {
	if ctx == nil {
		panic(pdfaNilContextPanic)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if file == nil {
		return pdf.NewError(opPDFUA, errSyntax)
	}
	info, err := ReadUA2(file)
	if err != nil {
		return err
	}
	return ua2Checks(file, info)
}

// ua2Checks runs the rules in order.
func ua2Checks(file *pdf.File, info UA2Info) error {
	if !info.Marked {
		return pdf.NewError(opPDFUA, ruleUA2Marked)
	}
	if err := ua2DocumentRule(file); err != nil {
		return err
	}
	return ua2MetadataRules(info)
}

// ua2DocumentRule parses the structure tree and checks the top element.
func ua2DocumentRule(file *pdf.File) error {
	tree, err := file.StructTree()
	if err != nil {
		return ua2TreeRule(err)
	}
	if tree == nil {
		return pdf.NewError(opPDFUA, ruleUA2StructTree)
	}
	if len(tree.Top) != 1 || tree.Top[0].Type != typeDocumentUA2 ||
		tree.Top[0].RoleNS != pdf.NamespacePDF20 {
		return pdf.NewError(opPDFUA, ruleUA2Document)
	}
	return nil
}

// ua2MetadataRules checks /Lang, /ViewerPreferences, pdfuaid, and dc:title.
func ua2MetadataRules(info UA2Info) error {
	if !langSyntaxOK(info.Lang) {
		return pdf.NewError(opPDFUA, ruleUA2Lang)
	}
	if !info.DisplayDocTitle {
		return pdf.NewError(opPDFUA, ruleUA2DisplayDocTitle)
	}
	if info.Part != "" && (info.Part != ua2PartValue || info.Rev != ua2RevValue) {
		return pdf.NewError(opPDFUA, ruleUA2PDFUAID)
	}
	if info.Title == "" {
		return pdf.NewError(opPDFUA, ruleUA2Title)
	}
	return nil
}

// ua2TreeRule maps one structure tree error to its UA-2 rule.
func ua2TreeRule(err error) error {
	var job *pdf.Error
	if errors.As(err, &job) {
		switch job.Op {
		case opRoleMapUA2:
			return pdf.NewError(opPDFUA, ruleUA2RoleMap)
		case opParentTreeUA2:
			return pdf.NewError(opPDFUA, ruleUA2MCID)
		}
	}
	return pdf.NewError(opPDFUA, ruleUA2StructTree)
}

// langSyntaxOK reports whether one language tag has the BCP 47 shape: a two
// or three letter primary tag, then one or more one-to-eight character
// alphanumeric subtags joined by hyphens. It checks syntax only, not the
// registry.
func langSyntaxOK(lang string) bool {
	if lang == "" {
		return false
	}
	parts := strings.Split(lang, "-")
	if !primaryTagOK(parts[0]) {
		return false
	}
	for _, part := range parts[1:] {
		if !subtagOK(part) {
			return false
		}
	}
	return true
}

// primaryTagOK reports whether one primary language subtag is two or three
// letters, or the private-use and grandfathered "x" and "i" tags.
func primaryTagOK(part string) bool {
	if len(part) == 2 || len(part) == 3 {
		return alphaOnly(part)
	}
	return part == "i" || part == "x"
}

// subtagOK reports whether one later subtag is one to eight alphanumerics.
func subtagOK(part string) bool {
	if part == "" || len(part) > 8 {
		return false
	}
	for index := range len(part) {
		if !alnumByte(part[index]) {
			return false
		}
	}
	return true
}

// alphaOnly reports whether text is ASCII letters only.
func alphaOnly(text string) bool {
	for index := range len(text) {
		cur := text[index]
		if (cur < 'a' || cur > 'z') && (cur < 'A' || cur > 'Z') {
			return false
		}
	}
	return true
}

// alnumByte reports whether one byte is an ASCII letter or digit.
func alnumByte(cur byte) bool {
	return (cur >= 'a' && cur <= 'z') || (cur >= 'A' && cur <= 'Z') ||
		(cur >= '0' && cur <= '9')
}
