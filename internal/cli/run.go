package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

const (
	exitOK    = 0
	exitJob   = 1
	exitUsage = 2
	exitIO    = 3

	comparePaths       = 2
	rasterFileMode     = 0o600
	rgbChannels        = 3
	rgbaChannels       = 4
	jpegDefaultQuality = 75
	jpegMinQuality     = 1
	jpegMaxQuality     = 100
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
	case "pdfimage":
		return cmdPDFImage(args[1:], stderr)
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
spectreps raster [-w points] [-h points] [-r dpi] [-jpegq quality] -o path file
spectreps pdfimage [-w points] [-h points] [-r dpi] -o path file
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
	rest, opt, code := parsePageFlags("run", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != 1 {
		usage(stderr)
		return exitUsage
	}
	return withInput(stderr, rest[0], func(ctx context.Context, in *spectreps.Instance, src []byte) error {
		_, err := in.RunPostScript(ctx, src, opt)
		return err
	})
}

func cmdRaster(args []string, stderr io.Writer) int {
	set := newFlagSet("raster", stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	outPath := set.String("o", "", "output path")
	jpegq := set.Int("jpegq", jpegDefaultQuality, "jpeg quality, 1 to 100")
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   *w,
		PageHeightPt:  *h,
		ResolutionDPI: *r,
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
	var pages []spectreps.PageImage
	var err error
	if strings.HasSuffix(path, ".pdf") {
		doc, openErr := in.OpenPDF(context.Background(), src)
		if openErr != nil {
			return finish(stderr, openErr)
		}
		var page spectreps.PageImage
		page, err = in.RasterizePage(context.Background(), doc, 0, opt)
		pages = []spectreps.PageImage{page}
	} else {
		pages, err = in.RunPostScript(context.Background(), src, opt)
	}
	if err != nil {
		return finish(stderr, err)
	}
	return writePages(*outPath, pages, clampJPEGQuality(*jpegq), stderr)
}

func cmdPDFImage(args []string, stderr io.Writer) int {
	set := newFlagSet("pdfimage", stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	outPath := set.String("o", "", "output path")
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   *w,
		PageHeightPt:  *h,
		ResolutionDPI: *r,
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
	pages, err := pdfImagePages(in, path, src, opt)
	if err != nil {
		return finish(stderr, err)
	}
	payload, err := in.ImagePDF(context.Background(), pages, float64(opt.ResolutionDPI))
	if err != nil {
		return finish(stderr, err)
	}
	return writeRewrite(*outPath, payload, stderr)
}

// pdfImagePages paints every page with the raster path. A .pdf input uses
// RasterizePage for each page. Any other input uses RunPostScript.
func pdfImagePages(
	in *spectreps.Instance,
	path string,
	src []byte,
	opt spectreps.RunOptions,
) ([]spectreps.PageImage, error) {
	ctx := context.Background()
	if !strings.HasSuffix(path, ".pdf") {
		return in.RunPostScript(ctx, src, opt)
	}
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return nil, err
	}
	pages := make([]spectreps.PageImage, 0, doc.PageCount())
	for page := range doc.PageCount() {
		image, err := in.RasterizePage(ctx, doc, page, opt)
		if err != nil {
			return nil, err
		}
		pages = append(pages, image)
	}
	return pages, nil
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
	return rewriteToFile(stderr, rest[0], *outPath, rewriteOptions(*compress))
}

func rewriteOptions(compress bool) spectreps.RewriteOptions {
	if compress {
		return spectreps.DefaultRewriteOptions()
	}
	return spectreps.RewriteOptions{CompressStreams: false}
}

func rewriteToFile(stderr io.Writer, inPath, outPath string, opt spectreps.RewriteOptions) int {
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	src, code := readFile(inPath, stderr)
	if code != 0 {
		return code
	}
	payload, err := rewriteBytes(in, src, opt)
	if err != nil {
		return finish(stderr, err)
	}
	return writeRewrite(outPath, payload, stderr)
}

func rewriteBytes(in *spectreps.Instance, src []byte, opt spectreps.RewriteOptions) ([]byte, error) {
	ctx := context.Background()
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return nil, err
	}
	return in.RewritePDF(ctx, doc, opt)
}

func writeRewrite(path string, payload []byte, stderr io.Writer) int {
	if err := os.WriteFile(path, payload, rasterFileMode); err != nil {
		fmt.Fprintln(stderr, err.Error())
		if errors.Is(err, fs.ErrNotExist) {
			return exitUsage
		}
		return exitIO
	}
	return exitOK
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
			return validatePDF(ctx, in, src)
		}
		_, err := in.RunPostScript(ctx, src, spectreps.RunOptions{
			PageWidthPt:   0,
			PageHeightPt:  0,
			ResolutionDPI: 0,
		})
		return err
	})
}

