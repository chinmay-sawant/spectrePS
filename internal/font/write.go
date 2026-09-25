package font

import (
	"encoding/binary"
	"errors"
	"sort"
)

// sfnt scaler types the table reader accepts. A collection header, "ttcf", is
// not one of them.
const (
	scalerTrueType = 0x00010000
	scalerTrue     = 0x74727565 // "true"
	scalerOTTO     = 0x4F54544F // "OTTO"
	headAdjust     = 0xB1B0AFBA
	headTag        = "head"
	sfntHeaderLen  = 12
	sfntEntryLen   = 16
	sfntTagLen     = 4
	headCheckField = 8
	headLocField   = 50
	maxTableCount  = 4096
	padMask        = 3
	magicPad       = 3
	wordLen        = 4
)

var (
	// errSFNTScaler reports an sfnt header Spectre does not read.
	errSFNTScaler = errors.New("font: unsupported sfnt scaler type")
	// errSFNTSyntax reports a truncated or inconsistent sfnt program.
	errSFNTSyntax = errors.New("font: malformed sfnt program")
)

// Table is one sfnt table: its four-byte tag and its raw data. Data aliases
// the program passed to ParseTables.
type Table struct {
	Tag  string
	Data []byte
}

// ParseTables reads the table directory of a TrueType, Mac TrueType, or
// OpenType program. The tables come back in directory order. A collection
// header, a bad scaler, a duplicate tag, or a directory that runs past the
// program returns an error.
func ParseTables(program []byte) ([]Table, error) {
	if len(program) < sfntHeaderLen || !scalerOK(program) {
		return nil, errSFNTScaler
	}
	count := int(binary.BigEndian.Uint16(program[4:]))
	if count < 1 || count > maxTableCount || sfntHeaderLen+sfntEntryLen*count > len(program) {
		return nil, errSFNTSyntax
	}
	return readDirectory(program, count)
}

// scalerOK reports whether the first four bytes are an sfnt scaler type this
// reader knows.
func scalerOK(program []byte) bool {
	scaler := binary.BigEndian.Uint32(program)
	return scaler == scalerTrueType || scaler == scalerTrue || scaler == scalerOTTO
}

// readDirectory reads count table records. A duplicate tag or a slice that
// runs past the program is an error.
func readDirectory(program []byte, count int) ([]Table, error) {
	tables := make([]Table, 0, count)
	seen := make(map[string]bool, count)
	for idx := range count {
		entry := program[sfntHeaderLen+sfntEntryLen*idx:]
		tag := string(entry[0:sfntTagLen])
		if seen[tag] {
			return nil, errSFNTSyntax
		}
		seen[tag] = true
		offset := int(binary.BigEndian.Uint32(entry[8:]))
		length := int(binary.BigEndian.Uint32(entry[12:]))
		if offset < 0 || length < 0 || offset+length > len(program) {
			return nil, errSFNTSyntax
		}
		tables = append(tables, Table{Tag: tag, Data: program[offset : offset+length]})
	}
	return tables, nil
}

// WriteTables writes an sfnt program from tables: the directory sorts by tag,
// every table starts on a four-byte boundary, table checksums are recomputed,
// and the head checkSumAdjustment is set last. The input tables are copied, so
// the caller can keep its own slices.
func WriteTables(tables []Table) ([]byte, error) {
	sorted, err := sortTables(tables)
	if err != nil {
		return nil, err
	}
	offsets, total := tableLayout(sorted)
	out := make([]byte, total)
	writeDirectory(out, sorted, offsets)
	writeBodies(out, sorted, offsets)
	adjustHead(out, sorted, offsets)
	return out, nil
}

// sortTables copies the tables and sorts them by tag. A bad count, a tag that
// is not four bytes, and a duplicate tag are errors.
func sortTables(tables []Table) ([]Table, error) {
	if len(tables) < 1 || len(tables) > maxTableCount {
		return nil, errSFNTSyntax
	}
	sorted := make([]Table, len(tables))
	copy(sorted, tables)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tag < sorted[j].Tag })
	for idx := range sorted {
		if len(sorted[idx].Tag) != sfntTagLen {
			return nil, errSFNTSyntax
		}
		if idx > 0 && sorted[idx].Tag == sorted[idx-1].Tag {
			return nil, errSFNTSyntax
		}
	}
	return sorted, nil
}

