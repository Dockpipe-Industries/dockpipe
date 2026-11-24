package mcpbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const zipDirectoryHeaderSize = 46
const zipDirectoryEndSize = 22
const zip64DirectoryEndSize = 56
const zip64LocatorSize = 20
const zipMaximumCommentSize = 65535

// frozenZIPMetadata keeps archive/zip's unbounded directory parser behind the
// validated member budget. Its metadata snapshot cannot change between the
// preflight and stdlib parsing. Storage is bounded by the 512 MiB archive budget.
type frozenZIPMetadata struct {
	context  context.Context
	source   io.ReaderAt
	start    int64
	metadata []byte
}

func (reader *frozenZIPMetadata) ReadAt(output []byte, offset int64) (int, error) {
	if err := reader.context.Err(); err != nil {
		return 0, err
	}
	if offset < 0 {
		return 0, fmt.Errorf("invalid ZIP offset")
	}
	written := 0
	if offset < reader.start {
		prefix := min(int64(len(output)), reader.start-offset)
		size, err := reader.source.ReadAt(output[:prefix], offset)
		written += size
		offset += int64(size)
		output = output[size:]
		if err != nil {
			return written, err
		}
	}
	if len(output) == 0 {
		return written, nil
	}
	position := offset - reader.start
	if position < 0 || position >= int64(len(reader.metadata)) {
		return written, io.EOF
	}
	copied := copy(output, reader.metadata[position:])
	written += copied
	if copied != len(output) {
		return written, io.EOF
	}
	return written, nil
}

type zipDirectoryLayout struct {
	offset      int64
	size        int64
	records     uint64
	zip64       bool
	endOffset   int64
	endBytes    []byte
	zip64Offset int64
	zip64Bytes  []byte
}

func openBoundedZIP(ctx context.Context, source io.ReaderAt, size int64) (*zip.Reader, error) {
	metadata, err := preflightZIP(ctx, source, size, maxMembers)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(metadata, size)
}

func preflightZIP(ctx context.Context, source io.ReaderAt, size int64, memberLimit int) (*frozenZIPMetadata, error) {
	if size < zipDirectoryEndSize || size > MaxArchiveBytes || memberLimit <= 0 {
		return nil, fmt.Errorf("MCPB ZIP exceeds archive bounds")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	layout, err := readZIPDirectoryLayout(source, size)
	if err != nil {
		return nil, err
	}
	if layout.records > uint64(memberLimit) || layout.offset < 0 || layout.size < 0 ||
		layout.offset > size || layout.size > size-layout.offset {
		return nil, fmt.Errorf("MCPB ZIP exceeds member or directory bounds")
	}
	metadata := make([]byte, int(size-layout.offset))
	reader := &contextZIPReader{context: ctx, reader: io.NewSectionReader(source, layout.offset, size-layout.offset)}
	if _, err := io.ReadFull(reader, metadata); err != nil {
		return nil, err
	}
	if !bytes.Equal(metadata[layout.endOffset-layout.offset:], layout.endBytes) {
		return nil, fmt.Errorf("MCPB ZIP end metadata changed while reading")
	}
	if layout.zip64 && !bytes.Equal(metadata[layout.zip64Offset-layout.offset:layout.zip64Offset-layout.offset+zip64DirectoryEndSize], layout.zip64Bytes) {
		return nil, fmt.Errorf("MCPB ZIP64 metadata changed while reading")
	}
	directory := metadata[:layout.size]
	records := 0
	for offset := 0; offset < len(directory); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := directory[offset:]
		// A ZIP digital signature may follow the final central-directory entry.
		if len(remaining) >= 6 && binary.LittleEndian.Uint32(remaining) == 0x05054b50 {
			length := 6 + int(binary.LittleEndian.Uint16(remaining[4:6]))
			if length != len(remaining) {
				return nil, fmt.Errorf("MCPB ZIP directory signature is invalid")
			}
			offset += length
			continue
		}
		if len(remaining) < zipDirectoryHeaderSize || binary.LittleEndian.Uint32(remaining) != 0x02014b50 {
			return nil, fmt.Errorf("MCPB ZIP central directory is invalid")
		}
		records++
		if records > memberLimit {
			return nil, fmt.Errorf("MCPB exceeds member budget")
		}
		length := zipDirectoryHeaderSize + int(binary.LittleEndian.Uint16(remaining[28:30])) +
			int(binary.LittleEndian.Uint16(remaining[30:32])) + int(binary.LittleEndian.Uint16(remaining[32:34]))
		if length > len(remaining) {
			return nil, fmt.Errorf("MCPB ZIP central directory is truncated")
		}
		offset += length
	}
	if (layout.zip64 && uint64(records) != layout.records) || (!layout.zip64 && uint64(records)&0xffff != layout.records) {
		return nil, fmt.Errorf("MCPB ZIP member count is inconsistent")
	}
	return &frozenZIPMetadata{context: ctx, source: source, start: layout.offset, metadata: metadata}, nil
}

type contextZIPReader struct {
	context context.Context
	reader  io.Reader
}

func (reader *contextZIPReader) Read(output []byte) (int, error) {
	if err := reader.context.Err(); err != nil {
		return 0, err
	}
	// Check cancellation between fixed-size local-file reads even when the
	// central directory occupies most of the maximum-sized archive.
	return reader.reader.Read(output[:min(len(output), 64<<10)])
}

