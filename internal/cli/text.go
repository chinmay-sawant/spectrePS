package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// cmdText prints the extracted text of the selected PDF pages. The output is
// UTF-8 with one CRLF per line; text pixels are a different question.
func cmdText(args []string, stdout, stderr io.Writer) int {
	set := newFlagSet("text", stderr)
	sel := pageFlag(set)
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 {
		usage(stderr)
		return exitUsage
	}
	path := rest[0]
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	src, code := readFile(path, stderr)
	if code != 0 {
		return code
	}
	return writeText(stdout, stderr, in, src, sel)
}

func writeText(
	stdout, stderr io.Writer,
	in *spectreps.Instance,
	src []byte,
	sel *pageSelection,
) int {
	ctx := context.Background()
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return finish(stderr, err)
	}
	indices, err := sel.pageIndices(doc.PageCount())
	if err != nil {
		return finish(stderr, err)
	}
	for _, index := range indices {
		text, err := in.ExtractText(ctx, doc, index)
		if err != nil {
			return finish(stderr, err)
		}
		fmt.Fprint(stdout, text)
	}
	return exitOK
}
