package spectreps

// PDFPageSize is one resolved page /MediaBox in points.
type PDFPageSize struct {
	Width  float64
	Height float64
}

// PDFFontInfo is one font dictionary: its /BaseFont name and whether an
// outline program is in the file.
type PDFFontInfo struct {
	Name     string
	Embedded bool
}

// PDFInfo is the read-only document summary behind spectreps info.
type PDFInfo struct {
	Version   string
	Pages     int
	PageSizes []PDFPageSize
	Tagged    bool
	Fonts     []PDFFontInfo
	Images    int
}

// Info reads the open document summary. It resolves objects and writes
// nothing. A page with no /MediaBox in its tree reports 612 by 792 points,
// and fonts come from every in-use /Type /Font object.
// A nil document returns a JobError with Op "Info" and Msg "rangecheck".
func (doc *Document) Info() (PDFInfo, error) {
	if doc == nil || doc.file == nil {
		return PDFInfo{}, JobError{Op: "Info", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
	}
	report, err := doc.file.Info()
	if err != nil {
		return PDFInfo{}, asPDFJobError(err)
	}
	pageSizes := make([]PDFPageSize, 0, len(report.PageSizes))
	for _, size := range report.PageSizes {
		pageSizes = append(pageSizes, PDFPageSize{Width: size.Width, Height: size.Height})
	}
	fonts := make([]PDFFontInfo, 0, len(report.Fonts))
	for _, font := range report.Fonts {
		fonts = append(fonts, PDFFontInfo{Name: font.Name, Embedded: font.Embedded})
	}
	return PDFInfo{
		Version:   report.Version,
		Pages:     report.Pages,
		PageSizes: pageSizes,
		Tagged:    report.Tagged,
		Fonts:     fonts,
		Images:    report.Images,
	}, nil
}
