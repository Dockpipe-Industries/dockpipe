package executor

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const isoSector = 2048

// buildNoCloudISO creates the deliberately tiny deterministic ISO-9660 image
// used by the sealed dockpipe-go-iso9660-v1 builder. Linux's normal ISO name
// mapping exposes the three level-2 identifiers as the required lowercase
// NoCloud filenames.
func buildNoCloudISO(request NoCloudSeedRequest) ([]byte, error) {
	if request.Label != "cidata" || len(request.Files) != 3 {
		return nil, fmt.Errorf("invalid deterministic NoCloud input")
	}
	names := []string{"META-DATA;1", "NETWORK-CONFIG;1", "USER-DATA;1"}
	fileStart := uint32(21)
	extents := make([]uint32, 3)
	sectors := uint32(0)
	for i, file := range request.Files {
		extents[i] = fileStart + sectors
		sectors += uint32((len(file.Content) + isoSector - 1) / isoSector)
	}
	total := fileStart + sectors
	image := make([]byte, int(total)*isoSector)
	pvd := image[16*isoSector : 17*isoSector]
	pvd[0] = 1
	copy(pvd[1:6], "CD001")
	pvd[6] = 1
	spacePad(pvd[8:40], "DOCKPIPE")
	spacePad(pvd[40:72], strings.ToUpper(request.Label))
	putBoth32(pvd[80:88], total)
	putBoth16(pvd[120:124], 1)
	putBoth16(pvd[124:128], 1)
	putBoth16(pvd[128:132], isoSector)
	putBoth32(pvd[132:140], 10)
	binary.LittleEndian.PutUint32(pvd[140:144], 18)
	binary.BigEndian.PutUint32(pvd[148:152], 19)
	rootRecord := directoryRecord(20, isoSector, 2, []byte{0})
	copy(pvd[156:], rootRecord)
	pvd[881] = 1
	term := image[17*isoSector : 18*isoSector]
	term[0] = 255
	copy(term[1:6], "CD001")
	term[6] = 1
	lpath := image[18*isoSector : 19*isoSector]
	lpath[0] = 1
	binary.LittleEndian.PutUint32(lpath[2:6], 20)
	binary.LittleEndian.PutUint16(lpath[6:8], 1)
	lpath[8] = 0
	mpath := image[19*isoSector : 20*isoSector]
	mpath[0] = 1
	binary.BigEndian.PutUint32(mpath[2:6], 20)
	binary.BigEndian.PutUint16(mpath[6:8], 1)
	mpath[8] = 0
	root := image[20*isoSector : 21*isoSector]
	offset := 0
	for _, record := range [][]byte{directoryRecord(20, isoSector, 2, []byte{0}), directoryRecord(20, isoSector, 2, []byte{1})} {
		copy(root[offset:], record)
		offset += len(record)
	}
	for i, name := range names {
		record := directoryRecord(extents[i], uint32(len(request.Files[i].Content)), 0, []byte(name))
		copy(root[offset:], record)
		offset += len(record)
	}
	for i, file := range request.Files {
		copy(image[int(extents[i])*isoSector:], file.Content)
	}
	return image, nil
}

func directoryRecord(extent, size uint32, flags byte, name []byte) []byte {
	length := 33 + len(name)
	if length%2 != 0 {
		length++
	}
	r := make([]byte, length)
	r[0] = byte(length)
	putBoth32(r[2:10], extent)
	putBoth32(r[10:18], size)
	copy(r[18:25], []byte{70, 1, 1, 0, 0, 0, 0})
	r[25] = flags
	putBoth16(r[28:32], 1)
	r[32] = byte(len(name))
	copy(r[33:], name)
	return r
}

func putBoth16(out []byte, value uint16) {
	binary.LittleEndian.PutUint16(out[:2], value)
	binary.BigEndian.PutUint16(out[2:4], value)
}
func putBoth32(out []byte, value uint32) {
	binary.LittleEndian.PutUint32(out[:4], value)
	binary.BigEndian.PutUint32(out[4:8], value)
}
func spacePad(out []byte, value string) {
	for i := range out {
		out[i] = ' '
	}
	copy(out, value)
}
