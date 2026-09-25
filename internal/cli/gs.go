package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// gsDefaultDPI is the resolution -g assumes, and the only one it accepts.
const gsDefaultDPI = 72

// Command and device names that appear more than once.
const (
	gsRaster   = "raster"
	gsPDFImage = "pdfimage"
	gsBBox     = "bbox"
	gsInkCov   = "inkcov"
	gsRewrite  = "rewrite"
	gsJPEG     = "jpeg"
)

var (
	errGSPageListSyntax  = errors.New("use pages and ranges, for example 1-3,5")
	errGSPageListEvenOdd = errors.New("even and odd selections are not accepted")
	errGSPageListOpen    = errors.New("open ranges are not accepted, use -dFirstPage or -dLastPage")
	errGSPageListNumber  = errors.New("pages are numbers from 1")
	errGSPageListTurn    = errors.New("ranges must run upward from page 1")
	errGSPageListGap     = errors.New("the pages are not contiguous")
	errGSPageListRepeat  = errors.New("the pages repeat or overlap")
)

// gsCommand is one parsed `spectreps gs` command line. The scanner fills the
// fields, and argv builds the command line for the existing subcommand.
type gsCommand struct {
	device         string
	command        string
	format         rasterFormat
	output         string
	hasOutput      bool
	pageText       string
	pageSwitch     string
	first          int
	last           int
	hasFirst       bool
	hasLast        bool
	width          float64
	height         float64
	hasGeometry    bool
	geometrySwitch string
	gridSize       bool
	dpi            int
	jpegq          int
	hasJPEGQ       bool
	input          string
}

// gsDevice is the subcommand and encoder one -sDEVICE name selects.
type gsDevice struct {
	command string
	format  rasterFormat
}

// cmdGS runs the bounded gs argv mode. Only the switches listed in
// documentation/gs-argv-grammar.md are accepted, and the job routes to an
// existing subcommand.
func cmdGS(args []string, stdout, stderr io.Writer) int {
	job := new(gsCommand)
	if code := scanGS(job, args, stderr); code != exitOK {
		return code
	}
	argv, code := job.argv(stderr)
	if code != exitOK {
		return code
	}
	return dispatch(argv, stdout, stderr)
}

// scanGS parses one gs argv into the job.
func scanGS(job *gsCommand, args []string, stderr io.Writer) int {
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" {
			if i+1 == len(args) {
				return gsError(stderr, "-f needs an input file")
			}
			i++
			if code := job.setInput(args[i], stderr); code != exitOK {
				return code
			}
			continue
		}
		if code := scanGSSwitch(job, args[i], stderr); code != exitOK {
			return code
		}
	}
	return exitOK
}

// scanGSSwitch routes one token. Positional tokens are the input file.
func scanGSSwitch(job *gsCommand, arg string, stderr io.Writer) int {
	switch {
	case arg == "-q":
		return exitOK
	case arg == "-c":
		return gsError(stderr, "-c runs inline PostScript, pass a file")
	case arg == "-":
		return gsError(stderr, "- is stdin, not accepted, pass a file")
	case strings.HasPrefix(arg, "@"):
		return gsError(stderr, arg+" is not accepted, pass the input path")
	case strings.HasPrefix(arg, "-"):
		return scanGSPrefix(job, arg, stderr)
	default:
		return job.setInput(arg, stderr)
	}
}

// scanGSPrefix routes the -s, -d, -r, and -g families.
func scanGSPrefix(job *gsCommand, arg string, stderr io.Writer) int {
	switch {
	case strings.HasPrefix(arg, "-s"):
		return job.setName(arg[2:], stderr)
	case strings.HasPrefix(arg, "-d"):
		return job.setDefine(arg[2:], stderr)
	case strings.HasPrefix(arg, "-r"):
		return job.setDPI(arg, stderr)
	case strings.HasPrefix(arg, "-g"):
		return job.setGridSize(arg, stderr)
	default:
		return gsError(stderr, arg+" is not in the gs allowlist")
	}
}

// setName parses one -sNAME=value token.
func (job *gsCommand) setName(text string, stderr io.Writer) int {
	name, value, hasValue := strings.Cut(text, "=")
	switch name {
	case "DEVICE":
		return job.setDevice(value, hasValue, stderr)
	case "OutputFile":
		return job.setOutput(value, hasValue, stderr)
	case "PageList":
		return job.setPageList(value, hasValue, stderr)
	default:
		return gsError(stderr, "-s"+name+" is not in the gs allowlist")
	}
}

