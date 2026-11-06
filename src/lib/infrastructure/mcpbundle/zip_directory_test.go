package mcpbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"testing"
)

func directoryFixture(t *testing.T, count int) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for index := 0; index < count; index++ {
		if _, err := writer.Create(fmt.Sprintf("entry-%d", index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func zip64Fixture(t *testing.T, original []byte) []byte {
	t.Helper()
	endOffset := len(original) - zipDirectoryEndSize
	originalEnd := original[endOffset:]
	record := make([]byte, zip64DirectoryEndSize)
	binary.LittleEndian.PutUint32(record, 0x06064b50)
	binary.LittleEndian.PutUint64(record[4:12], 44)
	binary.LittleEndian.PutUint16(record[12:14], 45)
	binary.LittleEndian.PutUint16(record[14:16], 45)
	count := uint64(binary.LittleEndian.Uint16(originalEnd[10:12]))
	binary.LittleEndian.PutUint64(record[24:32], count)
	binary.LittleEndian.PutUint64(record[32:40], count)
	binary.LittleEndian.PutUint64(record[40:48], uint64(binary.LittleEndian.Uint32(originalEnd[12:16])))
	binary.LittleEndian.PutUint64(record[48:56], uint64(binary.LittleEndian.Uint32(originalEnd[16:20])))
	locator := make([]byte, zip64LocatorSize)
	binary.LittleEndian.PutUint32(locator, 0x07064b50)
	binary.LittleEndian.PutUint64(locator[8:16], uint64(endOffset))
	binary.LittleEndian.PutUint32(locator[16:20], 1)
	end := append([]byte(nil), originalEnd...)
	binary.LittleEndian.PutUint16(end[8:10], 0xffff)
	binary.LittleEndian.PutUint16(end[10:12], 0xffff)
	binary.LittleEndian.PutUint32(end[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(end[16:20], 0xffffffff)
	result := append([]byte(nil), original[:endOffset]...)
	result = append(result, record...)
	result = append(result, locator...)
	return append(result, end...)
}

func TestZIPPreflightCountsActualRecordsBeforeStdlibAllocation(t *testing.T) {
	for _, useZIP64 := range []bool{false, true} {
		archive := directoryFixture(t, 3)
		if useZIP64 {
			archive = zip64Fixture(t, archive)
		}
		if _, err := preflightZIP(context.Background(), bytes.NewReader(archive), int64(len(archive)), 3); err != nil {
			t.Fatalf("valid ZIP64=%v rejected: %v", useZIP64, err)
		}
		parsed, err := openBoundedZIP(context.Background(), bytes.NewReader(archive), int64(len(archive)))
		if err != nil || len(parsed.File) != 3 {
			t.Fatalf("stdlib ZIP64=%v parse failed: %v", useZIP64, err)
		}
		// Falsify the declared count so the actual central-directory walk, not
		// merely an end-record count check, must enforce the smaller budget.
		if useZIP64 {
			record := len(archive) - zipDirectoryEndSize - zip64LocatorSize - zip64DirectoryEndSize
			binary.LittleEndian.PutUint64(archive[record+24:record+32], 2)
			binary.LittleEndian.PutUint64(archive[record+32:record+40], 2)
		} else {
			end := archive[len(archive)-zipDirectoryEndSize:]
			binary.LittleEndian.PutUint16(end[8:10], 2)
			binary.LittleEndian.PutUint16(end[10:12], 2)
		}
		if _, err := preflightZIP(context.Background(), bytes.NewReader(archive), int64(len(archive)), 2); err == nil {
			t.Fatalf("actual member count bypassed budget, ZIP64=%v", useZIP64)
		}
	}
}

func TestZIPPreflightRejectsOversizedDeclarationsAndCancellation(t *testing.T) {
	archive := zip64Fixture(t, directoryFixture(t, 1))
	record := len(archive) - zipDirectoryEndSize - zip64LocatorSize - zip64DirectoryEndSize
	binary.LittleEndian.PutUint64(archive[record+24:record+32], 1<<40)
	binary.LittleEndian.PutUint64(archive[record+32:record+40], 1<<40)
	if _, err := openBoundedZIP(context.Background(), bytes.NewReader(archive), int64(len(archive))); err == nil {
		t.Fatal("oversized ZIP64 declaration reached stdlib allocation")
	}
	archive = directoryFixture(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openBoundedZIP(ctx, bytes.NewReader(archive), int64(len(archive))); err == nil {
		t.Fatal("cancelled ZIP preflight succeeded")
	}
}

func TestZIPPreflightFreezesValidatedMetadata(t *testing.T) {
	for _, useZIP64 := range []bool{false, true} {
		archive := directoryFixture(t, 1)
		if useZIP64 {
			archive = zip64Fixture(t, archive)
		}
		source := bytes.NewReader(archive)
		frozen, err := preflightZIP(context.Background(), source, int64(len(archive)), 1)
		if err != nil {
			t.Fatal(err)
		}
		// A source file may change between preflight and zip.NewReader. Its
		// previously validated metadata remains the only parser input.
		for index := int(frozen.start); index < len(archive); index++ {
			archive[index] = 0xff
		}
		parsed, err := zip.NewReader(frozen, int64(len(archive)))
		if err != nil || len(parsed.File) != 1 || parsed.File[0].Name != "entry-0" {
			t.Fatalf("metadata mutation escaped snapshot: %v", err)
		}
	}
}

func TestZIPPreflightRejectsAmbiguousEndRecordInComment(t *testing.T) {
	archive := directoryFixture(t, 1)
	end := archive[len(archive)-zipDirectoryEndSize:]
	binary.LittleEndian.PutUint16(end[20:22], zipDirectoryEndSize+1)
	fake := make([]byte, zipDirectoryEndSize+1)
	binary.LittleEndian.PutUint32(fake, 0x06054b50)
	archive = append(archive, fake...)
	if _, err := openBoundedZIP(context.Background(), bytes.NewReader(archive), int64(len(archive))); err == nil {
		t.Fatal("preflight and stdlib could select different end records")
	}
}

func TestZIPPreflightRejectsDifferentClassicAndZIP64Directories(t *testing.T) {
	original := directoryFixture(t, 2)
	selected := zip64Fixture(t, directoryFixture(t, 1))
	layout, err := readZIPDirectoryLayout(bytes.NewReader(selected), int64(len(selected)))
	if err != nil {
		t.Fatal(err)
	}
	central := append([]byte(nil), selected[layout.offset:layout.zip64Offset]...)
	endBlock := append([]byte(nil), selected[layout.zip64Offset:]...)
	zip64Offset := len(original) + len(central)
	binary.LittleEndian.PutUint64(endBlock[48:56], uint64(len(original)))
	binary.LittleEndian.PutUint64(endBlock[zip64DirectoryEndSize+8:zip64DirectoryEndSize+16], uint64(zip64Offset))
	// Classic EOCD deliberately has no ZIP64 sentinel and selects the original
	// two-entry directory. The adjacent ZIP64 record selects a different single
	// entry. Preflight must not choose ZIP64 while archive/zip chooses classic.
	copy(endBlock[len(endBlock)-zipDirectoryEndSize:], original[len(original)-zipDirectoryEndSize:])
	forged := append(append(append([]byte(nil), original...), central...), endBlock...)
	parsed, err := zip.NewReader(bytes.NewReader(forged), int64(len(forged)))
	if err != nil || len(parsed.File) != 2 {
		t.Fatalf("fixture must exercise classic stdlib selection: %v", err)
	}
	if _, err := preflightZIP(context.Background(), bytes.NewReader(forged), int64(len(forged)), 1); err == nil {
		t.Fatal("ZIP64 preflight allowed a different unvalidated classic directory")
	}
}