func validatePDF(ctx context.Context, in *spectreps.Instance, src []byte) error {
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return err
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   0,
		PageHeightPt:  0,
		ResolutionDPI: 0,
	}
	for page := range doc.PageCount() {
		if _, err = in.RasterizePage(ctx, doc, page, opt); err != nil {
			return err
		}
	}
	return nil
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
		return cmdCompareRaster(args[1:], stdout, stderr)
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

func cmdCompareRaster(args []string, stdout, stderr io.Writer) int {
	rest, opt, code := parsePageFlags("compare raster", args, stderr)
	if code != 0 {
		return code
	}
	if len(rest) != comparePaths {
		usage(stderr)
		return exitUsage
	}
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	leftSrc, code := readFile(rest[0], stderr)
	if code != 0 {
		return code
	}
	rightSrc, code := readFile(rest[1], stderr)
	if code != 0 {
		return code
	}
	res, err := rasterPair(in, leftSrc, rightSrc, opt)
	if err != nil {
		return finish(stderr, err)
	}
	if res.Equal {
		return exitOK
	}
	if res.Reason == "pixel" {
		fmt.Fprintf(stdout, "mismatch pixel %d\n", res.Offset)
		return exitJob
	}
	fmt.Fprintf(stdout, "mismatch %s\n", res.Reason)
	return exitJob
}

func rasterPair(
	in *spectreps.Instance,
	leftSrc, rightSrc []byte,
	opt spectreps.RunOptions,
) (spectreps.CompareResult, error) {
	ctx := context.Background()
	left, err := in.RunPostScript(ctx, leftSrc, opt)
	if err != nil {
		return spectreps.CompareResult{}, err
	}
	right, err := in.RunPostScript(ctx, rightSrc, opt)
	if err != nil {
		return spectreps.CompareResult{}, err
	}
	if len(left) == 0 || len(right) == 0 {
		return spectreps.CompareResult{}, spectreps.JobError{
			Op: "compare", Msg: "rangecheck", Filename: "", Line: 0, Column: 0,
		}
	}
	return spectreps.CompareRaster(left[0], right[0]), nil
}

func writePages(outPath string, pages []spectreps.PageImage, jpegQuality int, stderr io.Writer) int {
	if len(pages) > 1 && !strings.Contains(outPath, "%d") {
		fmt.Fprintln(stderr, "spectreps: multiple pages need a page number in the output path")
		return exitUsage
	}
	for i, page := range pages {
		path := outPath
		if strings.Contains(path, "%d") {
			path = strings.ReplaceAll(path, "%d", strconv.Itoa(i+1))
		}
		payload, err := encodePage(path, page, jpegQuality)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitIO
		}
		if err := os.WriteFile(path, payload, rasterFileMode); err != nil {
			fmt.Fprintln(stderr, err.Error())
			if errors.Is(err, fs.ErrNotExist) {
				return exitUsage
			}
			return exitIO
		}
	}
	return exitOK
}

func encodePage(path string, page spectreps.PageImage, jpegQuality int) ([]byte, error) {
	switch {
	case strings.HasSuffix(path, ".png"):
		return encodePNG(page)
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return encodeJPEG(page, jpegQuality)
	default:
		return encodePPM(page), nil
	}
}

func encodePPM(img spectreps.PageImage) []byte {
	header := fmt.Sprintf("P6\n%d %d\n255\n", img.Width, img.Height)
	body := make([]byte, 0, img.Width*img.Height*rgbChannels)
	for row := range img.Height {
		base := row * img.Stride
		body = append(body, img.Pixels[base:base+img.Width*rgbChannels]...)
	}
	return append([]byte(header), body...)
}

func encodePNG(img spectreps.PageImage) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgbaFromPage(img)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeJPEG(img spectreps.PageImage, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgbaFromPage(img), &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rgbaFromPage(img spectreps.PageImage) *image.RGBA {
	pic := image.NewRGBA(image.Rect(0, 0, img.Width, img.Height))
	for y := range img.Height {
		for x := range img.Width {
			src := y*img.Stride + x*rgbChannels
			dst := y*pic.Stride + x*rgbaChannels
			pic.Pix[dst] = img.Pixels[src]
			pic.Pix[dst+1] = img.Pixels[src+1]
			pic.Pix[dst+2] = img.Pixels[src+2]
			pic.Pix[dst+3] = 255
		}
	}
	return pic
}

func clampJPEGQuality(quality int) int {
	if quality < jpegMinQuality {
		return jpegMinQuality
	}
	if quality > jpegMaxQuality {
		return jpegMaxQuality
	}
	return quality
}

func parsePageFlags(name string, args []string, stderr io.Writer) ([]string, spectreps.RunOptions, int) {
	set := newFlagSet(name, stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	// run and compare raster accept -o and ignore it.
	set.String("o", "", "output path")
	rest, code := parseSet(set, args)
	if code != 0 {
		return nil, spectreps.RunOptions{
			PageWidthPt:   0,
			PageHeightPt:  0,
			ResolutionDPI: 0,
		}, code
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   *w,
		PageHeightPt:  *h,
		ResolutionDPI: *r,
	}
	return rest, opt, exitOK
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
