package pdfa

import (
	"bytes"
	"encoding/binary"
	"math"
)

// The profile is an ICC v2.1 matrix-shaper for sRGB, adapted to the D50 PCS.
// The date in the header is fixed so two calls return equal bytes. The curve
// samples are the sRGB transfer function at 1024 points.
const (
	iccHeaderSize   = 128
	iccTagRowSize   = 12
	iccTagCountSize = 4
	iccWordSize     = 4
	iccAlign        = 4
	iccSampleBytes  = 2
	iccCurveSamples = 1024
	iccVersion      = 0x02100000
	iccClassMonitor = "mntr"
	iccSpaceRGB     = "RGB "
	iccSpaceXYZ     = "XYZ "
	iccSignature    = "acsp"
	iccIntent       = 0
	iccYear         = 2020
	iccMonth        = 1
	iccDay          = 1
	iccMacDescSize  = 67
	iccTagTypeSize  = 4

	srgbDesc        = "sRGB IEC61966-2.1 D50 matrix-shaper (spectrePS)"
	srgbCprt        = "Copyright (c) spectrePS contributors"
	srgbGamma       = 2.4
	srgbCutoff      = 0.04045
	srgbSlope       = 12.92
	srgbOffset      = 0.055
	srgbDivisor     = 1.055
	srgbCurveMax    = 65535
	srgbWhiteX      = 0.9642
	srgbWhiteY      = 1.0
	srgbWhiteZ      = 0.8249
	srgbRedX        = 0.436065674
	srgbRedY        = 0.222488403
	srgbRedZ        = 0.013916016
	srgbGreenX      = 0.385147095
	srgbGreenY      = 0.716873169
	srgbGreenZ      = 0.097076416
	srgbBlueX       = 0.143066406
	srgbBlueY       = 0.060607910
	srgbBlueZ       = 0.714096069
	s15Scale        = 65536
	iccComponentNum = 3
	iccExtraEntries = 2
)

type iccTag struct {
	name string
	data []byte
}

// ICCProfile returns a minimal D50 sRGB matrix-shaper ICC profile.
// It is generated in this package, and two calls return equal bytes.
func ICCProfile() []byte {
	curve := curveTag(srgbSamples())
	return buildICC([]iccTag{
		{name: "bTRC", data: curve},
		{name: "bXYZ", data: xyzTag(srgbBlueX, srgbBlueY, srgbBlueZ)},
		{name: "cprt", data: textTag(srgbCprt)},
		{name: "desc", data: descTag(srgbDesc)},
		{name: "gTRC", data: curve},
		{name: "gXYZ", data: xyzTag(srgbGreenX, srgbGreenY, srgbGreenZ)},
		{name: "rTRC", data: curve},
		{name: "rXYZ", data: xyzTag(srgbRedX, srgbRedY, srgbRedZ)},
		{name: "wtpt", data: xyzTag(srgbWhiteX, srgbWhiteY, srgbWhiteZ)},
	})
}

// buildICC lays out the header, the tag table, and the tag data on four-byte
// boundaries.
func buildICC(tags []iccTag) []byte {
	tableSize := iccTagCountSize + iccTagRowSize*len(tags)
	pos := iccHeaderSize + tableSize
	var data []byte
	offsets := make([]int, len(tags))
	for i, tag := range tags {
		padding := (iccAlign - pos%iccAlign) % iccAlign
		data = append(data, make([]byte, padding)...)
		pos += padding
		offsets[i] = pos
		data = append(data, tag.data...)
		pos += len(tag.data)
	}
	profile := make([]byte, pos)
	writeICCHeader(profile[:iccHeaderSize], pos)
	writeICCTable(profile[iccHeaderSize:iccHeaderSize+tableSize], tags, offsets)
	copy(profile[iccHeaderSize+tableSize:], data)
	return profile
}

func writeICCHeader(dst []byte, size int) {
	binary.BigEndian.PutUint32(dst[0:], sizeBits(size))
	binary.BigEndian.PutUint32(dst[8:], iccVersion)
	copy(dst[12:16], iccClassMonitor)
	copy(dst[16:20], iccSpaceRGB)
	copy(dst[20:24], iccSpaceXYZ)
	binary.BigEndian.PutUint16(dst[24:], iccYear)
	binary.BigEndian.PutUint16(dst[26:], iccMonth)
	binary.BigEndian.PutUint16(dst[28:], iccDay)
	copy(dst[36:40], iccSignature)
	binary.BigEndian.PutUint32(dst[64:], iccIntent)
	putS15(dst[68:], srgbWhiteX)
	putS15(dst[72:], srgbWhiteY)
	putS15(dst[76:], srgbWhiteZ)
}

