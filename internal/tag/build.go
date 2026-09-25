package tag

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
)

// documentType is the top structure type PDF/UA-2 requires.
const documentType = "Document"

// panicNilContext is the panic text of a nil context.
const panicNilContext = "tag: nil context"

// Options carries the document metadata one tagged write needs.
type Options struct {
	// Title is the dc:title. An empty title falls back to the source XMP
	// title, and a build with no title at all fails ua2-title.
	Title string
	// Lang is the catalog /Lang. An empty language falls back to the source
	// catalog language.
	Lang string
	// Claim writes pdfuaid:part 2 and pdfuaid:rev 2024 when the built bytes
	// pass PreflightUA2. A build that fails the preflight returns the tagged
	// bytes without the claim and the preflight error.
	Claim bool
}

// Build writes a tagged PDF 2.0 file for the recorded pages and the plan. The
// builder always runs pdf.Open plus pdfa.PreflightUA2 on its own bytes, open
// decision 6, and only a passing build carries the pdfuaid claim when the
// caller opted in. The result is generation and preflight, never
// certification.
//
// The source must be an untagged PDF. A well-formed tagged input is a named
// refusal owned by the public layer, open decision 2; a partial tree,
// /Marked true with no root or a tree that fails preflight, is rebuilt from
// the content. ActualText stays narrow: only a caller-supplied value is
// written, never a guessed one, open decision 9.
//
// A canceled context returns ctx.Err(). A nil context panics with
// "tag: nil context".
func Build(
	ctx context.Context,
	file *pdf.File,
	pages []*Recorder,
	plan *Plan,
	opt Options,
) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := checkRecorders(pages); err != nil {
		return nil, err
	}
	info, err := buildInfo(file, opt)
	if err != nil {
		return nil, err
	}
	built, err := newBuilder(file, pages, plan)
	if err != nil {
		return nil, err
	}
	out, err := built.write(ctx, pdfa.UA2Write(info, false, false))
	if err != nil {
		return nil, err
	}
	if !opt.Claim {
		_ = preflight(ctx, out)
		return out, nil
	}
	if err := preflight(ctx, out); err != nil {
		return out, err
	}
	return built.write(ctx, pdfa.UA2Write(info, true, true))
}

// checkRecorders rejects a missing recorder and a recorder refusal.
func checkRecorders(pages []*Recorder) error {
	for _, rec := range pages {
		if rec == nil {
			return pdf.NewError(opTag, errPlan)
		}
		if err := rec.Err(); err != nil {
			return err
		}
	}
	return nil
}

// buildInfo resolves the title and the language from the caller first, then
// from the source XMP. The pdfuaid claim is never taken from the source: a
// generated file gets its claim only from a passing preflight.
func buildInfo(file *pdf.File, opt Options) (pdfa.UA2Info, error) {
	source, err := pdfa.ReadUA2(file)
	if err != nil {
		return pdfa.UA2Info{
			Metadata:        false,
			Part:            "",
			Rev:             "",
			Title:           "",
			Lang:            "",
			Marked:          false,
			Suspects:        false,
			DisplayDocTitle: false,
		}, err
	}
	title := opt.Title
	if title == "" {
		title = source.Title
	}
	lang := opt.Lang
	if lang == "" {
		lang = source.Lang
	}
	return pdfa.UA2Info{
		Metadata:        false,
		Part:            "",
		Rev:             "",
		Title:           title,
		Lang:            lang,
		Marked:          false,
		Suspects:        false,
		DisplayDocTitle: false,
	}, nil
}

// preflight opens one built file and runs the UA-2 machine checks.
func preflight(ctx context.Context, src []byte) error {
	file, err := pdf.Open(ctx, src)
	if err != nil {
		return err
	}
	return pdfa.PreflightUA2(ctx, file)
}
