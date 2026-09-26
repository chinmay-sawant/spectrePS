package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// cmdInfo prints the read-only PDF summary: version, page count and sizes,
// the tagged flag, fonts with the embedded flag, and the image count. The
// command writes nothing else.
func cmdInfo(args []string, stdout, stderr io.Writer) int {
	rest, code := parseBare("info", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != 1 {
		usage(stderr)
		return exitUsage
	}
	return withInput(stderr, rest[0], func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		doc, err := in.OpenPDF(ctx, src)
		if err != nil {
			return err
		}
		info, err := doc.Info()
		if err != nil {
			return err
		}
		writeInfo(stdout, info)
		return nil
	})
}

// writeInfo prints one stable line per field. Page numbers are one-based.
func writeInfo(w io.Writer, info spectreps.PDFInfo) {
	fmt.Fprintf(w, "PDF version: %s\n", info.Version)
	fmt.Fprintf(w, "Pages: %d\n", info.Pages)
	for index, size := range info.PageSizes {
		fmt.Fprintf(w, "Page %d: %s x %s\n", index+1, pointText(size.Width), pointText(size.Height))
	}
	fmt.Fprintf(w, "Tagged: %t\n", info.Tagged)
	if len(info.Fonts) == 0 {
		fmt.Fprintln(w, "Fonts: none")
	} else {
		fmt.Fprintln(w, "Fonts:")
		for _, font := range info.Fonts {
			fmt.Fprintf(w, "  %s embedded=%t\n", font.Name, font.Embedded)
		}
	}
	fmt.Fprintf(w, "Images: %d\n", info.Images)
}
