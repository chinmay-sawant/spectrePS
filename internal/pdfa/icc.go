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
	iccVersion      = 0x02100000
	iccClassMonitor = "mntr"
	iccSpaceRGB     = "RGB "
	iccSpaceXYZ     = "XYZ "
	iccSignature    = "acsp"
	iccIntent       = 0
	iccCurveSamples = 1024
	iccYear         = 2020
	iccMonth        = 1
	iccDay          = 1

	srgbDesc  = "sRGB IEC61966-2.1 D50 matrix-shaper (spectrePS)"
	srgbCprt  = "Copyright (c) spectrePS contributors"
	srgbWhite = 0.9642
	srgbGamma = 2.4
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
		{name: "bXYZ", data: xyzTag(0.143066406, 0.060607910, 0.714096069)},
		{name: "cprt", data: textTag(srgbCprt)},
		{name: "desc", data: descTag(srgbDesc)},
		{name: "gTRC", data: curve},
		{name: "gXYZ", data: xyzTag(0.385147095, 0.716873169, 0.097076416)},
		{name: "rTRC", data: curve},
		{name: "rXYZ", data: xyzTag(0.436065674, 0.222488403, 0.013916016)},
		{name: "wtpt", data: xyzTag(srgbWhite, 1.0, 0.8249)},
	})
}

// buildICC lays out the header, the tag table, and the tag data on four-byte
// boundaries.
func buildICC(tags []iccTag) []byte {
	tableSize := 4 + iccTagRowSize*len(tags)
	pos := iccHeaderSize + tableSize
	var data []byte
	offsets := make([]int, len(tags))
	for i, tag := range tags {
		padding := (4 - pos%4) % 4
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
	binary.BigEndian.PutUint32(dst[0:], uint32(size))
	binary.BigEndian.PutUint32(dst[8:], iccVersion)
	copy(dst[12:16], iccClassMonitor)
	copy(dst[16:20], iccSpaceRGB)
	copy(dst[20:24], iccSpaceXYZ)
	binary.BigEndian.PutUint16(dst[24:], iccYear)
	binary.BigEndian.PutUint16(dst[26:], iccMonth)
	binary.BigEndian.PutUint16(dst[28:], iccDay)
	copy(dst[36:40], iccSignature)
	binary.BigEndian.PutUint32(dst[64:], iccIntent)
	putS15(dst[68:], srgbWhite)
	putS15(dst[72:], 1.0)
	putS15(dst[76:], 0.8249)
}

func writeICCTable(dst []byte, tags []iccTag, offsets []int) {
	binary.BigEndian.PutUint32(dst[0:], uint32(len(tags)))
	for i, tag := range tags {
		row := dst[4+i*iccTagRowSize:]
		copy(row[0:4], tag.name)
		binary.BigEndian.PutUint32(row[4:], uint32(offsets[i]))
		binary.BigEndian.PutUint32(row[8:], uint32(len(tag.data)))
	}
}

// srgbSamples encodes the sRGB transfer function at 1024 points. 65535 is 1.0.
func srgbSamples() []byte {
	samples := make([]byte, 2*iccCurveSamples)
	for i := range iccCurveSamples {
		value := float64(i) / float64(iccCurveSamples-1)
		sample := uint16(math.Round(srgbToLinear(value) * 65535))
		binary.BigEndian.PutUint16(samples[2*i:], sample)
	}
	return samples
}

// srgbToLinear maps an sRGB value in 0 through 1 to a linear value.
func srgbToLinear(value float64) float64 {
	if value <= 0.04045 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, srgbGamma)
}

// curveTag writes a sampled curveType tag. No samples would be an identity
// curve, which this profile never uses.
func curveTag(samples []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("curv")
	buf.Write(make([]byte, 4))
	writeUint32(&buf, uint32(len(samples)/2))
	buf.Write(samples)
	return buf.Bytes()
}

// xyzTag writes one XYZType tag.
func xyzTag(x, y, z float64) []byte {
	var buf bytes.Buffer
	buf.WriteString("XYZ ")
	buf.Write(make([]byte, 4))
	for _, value := range []float64{x, y, z} {
		writeS15(&buf, value)
	}
	return buf.Bytes()
}

// textTag writes one textType tag.
func textTag(text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("text")
	buf.Write(make([]byte, 4))
	buf.WriteString(text)
	buf.WriteByte(0)
	return buf.Bytes()
}

// descTag writes one textDescriptionType tag. The Unicode, script, and
// Macintosh sections are present and empty, as the type requires.
func descTag(text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("desc")
	buf.Write(make([]byte, 4))
	writeUint32(&buf, uint32(len(text)+1))
	buf.WriteString(text)
	buf.WriteByte(0)
	writeUint32(&buf, 0)
	writeUint32(&buf, 0)
	writeUint16(&buf, 0)
	buf.WriteByte(0)
	buf.Write(make([]byte, 67))
	return buf.Bytes()
}

func writeUint32(buf *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buf.Write(raw[:])
}

func writeUint16(buf *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buf.Write(raw[:])
}

func writeS15(buf *bytes.Buffer, value float64) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], uint32(s15Fixed16(value)))
	buf.Write(raw[:])
}

func putS15(dst []byte, value float64) {
	binary.BigEndian.PutUint32(dst, uint32(s15Fixed16(value)))
}

func s15Fixed16(value float64) int32 {
	return int32(math.Round(value * 65536))
}
