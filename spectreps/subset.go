package spectreps

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// subsetObjects returns the font overrides and the appended bodies a
// subsetting rewrite adds, starting at firstNum. The option is off by
// default, and then both results are nil and the copy is byte-identical to a
// plain rewrite.
func subsetObjects(
	ctx context.Context,
	file *pdf.File,
	subset bool,
	firstNum int,
) (map[int][]byte, [][]byte, error) {
	if !subset {
		return nil, nil, nil
	}
	overrides, appended, err := file.SubsetFontObjects(ctx, firstNum)
	if err != nil {
		return nil, nil, asPDFJobError(err)
	}
	return overrides, appended, nil
}

// mergeOverrides writes the subset font bodies over the level overrides. A
// font dictionary is not a content or image object, so the two sets do not
// name the same number.
func mergeOverrides(overrides, extra map[int][]byte) {
	for num, body := range extra {
		overrides[num] = body
	}
}