func readZIPDirectoryLayout(source io.ReaderAt, size int64) (zipDirectoryLayout, error) {
	var result zipDirectoryLayout
	tailSize := min(size, int64(zipDirectoryEndSize+zipMaximumCommentSize))
	tail := make([]byte, int(tailSize))
	if _, err := source.ReadAt(tail, size-tailSize); err != nil {
		return result, err
	}
	position := -1
	for index := len(tail) - zipDirectoryEndSize; index >= 0; index-- {
		if binary.LittleEndian.Uint32(tail[index:]) == 0x06054b50 {
			commentSize := int(binary.LittleEndian.Uint16(tail[index+20 : index+22]))
			// archive/zip selects the last signature, including one embedded
			// in a comment. Reject trailing or truncated candidates rather than
			// letting preflight and the stdlib choose different end records.
			if index+zipDirectoryEndSize+commentSize != len(tail) {
				return result, fmt.Errorf("MCPB ZIP end record has inconsistent comment bounds")
			}
			position = index
			break
		}
	}
	if position < 0 {
		return result, fmt.Errorf("MCPB ZIP end record is missing")
	}
	end := tail[position:]
	result.endOffset = size - tailSize + int64(position)
	result.endBytes = append([]byte(nil), end...)
	if binary.LittleEndian.Uint16(end[4:6]) != 0 || binary.LittleEndian.Uint16(end[6:8]) != 0 ||
		binary.LittleEndian.Uint16(end[8:10]) != binary.LittleEndian.Uint16(end[10:12]) {
		return result, fmt.Errorf("multi-disk MCPB ZIP archives are unsupported")
	}
	result.records = uint64(binary.LittleEndian.Uint16(end[10:12]))
	result.size = int64(binary.LittleEndian.Uint32(end[12:16]))
	result.offset = int64(binary.LittleEndian.Uint32(end[16:20]))
	directoryEnd := result.endOffset
	needsZIP64 := result.records == 0xffff || result.size == 0xffffffff || result.offset == 0xffffffff
	locator := make([]byte, zip64LocatorSize)
	hasLocator := result.endOffset >= zip64LocatorSize
	if hasLocator {
		if _, err := source.ReadAt(locator, result.endOffset-zip64LocatorSize); err != nil {
			return result, err
		}
		hasLocator = binary.LittleEndian.Uint32(locator) == 0x07064b50
	}
	if needsZIP64 && !hasLocator {
		// Non-ZIP64 archives may use the low 16 bits of their member count.
		if result.size == 0xffffffff || result.offset == 0xffffffff {
			return result, fmt.Errorf("MCPB ZIP64 locator is missing")
		}
	}
	// Match archive/zip's ZIP64 selection, including its historical 0xffff
	// directory-size trigger. Otherwise a dual-directory archive could pass
	// ZIP64 preflight while the stdlib selects an unvalidated classic region.
	stdlibZIP64 := result.records == 0xffff || result.size == 0xffff || result.offset == 0xffffffff
	if hasLocator && !stdlibZIP64 {
		return result, fmt.Errorf("MCPB ZIP64 locator disagrees with classic end record")
	}
	if hasLocator {
		if binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
			return result, fmt.Errorf("multi-disk MCPB ZIP64 archives are unsupported")
		}
		offset, err := checkedZIPOffset(binary.LittleEndian.Uint64(locator[8:16]))
		if err != nil {
			return result, err
		}
		locatorOffset := result.endOffset - zip64LocatorSize
		if offset > locatorOffset || locatorOffset-offset < zip64DirectoryEndSize {
			return result, fmt.Errorf("MCPB ZIP64 directory offset is invalid")
		}
		record := make([]byte, zip64DirectoryEndSize)
		if _, err := source.ReadAt(record, offset); err != nil {
			return result, err
		}
		recordSize, err := checkedZIPOffset(binary.LittleEndian.Uint64(record[4:12]))
		if err != nil {
			return result, err
		}
		if binary.LittleEndian.Uint32(record) != 0x06064b50 || recordSize < 44 ||
			recordSize != locatorOffset-offset-12 ||
			binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 ||
			binary.LittleEndian.Uint64(record[24:32]) != binary.LittleEndian.Uint64(record[32:40]) {
			return result, fmt.Errorf("MCPB ZIP64 end record is invalid")
		}
		directorySize, err := checkedZIPOffset(binary.LittleEndian.Uint64(record[40:48]))
		if err != nil {
			return result, err
		}
		directoryOffset, err := checkedZIPOffset(binary.LittleEndian.Uint64(record[48:56]))
		if err != nil {
			return result, err
		}
		if directorySize > size || directoryOffset > size {
			return result, fmt.Errorf("MCPB ZIP64 directory exceeds archive bounds")
		}
		result.zip64 = true
		result.zip64Offset = offset
		result.zip64Bytes = record
		result.endBytes = append(append([]byte(nil), locator...), result.endBytes...)
		result.endOffset -= zip64LocatorSize
		result.records = binary.LittleEndian.Uint64(record[32:40])
		result.size = directorySize
		result.offset = directoryOffset
		directoryEnd = offset
	}
	// Require consistent offsets, including any declared prefix. Permissive
	// stdlib corrections for malformed offsets must not select another region.
	if result.offset < 0 || result.offset > directoryEnd || result.size != directoryEnd-result.offset {
		return result, fmt.Errorf("MCPB ZIP central directory offsets are inconsistent")
	}
	return result, nil
}

func checkedZIPOffset(value uint64) (int64, error) {
	if value > math.MaxInt64 || value > uint64(MaxArchiveBytes) {
		return 0, fmt.Errorf("MCPB ZIP64 offset exceeds archive bounds")
	}
	return int64(value), nil
}