// setDefine parses one -dNAME or -dNAME=value token.
func (job *gsCommand) setDefine(text string, stderr io.Writer) int {
	name, value, hasValue := strings.Cut(text, "=")
	switch name {
	case "NOSAFER", "DELAYSAFER":
		return gsError(stderr, "-d"+name+" is rejected, SAFER is always on")
	case "BATCH", "NOPAUSE", "SAFER", "FIXEDMEDIA":
		return gsBoolDefine(name, value, hasValue, stderr)
	default:
		return job.setValue(name, value, hasValue, stderr)
	}
}

// setValue parses the -d names that carry a value.
func (job *gsCommand) setValue(name, value string, hasValue bool, stderr io.Writer) int {
	switch name {
	case "FirstPage":
		return job.setFirstPage(value, hasValue, stderr)
	case "LastPage":
		return job.setLastPage(value, hasValue, stderr)
	case "DEVICEWIDTHPOINTS":
		return job.setWidth(value, hasValue, stderr)
	case "DEVICEHEIGHTPOINTS":
		return job.setHeight(value, hasValue, stderr)
	case "JPEGQ":
		return job.setJPEGQ(value, hasValue, stderr)
	default:
		return gsError(stderr, "-d"+name+" is not in the gs allowlist")
	}
}

// gsBoolDefine accepts the -d boolean forms. A false value would turn off
// behavior Spectre always has.
func gsBoolDefine(name, value string, hasValue bool, stderr io.Writer) int {
	if !hasValue || value == "true" || value == "1" {
		return exitOK
	}
	return gsError(stderr, "-d"+name+"="+value+" would turn off always-on behavior")
}

// setDevice maps one -sDEVICE name onto a subcommand.
func (job *gsCommand) setDevice(value string, hasValue bool, stderr io.Writer) int {
	if !hasValue || value == "" {
		return gsError(stderr, "-sDEVICE needs a device name")
	}
	device, ok := gsDeviceFor(value)
	if !ok {
		return gsError(stderr, "-sDEVICE="+value+" is not an accepted device")
	}
	job.device = value
	job.command = device.command
	job.format = device.format
	return exitOK
}

// gsDeviceFor maps the accepted device names.
func gsDeviceFor(name string) (gsDevice, bool) {
	switch name {
	case "ppmraw":
		return gsDevice{command: gsRaster, format: rasterPPM}, true
	case "png16m":
		return gsDevice{command: gsRaster, format: rasterPNG}, true
	case gsJPEG:
		return gsDevice{command: gsRaster, format: rasterJPEG}, true
	case "tiff24nc":
		return gsDevice{command: gsRaster, format: rasterTIFF}, true
	case gsBBox:
		return gsDevice{command: gsBBox, format: rasterFromPath}, true
	case gsInkCov:
		return gsDevice{command: gsInkCov, format: rasterFromPath}, true
	case "pdfimage24":
		return gsDevice{command: gsPDFImage, format: rasterFromPath}, true
	case "pdfwrite":
		return gsDevice{command: gsRewrite, format: rasterFromPath}, true
	default:
		return gsDevice{command: "", format: rasterFromPath}, false
	}
}

// setOutput keeps the -sOutputFile value. The form checks run after the scan,
// because they depend on the device.
func (job *gsCommand) setOutput(value string, hasValue bool, stderr io.Writer) int {
	if !hasValue || value == "" {
		return gsError(stderr, "-sOutputFile needs a path")
	}
	job.output = value
	job.hasOutput = true
	return exitOK
}

// setPageList maps one -sPageList value onto the -pages grammar.
func (job *gsCommand) setPageList(value string, hasValue bool, stderr io.Writer) int {
	if !hasValue || value == "" {
		return gsError(stderr, "-sPageList needs a list")
	}
	text, err := gsPageListText(value)
	if err != nil {
		return gsError(stderr, "-sPageList="+value+" is not accepted, "+err.Error())
	}
	job.pageText = text
	job.pageSwitch = "-sPageList"
	return exitOK
}

// setFirstPage maps -dFirstPage=N onto the open range N-.
func (job *gsCommand) setFirstPage(value string, hasValue bool, stderr io.Writer) int {
	page, code := gsPageNumber("-dFirstPage", value, hasValue, stderr)
	if code != exitOK {
		return code
	}
	job.first = page
	job.hasFirst = true
	job.pageSwitch = "-dFirstPage"
	return exitOK
}

