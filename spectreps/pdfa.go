package spectreps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

const (
	// pdfaIntentExtra is the index of the output intent in the appended
	// objects, after the metadata stream and the ICC profile stream.
	pdfaIntentExtra = 2
	// pdfaCatalogError is the error when the source catalog cannot be read.
	pdfaCatalogError = "syntaxerror"
)

// rewritePDFA appends the PDF/A-4 metadata and output intent, or refuses the
// claim with a JobError whose Msg is the failed preflight rule.
func rewritePDFA(ctx context.Context, file *pdf.File, opt RewriteOptions) ([]byte, error) {
	mode := pdfaMode(opt.PDFA)
	if err := pdfa.Preflight(ctx, file, mode); err != nil {
		return nil, asPDFJobError(err)
	}
	overrides, err := pdfaOverrides(ctx, file, opt.Level)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	first := file.ObjectCount() + 1
	extras := pdfa.ExtraObjects(mode, first)
	subsetOverrides, subsetAppended, err := subsetObjects(ctx, file, opt.SubsetFonts, first+len(extras))
	if err != nil {
		return nil, err
	}
	mergeOverrides(overrides, subsetOverrides)
	catalog, err := pdfaCatalog(file, first, first+pdfaIntentExtra)
	if err != nil {
		return nil, err
	}
	copyOpt := pdfout.CopyOptions{
		Overrides:       overrides,
		PackObjects:     false,
		AppendObjects:   append(extras, subsetAppended...),
		CatalogOverride: catalog,
		PDFA:            true,
	}
	out, err := pdfout.WriteCopy(ctx, file, copyOpt)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

// pdfaOverrides maps a level above 0 to its stream and image overrides.
// Level 0 copies streams unchanged, so it returns an empty map.
func pdfaOverrides(ctx context.Context, file *pdf.File, level int) (map[int][]byte, error) {
	if level < pdfout.MinCompressionLevel {
		return map[int][]byte{}, nil
	}
	return pdfout.LevelOverrides(ctx, file, level)
}

// pdfaCatalog reads the source catalog for the override that carries
// /Metadata and /OutputIntents.
func pdfaCatalog(file *pdf.File, metadataNum, intentNum int) ([]byte, error) {
	root := file.RootNum()
	if root <= 0 {
		return nil, rewriteJobError(pdfaCatalogError)
	}
	catalog, ok, err := file.ObjectValue(root)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	if !ok || catalog.Kind != pdf.KindDict {
		return nil, rewriteJobError(pdfaCatalogError)
	}
	return pdfa.Catalog(catalog, metadataNum, intentNum), nil
}

// pdfaMode maps the public mode to the internal one.
func pdfaMode(mode PDFAMode) pdfa.Mode {
	switch mode {
	case PDFANone:
		return pdfa.ModeNone
	case PDFA4:
		return pdfa.Mode4
	case PDFA4F:
		return pdfa.Mode4F
	default:
		return pdfa.ModeNone
	}
}

// rewriteJobError is one RewritePDF job error.
func rewriteJobError(msg string) JobError {
	return JobError{Op: "RewritePDF", Msg: msg, Filename: "", Line: 0, Column: 0}
}
