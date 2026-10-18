package packagecatalog

import (
	"fmt"

	"dockpipe/src/lib/infrastructure/archivecodec"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

// SelectArchive changes the encoding, never the package identity or version.
// Selection is explicit: an unavailable requested encoding is an error.
func SelectArchive(entry Entry, codec string) (Entry, error) {
	if codec == "" {
		codec = "gzip"
	}
	if err := validateEntry(entry); err != nil {
		return Entry{}, err
	}
	current, err := archivecodec.CodecForPath(entry.Tarball)
	if err != nil {
		return Entry{}, err
	}
	if current == codec {
		return entry, nil
	}
	for _, variant := range entry.Variants {
		if variant.Codec == codec {
			return withVariant(entry, variant), nil
		}
	}
	return Entry{}, fmt.Errorf("package %s/%s has no %s archive", entry.Kind, entry.Name, codec)
}

func withVariant(entry Entry, variant packagebuild.ArchiveVariant) Entry {
	entry.Codec = variant.Codec
	entry.Tarball = variant.Tarball
	entry.SHA256 = variant.SHA256
	entry.SizeBytes = variant.SizeBytes
	entry.DecodedSHA256 = variant.DecodedSHA256
	entry.DecodedSizeBytes = variant.DecodedSizeBytes
	entry.Variants = nil
	return entry
}
