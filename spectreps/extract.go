package spectreps

import "context"

// ExtractText returns the text of one zero-based page of an open PDF.
// Lines run top to bottom and left to right, each line ends with CRLF, and a
// font with neither /ToUnicode nor a named encoding falls back to the code
// point. The index follows RasterizePage, so a negative index or an index
// past the last page returns rangecheck.
func (in *Instance) ExtractText(ctx context.Context, doc *Document, pageIndex int) (string, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	_ = in
	if pageOutOfRange(doc, pageIndex) {
		return "", extractRange()
	}
	text, err := doc.file.ExtractText(ctx, pageIndex)
	if err != nil {
		return "", asPDFJobError(err)
	}
	return text, nil
}

func extractRange() JobError {
	return JobError{Op: "ExtractText", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
}
