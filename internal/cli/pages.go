package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// errPageRangeSyntax marks a malformed -pages value. The flag package turns
// it into exit 2 before any page is read.
var errPageRangeSyntax = errors.New("invalid page range, want N or A-B")

// pageSelection is one -pages flag value. The grammar is N or A-B, 1-based
// and inclusive. A- runs to the last page, -B starts at page 1. The empty
// value selects every page.
type pageSelection struct {
	text string
}

func (sel *pageSelection) String() string {
	if sel == nil {
		return ""
	}
	return sel.text
}

// Set parses the flag at flag.Parse time.
func (sel *pageSelection) Set(value string) error {
	if _, err := pageBounds(value); err != nil {
		return err
	}
	sel.text = value
	return nil
}

// pageFlag registers -pages on one flag set.
func pageFlag(set *flag.FlagSet) *pageSelection {
	sel := new(pageSelection)
	set.Var(sel, "pages", "page range, N or A-B, 1-based")
	return sel
}

// pageRange is the parsed form of one range. Both ends are optional, and an
// end without its flag text is open.
type pageRange struct {
	first    int
	last     int
	hasFirst bool
	hasLast  bool
}

// pageNone returns the range with no end set.
func pageNone() pageRange {
	return pageRange{first: 0, last: 0, hasFirst: false, hasLast: false}
}

// pageBounds parses the flag text.
func pageBounds(text string) (pageRange, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return pageNone(), nil
	}
	left, right, dash := strings.Cut(text, "-")
	if !dash {
		page, err := strconv.Atoi(left)
		if err != nil {
			return pageNone(), pageUsageError(text)
		}
		return pageRange{first: page, last: page, hasFirst: true, hasLast: true}, nil
	}
	bound := pageNone()
	if left != "" {
		page, err := strconv.Atoi(left)
		if err != nil {
			return pageNone(), pageUsageError(text)
		}
		bound.first = page
		bound.hasFirst = true
	}
	if right != "" {
		page, err := strconv.Atoi(right)
		if err != nil {
			return pageNone(), pageUsageError(text)
		}
		bound.last = page
		bound.hasLast = true
	}
	return bound, nil
}

func pageUsageError(text string) error {
	return fmt.Errorf("%w: %q", errPageRangeSyntax, text)
}

// pageIndices maps the range to zero-based page indices for a document with
// total pages. A start below 1 or past the last page returns JobError with Op
// "pages". An end past the last page clamps to the last page. The empty range
// selects every page, so a zero-page document yields no indices.
func (sel *pageSelection) pageIndices(total int) ([]int, error) {
	bound, err := pageBounds(sel.text)
	if err != nil {
		return nil, err
	}
	if !bound.hasFirst && !bound.hasLast {
		return pageAll(total), nil
	}
	start, end, err := bound.pageSpan(total)
	if err != nil {
		return nil, err
	}
	return pageSpanIndices(start, end), nil
}

// pageSpan resolves the range against total pages.
func (bound pageRange) pageSpan(total int) (int, int, error) {
	start := 1
	if bound.hasFirst {
		start = bound.first
	}
	end := total
	if bound.hasLast {
		end = bound.last
	}
	if start < 1 || start > total || end < start {
		return 0, 0, pageRangeError()
	}
	if end > total {
		end = total
	}
	return start, end, nil
}

func pageSpanIndices(start, end int) []int {
	indices := make([]int, 0, end-start+1)
	for page := start; page <= end; page++ {
		indices = append(indices, page-1)
	}
	return indices
}

func pageAll(total int) []int {
	indices := make([]int, total)
	for index := range indices {
		indices[index] = index
	}
	return indices
}

func pageRangeError() spectreps.JobError {
	return spectreps.JobError{Op: "pages", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
}

// pageSelect filters painted pages to the range.
func pageSelect(pages []spectreps.PageImage, sel pageSelection) ([]spectreps.PageImage, error) {
	indices, err := sel.pageIndices(len(pages))
	if err != nil {
		return nil, err
	}
	selected := make([]spectreps.PageImage, 0, len(indices))
	for _, index := range indices {
		selected = append(selected, pages[index])
	}
	return selected, nil
}

// pageImages paints the selected pages of one input. A .pdf input opens with
// OpenPDF and paints each selected page with RasterizePage. Any other input
// runs PostScript and filters the result. A failing PostScript page fails the
// command even when the range skips it.
func pageImages(
	in *spectreps.Instance,
	path string,
	src []byte,
	opt spectreps.RunOptions,
	sel pageSelection,
) ([]spectreps.PageImage, error) {
	ctx := context.Background()
	if !strings.HasSuffix(path, ".pdf") {
		pages, err := in.RunPostScript(ctx, src, opt)
		if err != nil {
			return nil, err
		}
		return pageSelect(pages, sel)
	}
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return nil, err
	}
	indices, err := sel.pageIndices(doc.PageCount())
	if err != nil {
		return nil, err
	}
	pages := make([]spectreps.PageImage, 0, len(indices))
	for _, index := range indices {
		img, err := in.RasterizePage(ctx, doc, index, opt)
		if err != nil {
			return nil, err
		}
		pages = append(pages, img)
	}
	return pages, nil
}

// pageRasterPair rasterizes both inputs with the same options and range, then
// compares the page lists pairwise.
func pageRasterPair(
	in *spectreps.Instance,
	leftPath string,
	leftSrc []byte,
	rightPath string,
	rightSrc []byte,
	opt spectreps.RunOptions,
	sel pageSelection,
) (spectreps.CompareResult, error) {
	left, err := pageImages(in, leftPath, leftSrc, opt, sel)
	if err != nil {
		return spectreps.CompareResult{}, err
	}
	right, err := pageImages(in, rightPath, rightSrc, opt, sel)
	if err != nil {
		return spectreps.CompareResult{}, err
	}
	return pageCompare(left, right), nil
}

// pageCompare compares page pairs. Different selected counts report Reason
// "length".
func pageCompare(left, right []spectreps.PageImage) spectreps.CompareResult {
	if len(left) != len(right) {
		return spectreps.CompareResult{Equal: false, Offset: -1, Reason: "length"}
	}
	for i := range left {
		if res := spectreps.CompareRaster(left[i], right[i]); !res.Equal {
			return res
		}
	}
	return spectreps.CompareResult{Equal: true, Offset: -1, Reason: ""}
}
