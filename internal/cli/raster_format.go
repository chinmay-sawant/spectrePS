package cli

// rasterFormat selects the raster encoder. The zero value follows the -o
// suffix. The -format flag overrides the suffix, and the gs mode passes the
// format from -sDEVICE.
type rasterFormat uint8

const (
	rasterFromPath rasterFormat = iota
	rasterPPM
	rasterPNG
	rasterJPEG
	rasterTIFF
)

// parseRasterFormat maps the -format flag text. The empty name follows the
// output suffix.
func parseRasterFormat(name string) (rasterFormat, bool) {
	switch name {
	case "":
		return rasterFromPath, true
	case "ppm":
		return rasterPPM, true
	case "png":
		return rasterPNG, true
	case "jpeg":
		return rasterJPEG, true
	case "tiff":
		return rasterTIFF, true
	default:
		return rasterFromPath, false
	}
}

// String returns the flag text for the encoder.
func (format rasterFormat) String() string {
	switch format {
	case rasterFromPath:
		return ""
	case rasterPPM:
		return "ppm"
	case rasterPNG:
		return "png"
	case rasterJPEG:
		return "jpeg"
	case rasterTIFF:
		return "tiff"
	default:
		return ""
	}
}