// tableLayout returns each table's file offset and the total program size.
func tableLayout(tables []Table) ([]int, int) {
	offsets := make([]int, len(tables))
	total := sfntHeaderLen + sfntEntryLen*len(tables)
	for idx, table := range tables {
		offsets[idx] = total
		total += paddedLen(len(table.Data))
	}
	return offsets, total
}

// writeDirectory writes the header and the table records. The head checksum
// is computed with its checkSumAdjustment field zeroed.
func writeDirectory(out []byte, tables []Table, offsets []int) {
	searchRange, entrySelector := searchRangeOf(len(tables))
	rangeShift := sfntEntryLen*len(tables) - searchRange
	binary.BigEndian.PutUint32(out, scalerTrueType)
	binary.BigEndian.PutUint16(out[4:], uint16(len(tables)))   //nolint:gosec // count is bounded by maxTableCount
	binary.BigEndian.PutUint16(out[6:], uint16(searchRange))   //nolint:gosec // the range is a header field
	binary.BigEndian.PutUint16(out[8:], uint16(entrySelector)) //nolint:gosec // the selector is a header field
	binary.BigEndian.PutUint16(out[10:], uint16(rangeShift))   //nolint:gosec // the shift is a header field
	for idx, table := range tables {
		entry := out[sfntHeaderLen+sfntEntryLen*idx:]
		copy(entry[0:sfntTagLen], table.Tag)
		binary.BigEndian.PutUint32(entry[4:], Checksum(checksumData(table)))
		binary.BigEndian.PutUint32(entry[8:], uint32(offsets[idx]))     //nolint:gosec // offsets fit the 4 GiB program cap
		binary.BigEndian.PutUint32(entry[12:], uint32(len(table.Data))) //nolint:gosec // lengths fit the 4 GiB program cap
	}
}

// checksumData returns the bytes a table checksum covers. The head table
// checksums with checkSumAdjustment zeroed.
func checksumData(table Table) []byte {
	if table.Tag != headTag || len(table.Data) < headCheckField+4 {
		return table.Data
	}
	clone := make([]byte, len(table.Data))
	copy(clone, table.Data)
	binary.BigEndian.PutUint32(clone[headCheckField:], 0)
	return clone
}

// writeBodies copies each table into its offset.
func writeBodies(out []byte, tables []Table, offsets []int) {
	for idx, table := range tables {
		copy(out[offsets[idx]:], table.Data)
	}
}

// adjustHead zeroes the head checkSumAdjustment in the written program and
// sets the value that completes the whole-font checksum to 0xB1B0AFBA.
func adjustHead(out []byte, tables []Table, offsets []int) {
	for idx, table := range tables {
		if table.Tag == headTag && len(table.Data) >= headCheckField+4 {
			binary.BigEndian.PutUint32(out[offsets[idx]+headCheckField:], 0)
		}
	}
	for idx, table := range tables {
		if table.Tag == headTag && len(table.Data) >= headCheckField+4 {
			adjust := headAdjust - Checksum(out)
			binary.BigEndian.PutUint32(out[offsets[idx]+headCheckField:], adjust)
		}
	}
}

// TableData returns the data of one table by tag.
func TableData(tables []Table, tag string) ([]byte, bool) {
	for _, table := range tables {
		if table.Tag == tag {
			return table.Data, true
		}
	}
	return nil, false
}

// putTable replaces or appends one table and returns the new slice. The input
// tables are not changed.
func putTable(tables []Table, tag string, data []byte) []Table {
	out := make([]Table, 0, len(tables)+1)
	done := false
	for _, table := range tables {
		if table.Tag == tag {
			out = append(out, Table{Tag: tag, Data: data})
			done = true
			continue
		}
		out = append(out, table)
	}
	if !done {
		out = append(out, Table{Tag: tag, Data: data})
	}
	return out
}

// searchRangeOf returns the sfnt binary-search header fields for one count.
func searchRangeOf(count int) (int, int) {
	searchRange := sfntEntryLen
	entrySelector := 0
	for searchRange*2 <= sfntEntryLen*count {
		searchRange *= 2
		entrySelector++
	}
	return searchRange, entrySelector
}

func paddedLen(size int) int {
	return (size + magicPad) &^ padMask
}

// Checksum returns the sfnt checksum of data: the sum of its big-endian
// 32-bit words, with a short final word zero-padded.
func Checksum(data []byte) uint32 {
	var sum uint32
	for idx := 0; idx < len(data); idx += wordLen {
		var word [wordLen]byte
		copy(word[:], data[idx:min(idx+wordLen, len(data))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}