// setLastPage maps -dLastPage=M onto the range 1-M.
func (job *gsCommand) setLastPage(value string, hasValue bool, stderr io.Writer) int {
	page, code := gsPageNumber("-dLastPage", value, hasValue, stderr)
	if code != exitOK {
		return code
	}
	job.last = page
	job.hasLast = true
	job.pageSwitch = "-dLastPage"
	return exitOK
}

// gsPageNumber parses the page value of one -d switch.
func gsPageNumber(name, value string, hasValue bool, stderr io.Writer) (int, int) {
	if !hasValue || value == "" {
		return 0, gsError(stderr, name+" needs =N")
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 0, gsError(stderr, name+" wants a page number, got "+strconv.Quote(value))
	}
	return page, exitOK
}

// setWidth maps -dDEVICEWIDTHPOINTS onto -w.
func (job *gsCommand) setWidth(value string, hasValue bool, stderr io.Writer) int {
	points, code := gsPointValue("-dDEVICEWIDTHPOINTS", value, hasValue, stderr)
	if code != exitOK {
		return code
	}
	job.width = points
	job.hasGeometry = true
	job.geometrySwitch = "-dDEVICEWIDTHPOINTS"
	return exitOK
}

// setHeight maps -dDEVICEHEIGHTPOINTS onto -h.
func (job *gsCommand) setHeight(value string, hasValue bool, stderr io.Writer) int {
	points, code := gsPointValue("-dDEVICEHEIGHTPOINTS", value, hasValue, stderr)
	if code != exitOK {
		return code
	}
	job.height = points
	job.hasGeometry = true
	job.geometrySwitch = "-dDEVICEHEIGHTPOINTS"
	return exitOK
}

// gsPointValue parses a positive point size.
func gsPointValue(name, value string, hasValue bool, stderr io.Writer) (float64, int) {
	if !hasValue || value == "" {
		return 0, gsError(stderr, name+" needs =N")
	}
	points, err := strconv.ParseFloat(value, 64)
	if err != nil || points <= 0 {
		return 0, gsError(stderr, name+" wants a positive size, got "+strconv.Quote(value))
	}
	return points, exitOK
}

// setJPEGQ maps -dJPEGQ onto -jpegq.
func (job *gsCommand) setJPEGQ(value string, hasValue bool, stderr io.Writer) int {
	if !hasValue || value == "" {
		return gsError(stderr, "-dJPEGQ needs =N")
	}
	quality, err := strconv.Atoi(value)
	if err != nil {
		return gsError(stderr, "-dJPEGQ wants a number, got "+strconv.Quote(value))
	}
	job.jpegq = quality
	job.hasJPEGQ = true
	return exitOK
}

// setDPI parses one -rN token. The value applies to both axes.
func (job *gsCommand) setDPI(arg string, stderr io.Writer) int {
	text := arg[2:]
	if text == "" {
		return gsError(stderr, "-r needs a resolution, e.g. -r300")
	}
	dpi, err := strconv.Atoi(text)
	if err != nil || dpi < 0 {
		return gsError(stderr, arg+" is not accepted, use -r300")
	}
	job.dpi = dpi
	job.hasGeometry = true
	job.geometrySwitch = "-r"
	return exitOK
}

// setGridSize parses one -gWxH token. The sizes count device pixels, which
// match -w and -h only at 72 dpi.
func (job *gsCommand) setGridSize(arg string, stderr io.Writer) int {
	text := arg[2:]
	left, right, isPair := strings.Cut(text, "x")
	if !isPair {
		return gsError(stderr, "-g wants WxH, got "+strconv.Quote(text))
	}
	width, errW := strconv.ParseFloat(left, 64)
	height, errH := strconv.ParseFloat(right, 64)
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return gsError(stderr, "-g wants WxH, got "+strconv.Quote(text))
	}
	job.width = width
	job.height = height
	job.hasGeometry = true
	job.gridSize = true
	job.geometrySwitch = "-g"
	return exitOK
}

// setInput keeps the one input file.
func (job *gsCommand) setInput(path string, stderr io.Writer) int {
	if job.input != "" {
		return gsUsageError(stderr, "gs takes one input file")
	}
	job.input = path
	return exitOK
}

