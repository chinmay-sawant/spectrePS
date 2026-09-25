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
	"math"
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
	maxRewriteLevel    = 5
	inkPercentScale    = 100
)

// Run parses args and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	return dispatch(args, stdout, stderr)
}

//nolint:cyclop // one case per command, and the list is the CLI
func dispatch(args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stderr)
	case "raster":
		return cmdRaster(args[1:], stderr)
	case "pdfimage":
		return cmdPDFImage(args[1:], stderr)
	case "bbox", "inkcov", "ink_cov":
		return cmdMeasure(args[0], args[1:], stdout, stderr)
	case "rewrite":
		return cmdRewrite(args[1:], stderr)
	case "ps":
		return cmdPS(args[1:], stderr)
	case "validate":
		return cmdValidate(args[1:], stderr)
	case "compare":
		return cmdCompare(args[1:], stdout, stderr)
	case "gs":
		return cmdGS(args[1:], stdout, stderr)
	}
	usage(stderr)
	return exitUsage
}

func usage(w io.Writer) {
	fmt.Fprint(w, `spectreps version
spectreps run [-w points] [-h points] [-r dpi] [-o path] file
spectreps raster [-w points] [-h points] [-r dpi] [-format ppm|png|jpeg|tiff]
                [-jpegq quality] [-tiffcompress none|deflate] [-pages range] -o path file
spectreps pdfimage [-w points] [-h points] [-r dpi] [-colorspace rgb|gray|cmyk] [-pages range] -o path file
spectreps bbox [-w points] [-h points] [-r dpi] [-pages range] file
spectreps inkcov [-w points] [-h points] [-r dpi] [-pages range] file
spectreps ink_cov [-w points] [-h points] [-r dpi] [-pages range] file
spectreps rewrite [-compress] [-level N] -o path file.pdf
spectreps ps -o path file.pdf
spectreps validate file
spectreps compare bytes fileA fileB
spectreps compare raster [-w points] [-h points] [-r dpi] [-pages range] [-o path] fileA fileB
spectreps gs [-sDEVICE=name] [-sOutputFile=path] [switches] file
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
	// run accepts -pages and ignores it, the same way it accepts -o.
	rest, opt, _, code := parsePageFlags("run", args, stderr)
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
	tiffcompress := set.String("tiffcompress", "deflate", "tiff compression, none or deflate")
	format := set.String("format", "", "output format: ppm, png, jpeg, or tiff")
	sel := pageFlag(set)
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	encoding, ok := tiffEncodingFromFlag(*tiffcompress)
	if !ok {
		fmt.Fprintf(stderr, "spectreps: -tiffcompress wants none or deflate, got %q\n", *tiffcompress)
		return exitUsage
	}
	formatChoice, ok := parseRasterFormat(*format)
	if !ok {
		fmt.Fprintf(stderr, "spectreps: -format wants ppm, png, jpeg, or tiff, got %q\n", *format)
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
	pages, err := pageImages(in, path, src, opt, *sel)
	if err != nil {
		return finish(stderr, err)
	}
	if len(pages) == 0 {
		// A PDF with no pages has no page 1 to paint.
		return finish(stderr, spectreps.JobError{
			Op: "RasterizePage", Msg: "rangecheck", Filename: "", Line: 0, Column: 0,
		})
	}
	return writePages(*outPath, pages, clampJPEGQuality(*jpegq), encoding, formatChoice, stderr)
}

func cmdPDFImage(args []string, stderr io.Writer) int {
	set := newFlagSet("pdfimage", stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	spaceName := set.String("colorspace", "rgb", "image color space: rgb, gray, or cmyk")
	outPath := set.String("o", "", "output path")
	sel := pageFlag(set)
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	color, ok := parseImageColor(*spaceName)
	if !ok {
		usage(stderr)
		return exitUsage
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
	pages, err := pageImages(in, path, src, opt, *sel)
	if err != nil {
		return finish(stderr, err)
	}
	payload, err := in.ImagePDFColor(context.Background(), pages, float64(opt.ResolutionDPI), color)
	if err != nil {
		return finish(stderr, err)
	}
	return writeRewrite(*outPath, payload, stderr)
}

// parseImageColor maps the -colorspace flag value.
func parseImageColor(value string) (spectreps.ImageColor, bool) {
	switch value {
	case "rgb":
		return spectreps.ImageColorRGB, true
	case "gray":
		return spectreps.ImageColorGray, true
	case "cmyk":
		return spectreps.ImageColorCMYK, true
	default:
		return spectreps.ImageColorRGB, false
	}
}

func cmdMeasure(name string, args []string, stdout, stderr io.Writer) int {
	pages, opt, code := measurePages(name, args, stderr)
	if code != 0 {
		return code
	}
	for i, page := range pages {
		switch name {
		case "bbox":
			writeBBox(stdout, page, opt.ResolutionDPI)
		case "ink_cov":
			writeInkAmount(stdout, i+1, page)
		default:
			writeInk(stdout, i+1, page)
		}
	}
	return exitOK
}

// measurePages rasterizes the selected pages of one PostScript or PDF input.
func measurePages(name string, args []string, stderr io.Writer) ([]spectreps.PageImage, spectreps.RunOptions, int) {
	rest, opt, sel, code := parsePageFlags(name, args, stderr)
	if code != 0 {
		return nil, opt, code
	}
	if len(rest) != 1 {
		usage(stderr)
		return nil, opt, exitUsage
	}
	path := rest[0]
	in, code := newInstance(stderr)
	if code != 0 {
		return nil, opt, code
	}
	defer in.Close()
	src, code := readFile(path, stderr)
	if code != 0 {
		return nil, opt, code
	}
	pages, err := pageImages(in, path, src, opt, sel)
	if err != nil {
		return nil, opt, finish(stderr, err)
	}
	return pages, opt, exitOK
}

func writeBBox(w io.Writer, img spectreps.PageImage, dpi int) {
	box, ok := spectreps.MeasureBox(img, float64(dpi))
	if !ok {
		fmt.Fprintln(w, "%%BoundingBox: 0 0 0 0")
		fmt.Fprintln(w, "%%HiResBoundingBox: 0 0 0 0")
		return
	}
	fmt.Fprintf(w, "%%%%BoundingBox: %d %d %d %d\n",
		int(math.Floor(box.MinX)), int(math.Floor(box.MinY)),
		int(math.Ceil(box.MaxX)), int(math.Ceil(box.MaxY)))
	fmt.Fprintf(w, "%%%%HiResBoundingBox: %s %s %s %s\n",
		pointText(box.MinX), pointText(box.MinY), pointText(box.MaxX), pointText(box.MaxY))
}

func pointText(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func writeInk(w io.Writer, page int, img spectreps.PageImage) {
	ink := spectreps.MeasureInk(img)
	fmt.Fprintf(w, "Page %d\n", page)
	fmt.Fprintf(w, "%.5f %.5f %.5f RGB\n", ink.R, ink.G, ink.B)
}

// writeInkAmount prints the weighted amount as a percent per channel.
func writeInkAmount(w io.Writer, page int, img spectreps.PageImage) {
	ink := spectreps.MeasureInkAmount(img)
	fmt.Fprintf(w, "Page %d\n", page)
	fmt.Fprintf(w, "%.5f %.5f %.5f RGB\n",
		ink.R*inkPercentScale, ink.G*inkPercentScale, ink.B*inkPercentScale)
}

func cmdRewrite(args []string, stderr io.Writer) int {
	set := newFlagSet("rewrite", stderr)
	outPath := set.String("o", "", "output path")
	compress := set.Bool("compress", true, "flate content streams at level 0")
	level := set.Int("level", 0, "compression level, 0 through 5")
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if *level < 0 || *level > maxRewriteLevel {
		fmt.Fprintf(stderr, "spectreps: -level wants 0 through 5, got %d\n", *level)
		return exitUsage
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	return rewriteToFile(stderr, rest[0], *outPath, rewriteOptions(*level, *compress))
}

// rewriteOptions maps the flags. An explicit level above 0 wins. Level 0 keeps
// the -compress switch as the Flate option.
func rewriteOptions(level int, compress bool) spectreps.RewriteOptions {
	if level > 0 {
		return spectreps.RewriteOptions{CompressStreams: true, Level: level}
	}
	return spectreps.RewriteOptions{CompressStreams: compress, Level: 0}
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

func cmdPS(args []string, stderr io.Writer) int {
	set := newFlagSet("ps", stderr)
	outPath := set.String("o", "", "output path")
	rest, code := parseSet(set, args)
	if code != 0 {
		return code
	}
	if len(rest) != 1 || *outPath == "" {
		usage(stderr)
		return exitUsage
	}
	return psToFile(stderr, rest[0], *outPath)
}

// psToFile opens a PDF and writes its path subset as PostScript.
func psToFile(stderr io.Writer, inPath, outPath string) int {
	in, code := newInstance(stderr)
	if code != 0 {
		return code
	}
	defer in.Close()
	src, code := readFile(inPath, stderr)
	if code != 0 {
		return code
	}
	payload, err := psBytes(in, src)
	if err != nil {
		return finish(stderr, err)
	}
	return writeRewrite(outPath, payload, stderr)
}

func psBytes(in *spectreps.Instance, src []byte) ([]byte, error) {
	ctx := context.Background()
	doc, err := in.OpenPDF(ctx, src)
	if err != nil {
		return nil, err
	}
	return in.WritePostScript(ctx, doc, spectreps.PostScriptOptions{})
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
	rest, opt, sel, code := parsePageFlags("compare raster", args, stderr)
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
	res, err := pageRasterPair(in, rest[0], leftSrc, rest[1], rightSrc, opt, sel)
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

func writePages(
	outPath string,
	pages []spectreps.PageImage,
	jpegQuality int,
	tiffCompress tiffEncoding,
	format rasterFormat,
	stderr io.Writer,
) int {
	if len(pages) > 1 && !strings.Contains(outPath, "%d") {
		fmt.Fprintln(stderr, "spectreps: multiple pages need a page number in the output path")
		return exitUsage
	}
	for i, page := range pages {
		path := outPath
		if strings.Contains(path, "%d") {
			path = strings.ReplaceAll(path, "%d", strconv.Itoa(i+1))
		}
		payload, err := encodePage(path, page, jpegQuality, tiffCompress, format)
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

func encodePage(
	path string,
	page spectreps.PageImage,
	jpegQuality int,
	tiffCompress tiffEncoding,
	format rasterFormat,
) ([]byte, error) {
	switch format {
	case rasterPPM:
		return encodePPM(page), nil
	case rasterPNG:
		return encodePNG(page)
	case rasterJPEG:
		return encodeJPEG(page, jpegQuality)
	case rasterTIFF:
		return encodeTIFF(page, tiffCompress)
	case rasterFromPath:
		// The zero value follows the -o suffix below.
	}
	switch {
	case strings.HasSuffix(path, ".png"):
		return encodePNG(page)
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return encodeJPEG(page, jpegQuality)
	case strings.HasSuffix(path, ".tif"), strings.HasSuffix(path, ".tiff"):
		return encodeTIFF(page, tiffCompress)
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

func parsePageFlags(name string, args []string, stderr io.Writer) ([]string, spectreps.RunOptions, pageSelection, int) {
	set := newFlagSet(name, stderr)
	w := set.Float64("w", 0, "page width in points")
	h := set.Float64("h", 0, "page height in points")
	r := set.Int("r", 0, "pixels per inch")
	// run and compare raster accept -o and ignore it.
	set.String("o", "", "output path")
	sel := pageFlag(set)
	rest, code := parseSet(set, args)
	if code != 0 {
		return nil, spectreps.RunOptions{
			PageWidthPt:   0,
			PageHeightPt:  0,
			ResolutionDPI: 0,
		}, pageSelection{text: ""}, code
	}
	opt := spectreps.RunOptions{
		PageWidthPt:   *w,
		PageHeightPt:  *h,
		ResolutionDPI: *r,
	}
	return rest, opt, *sel, exitOK
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
