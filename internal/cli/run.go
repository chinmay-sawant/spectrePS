package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

const (
	exitOK    = 0
	exitJob   = 1
	exitUsage = 2
	exitIO    = 3

	comparePaths = 2
)

// Run parses args and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stderr)
	case "raster":
		return cmdRaster(args[1:], stderr)
	case "rewrite":
		return cmdRewrite(args[1:], stderr)
	case "validate":
		return cmdValidate(args[1:], stderr)
	case "compare":
		return cmdCompare(args[1:], stdout, stderr)
	default:
		usage(stderr)
		return exitUsage
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `spectreps version
spectreps run [-w points] [-h points] [-r dpi] [-o path] file
spectreps raster [-w points] [-h points] [-r dpi] -o path file
spectreps rewrite [-compress] -o path file.pdf
spectreps validate file
spectreps compare bytes fileA fileB
spectreps compare raster [-w points] [-h points] [-r dpi] [-o path] fileA fileB
`)
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		usage(stderr)
		return exitUsage
	}
	fmt.Fprintln(stdout, spectreps.Version())
	return exitOK
}

func cmdRun(args []string, stderr io.Writer) int {
	rest, opt, outPath, code := parsePageFlags("run", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != 1 {
		usage(stderr)
		return exitUsage
	}
	// Accepted and ignored until the command writes a file.
	_ = outPath
	return withInput(stderr, rest[0], func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		_, err := in.RunPostScript(ctx, src, opt)
		return err
	})
}

func cmdRaster(args []string, stderr io.Writer) int {
	rest, opt, outPath, code := parsePageFlags("raster", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || outPath == "" {
		usage(stderr)
		return exitUsage
	}
	path := rest[0]
	return withInput(stderr, path, func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		if strings.HasSuffix(path, ".pdf") {
			doc, err := in.OpenPDF(ctx, src)
			if err != nil {
				return err
			}
			_, err = in.RasterizePage(ctx, doc, 0, opt)
			return err
		}
		_, err := in.RunPostScript(ctx, src, opt)
		return err
	})
}

func cmdRewrite(args []string, stderr io.Writer) int {
	set := newFlagSet("rewrite", stderr)
	outPath := set.String("o", "", "output path")
	compress := set.Bool("compress", true, "flate content streams")
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	var opt spectreps.RewriteOptions
	if *compress {
		opt = spectreps.DefaultRewriteOptions()
	} else {
		opt = spectreps.RewriteOptions{CompressStreams: false}
	}
	return withInput(stderr, rest[0], func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		doc, err := in.OpenPDF(ctx, src)
		if err != nil {
			return err
		}
		_, err = in.RewritePDF(ctx, doc, opt)
		return err
	})
}

func cmdValidate(args []string, stderr io.Writer) int {
	rest, code := parseBare("validate", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != 1 {
		usage(stderr)
		return exitUsage
	}
	path := rest[0]
	return withInput(stderr, path, func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		if strings.HasSuffix(path, ".pdf") {
			_, err := in.OpenPDF(ctx, src)
			return err
		}
		_, err := in.RunPostScript(ctx, src, spectreps.RunOptions{
			PageWidthPt:   0,
			PageHeightPt:  0,
			ResolutionDPI: 0,
		})
		return err
	})
}

func cmdCompare(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "bytes":
		return cmdCompareBytes(args[1:], stdout, stderr)
	case "raster":
		return cmdCompareRaster(args[1:], stderr)
	default:
		usage(stderr)
		return exitUsage
	}
}

func cmdCompareBytes(args []string, stdout, stderr io.Writer) int {
	rest, code := parseBare("compare bytes", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != comparePaths {
		usage(stderr)
		return exitUsage
	}
	a, code := readFile(rest[0], stderr)
	if code != 0 {
		return code
	}
	b, code := readFile(rest[1], stderr)
	if code != 0 {
		return code
	}
	res := spectreps.CompareFiles(a, b)
	if res.Equal {
		return exitOK
	}
	switch res.Reason {
	case "byte", "pixel", "length":
		fmt.Fprintf(stdout, "mismatch %s %d\n", res.Reason, res.Offset)
	default:
		fmt.Fprintf(stdout, "mismatch %s\n", res.Reason)
	}
	return exitJob
}

func cmdCompareRaster(args []string, stderr io.Writer) int {
	rest, opt, outPath, code := parsePageFlags("compare raster", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != comparePaths {
		usage(stderr)
		return exitUsage
	}
	return withInputs(stderr, rest, func(context.Context, *spectreps.Instance, [][]byte) error {
		return compareRasterUnimplemented(opt, outPath)
	})
}

// CompareRaster panics with ErrNotImplemented in this phase.
func compareRasterUnimplemented(spectreps.RunOptions, string) error {
	return spectreps.ErrNotImplemented
}

func parsePageFlags(name string, args []string, stderr io.Writer) ([]string, spectreps.RunOptions, string, int) {
	set := newFlagSet(name, stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	outPath := set.String("o", "", "output path")
	rest, code := parseSet(set, args)
	if code != 0 {
		return nil, spectreps.RunOptions{
			PageWidthPt:   0,
			PageHeightPt:  0,
			ResolutionDPI: 0,
		}, "", code
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   *w,
		PageHeightPt:  *h,
		ResolutionDPI: *r,
	}
	return rest, opt, *outPath, exitOK
}

func parseBare(name string, args []string, stderr io.Writer) ([]string, int) {
	return parseSet(newFlagSet(name, stderr), args)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() { usage(stderr) }
	return set
}

func parseSet(set *flag.FlagSet, args []string) ([]string, int) {
	if err := set.Parse(args); err != nil {
		return nil, exitUsage
	}
	return set.Args(), exitOK
}

func withInput(stderr io.Writer, path string, fn func(context.Context, *spectreps.Instance, []byte) error) int {
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	src, code := readFile(path, stderr)
	if code != 0 {
		return code
	}
	return finish(stderr, fn(context.Background(), in, src))
}

func withInputs(stderr io.Writer, paths []string, fn func(context.Context, *spectreps.Instance, [][]byte) error) int {
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	srcs := make([][]byte, len(paths))
	for i, path := range paths {
		src, readCode := readFile(path, stderr)
		if readCode != 0 {
			return readCode
		}
		srcs[i] = src
	}
	return finish(stderr, fn(context.Background(), in, srcs))
}

func newInstance(stderr io.Writer) (*spectreps.Instance, int) {
	in, err := spectreps.New()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return nil, exitJob
	}
	return in, exitOK
}

func finish(stderr io.Writer, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitJob
	}
	return exitOK
}

func readFile(path string, stderr io.Writer) ([]byte, int) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, exitOK
	}
	fmt.Fprintln(stderr, err.Error())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, exitUsage
	}
	return nil, exitIO
}