// argv validates the finished job and builds the command line for the
// existing subcommand.
func (job *gsCommand) argv(stderr io.Writer) ([]string, int) {
	if job.command == "" {
		return nil, gsUsageError(stderr, "gs needs -sDEVICE=name")
	}
	if job.input == "" {
		return nil, gsUsageError(stderr, "gs needs one input file")
	}
	if code := job.checkSwitches(stderr); code != exitOK {
		return nil, code
	}
	output, code := job.checkOutput(stderr)
	if code != exitOK {
		return nil, code
	}
	pages, code := job.checkPages(stderr)
	if code != exitOK {
		return nil, code
	}
	if code := job.checkGrid(stderr); code != exitOK {
		return nil, code
	}
	return job.commandArgs(output, pages), exitOK
}

// checkSwitches rejects switches the selected command cannot express.
func (job *gsCommand) checkSwitches(stderr io.Writer) int {
	if job.command == gsRewrite {
		if job.hasGeometry {
			return gsError(stderr, job.geometrySwitch+" has no place on -sDEVICE="+job.device)
		}
		if job.pageSwitch != "" {
			return gsError(stderr, job.pageSwitch+" has no place on -sDEVICE="+job.device)
		}
	}
	if job.hasJPEGQ && job.device != gsJPEG {
		return gsError(stderr, "-dJPEGQ needs -sDEVICE="+gsJPEG)
	}
	if (job.command == gsBBox || job.command == gsInkCov) && job.hasOutput {
		return gsError(stderr, "-sOutputFile has no place on -sDEVICE="+job.device)
	}
	return exitOK
}

// checkOutput checks the output file and returns the path to pass on.
func (job *gsCommand) checkOutput(stderr io.Writer) (string, int) {
	if job.command != gsRaster && job.command != gsPDFImage && job.command != gsRewrite {
		return "", exitOK
	}
	if !job.hasOutput {
		return "", gsUsageError(stderr, "gs needs -sOutputFile=path for -sDEVICE="+job.device)
	}
	if job.command == gsRaster {
		return gsRasterOutput(job.output, stderr)
	}
	return gsPlainOutput(job.output, job.device, stderr)
}

// gsRasterOutput validates a raster output path. Only the two characters %d
// count as a page pattern.
func gsRasterOutput(value string, stderr io.Writer) (string, int) {
	if code := gsStreamOutput(value, stderr); code != exitOK {
		return "", code
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			continue
		}
		if i+1 == len(value) || value[i+1] != 'd' {
			return "", gsError(stderr, "-sOutputFile wants only %d in a path, got "+strconv.Quote(value))
		}
		i++
	}
	return value, exitOK
}

// gsPlainOutput validates the output path of the one-file commands.
func gsPlainOutput(value, device string, stderr io.Writer) (string, int) {
	if code := gsStreamOutput(value, stderr); code != exitOK {
		return "", code
	}
	if strings.Contains(value, "%") {
		return "", gsError(stderr, "-sOutputFile wants a plain path on -sDEVICE="+device+", got "+strconv.Quote(value))
	}
	return value, exitOK
}

// gsStreamOutput rejects the stream spellings, which have no file on disk.
func gsStreamOutput(value string, stderr io.Writer) int {
	if value == "-" || strings.HasPrefix(value, "%stdout") || strings.HasPrefix(value, "%pipe%") {
		return gsError(stderr, "-sOutputFile="+value+" is a stream, not a file path")
	}
	return exitOK
}

// checkPages maps the page switches onto one -pages value.
func (job *gsCommand) checkPages(stderr io.Writer) (string, int) {
	if job.pageText != "" {
		return job.pageListText(stderr)
	}
	if job.hasFirst && job.hasLast && job.first > job.last {
		return "", gsError(stderr, "-dLastPage="+strconv.Itoa(job.last)+" is before -dFirstPage="+strconv.Itoa(job.first))
	}
	return job.pageSpanText(), exitOK
}

// pageListText rejects a -sPageList value that also carries a first or last
// page switch.
func (job *gsCommand) pageListText(stderr io.Writer) (string, int) {
	if job.hasFirst || job.hasLast {
		return "", gsError(stderr, "-sPageList cannot be combined with -dFirstPage or -dLastPage")
	}
	return job.pageText, exitOK
}