func writeICCTable(dst []byte, tags []iccTag, offsets []int) {
	binary.BigEndian.PutUint32(dst[0:], sizeBits(len(tags)))
	for i, tag := range tags {
		row := dst[iccTagCountSize+i*iccTagRowSize:]
		copy(row[0:4], tag.name)
		binary.BigEndian.PutUint32(row[4:], sizeBits(offsets[i]))
		binary.BigEndian.PutUint32(row[8:], sizeBits(len(tag.data)))
	}
}

// sizeBits converts an offset or a length to its four-byte field. Profile
// sizes and offsets are far below 4 GiB, which gosec cannot see here.
func sizeBits(value int) uint32 {
	return uint32(value) //nolint:gosec // profile sizes and offsets are far below 4 GiB
}

// srgbSamples encodes the sRGB transfer function at 1024 points. 65535 is 1.0.
func srgbSamples() []byte {
	samples := make([]byte, iccSampleBytes*iccCurveSamples)
	for i := range iccCurveSamples {
		value := float64(i) / float64(iccCurveSamples-1)
		sample := uint16(math.Round(srgbToLinear(value) * srgbCurveMax))
		binary.BigEndian.PutUint16(samples[iccSampleBytes*i:], sample)
	}
	return samples
}

// srgbToLinear maps an sRGB value in 0 through 1 to a linear value.
func srgbToLinear(value float64) float64 {
	if value <= srgbCutoff {
		return value / srgbSlope
	}
	return math.Pow((value+srgbOffset)/srgbDivisor, srgbGamma)
}

// curveTag writes a sampled curveType tag. No samples would be an identity
// curve, which this profile never uses.
func curveTag(samples []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("curv")
	buf.Write(make([]byte, iccTagTypeSize))
	writeUint32(&buf, sizeBits(len(samples)/iccSampleBytes))
	buf.Write(samples)
	return buf.Bytes()
}

// xyzTag writes one XYZType tag.
func xyzTag(x, y, z float64) []byte {
	var buf bytes.Buffer
	buf.WriteString("XYZ ")
	buf.Write(make([]byte, iccTagTypeSize))
	for _, value := range []float64{x, y, z} {
		writeS15(&buf, value)
	}
	return buf.Bytes()
}

// textTag writes one textType tag.
func textTag(text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("text")
	buf.Write(make([]byte, iccTagTypeSize))
	buf.WriteString(text)
	buf.WriteByte(0)
	return buf.Bytes()
}

// descTag writes one textDescriptionType tag. The Unicode, script, and
// Macintosh sections are present and empty, as the type requires.
func descTag(text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("desc")
	buf.Write(make([]byte, iccTagTypeSize))
	writeUint32(&buf, sizeBits(len(text)+1))
	buf.WriteString(text)
	buf.WriteByte(0)
	writeUint32(&buf, 0)
	writeUint32(&buf, 0)
	writeUint16(&buf, 0)
	buf.WriteByte(0)
	buf.Write(make([]byte, iccMacDescSize))
	return buf.Bytes()
}

func writeUint32(buf *bytes.Buffer, value uint32) {
	var raw [iccWordSize]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buf.Write(raw[:])
}

func writeUint16(buf *bytes.Buffer, value uint16) {
	var raw [iccSampleBytes]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buf.Write(raw[:])
}

func writeS15(buf *bytes.Buffer, value float64) {
	var raw [iccWordSize]byte
	binary.BigEndian.PutUint32(raw[:], s15Bits(value))
	buf.Write(raw[:])
}

func putS15(dst []byte, value float64) {
	binary.BigEndian.PutUint32(dst, s15Bits(value))
}

// s15Bits returns the two's-complement bit pattern of an s15Fixed16 value.
func s15Bits(value float64) uint32 {
	//nolint:gosec // the signed fixed-point value is written as its bit pattern
	return uint32(s15Fixed16(value))
}

func s15Fixed16(value float64) int32 {
	return int32(math.Round(value * s15Scale))
}