// pageSpanText builds the -pages value from -dFirstPage and -dLastPage.
func (job *gsCommand) pageSpanText() string {
	switch {
	case job.hasFirst && job.hasLast:
		return strconv.Itoa(job.first) + "-" + strconv.Itoa(job.last)
	case job.hasFirst:
		return strconv.Itoa(job.first) + "-"
	case job.hasLast:
		return "1-" + strconv.Itoa(job.last)
	default:
		return ""
	}
}

// checkGrid enforces the 72 dpi rule for -g.
func (job *gsCommand) checkGrid(stderr io.Writer) int {
	if !job.gridSize {
		return exitOK
	}
	dpi := job.dpi
	if dpi == 0 {
		dpi = gsDefaultDPI
	}
	if dpi != gsDefaultDPI {
		return gsError(stderr, "-g needs -r "+strconv.Itoa(gsDefaultDPI)+", got -r "+strconv.Itoa(dpi))
	}
	return exitOK
}

// commandArgs builds the argv for the routed subcommand.
func (job *gsCommand) commandArgs(output, pages string) []string {
	argv := []string{job.command}
	switch job.command {
	case gsRaster:
		argv = append(argv, "-format", job.format.String())
		argv = append(argv, job.geometryArgs()...)
		if job.hasJPEGQ {
			argv = append(argv, "-jpegq", strconv.Itoa(job.jpegq))
		}
		argv = append(argv, "-o", output)
	case gsPDFImage:
		argv = append(argv, job.geometryArgs()...)
		argv = append(argv, "-o", output)
	case gsBBox, gsInkCov:
		argv = append(argv, job.geometryArgs()...)
	default:
		argv = append(argv, "-o", output)
	}
	if pages != "" {
		argv = append(argv, "-pages", pages)
	}
	return append(argv, job.input)
}

// geometryArgs builds the shared -w, -h, and -r flags.
func (job *gsCommand) geometryArgs() []string {
	return []string{
		"-w", gsNumber(job.width),
		"-h", gsNumber(job.height),
		"-r", strconv.Itoa(job.dpi),
	}
}

// gsNumber formats one geometry value as the flag package reads it.
func gsNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// gsPageListText maps one -sPageList value onto a -pages value. The elements
// must name one contiguous, increasing range.
func gsPageListText(value string) (string, error) {
	elements := strings.Split(value, ",")
	start, end := 0, 0
	for i, element := range elements {
		low, high, err := gsPageElement(strings.TrimSpace(element))
		if err != nil {
			return "", err
		}
		if i == 0 {
			start, end = low, high
			continue
		}
		if low <= end {
			return "", errGSPageListRepeat
		}
		if low != end+1 {
			return "", errGSPageListGap
		}
		end = high
	}
	if start == end {
		return strconv.Itoa(start), nil
	}
	return strconv.Itoa(start) + "-" + strconv.Itoa(end), nil
}

// gsPageElement parses one N or A-B element. Open and reversed ranges, even
// and odd selections, and page 0 stay rejected.
func gsPageElement(element string) (int, int, error) {
	if element == "even" || element == "odd" {
		return 0, 0, errGSPageListEvenOdd
	}
	left, right, isRange := strings.Cut(element, "-")
	if !isRange {
		return gsPageSingle(element)
	}
	return gsPageRange(left, right)
}

// gsPageSingle parses one page number.
func gsPageSingle(element string) (int, int, error) {
	page, err := strconv.Atoi(element)
	if err != nil || page < 1 {
		return 0, 0, errGSPageListNumber
	}
	return page, page, nil
}

// gsPageRange parses one A-B range.
func gsPageRange(left, right string) (int, int, error) {
	if left == "" || right == "" {
		return 0, 0, errGSPageListOpen
	}
	if strings.Contains(right, "-") {
		return 0, 0, errGSPageListSyntax
	}
	low, errLow := strconv.Atoi(left)
	high, errHigh := strconv.Atoi(right)
	if errLow != nil || errHigh != nil || low < 1 {
		return 0, 0, errGSPageListNumber
	}
	if high < low {
		return 0, 0, errGSPageListTurn
	}
	return low, high, nil
}

// gsError writes a rejection and returns the usage exit code.
func gsError(stderr io.Writer, message string) int {
	fmt.Fprintln(stderr, "spectreps: "+message)
	return exitUsage
}

// gsUsageError prints the usage and then the named rejection.
func gsUsageError(stderr io.Writer, message string) int {
	usage(stderr)
	return gsError(stderr, message)
}
