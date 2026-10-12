package infrastructure

import (
	"time"

	"dockpipe/src/lib/infrastructure/sourcemtime"
)

// MaxModTimeFilesUnder returns the latest mod time of any regular file under root.
func MaxModTimeFilesUnder(root string) (time.Time, error) {
	return sourcemtime.MaxModTimeFilesUnder(root)
}

// SourceDirNewerThanPath reports whether source files are newer than refPath.
func SourceDirNewerThanPath(srcRoot, refPath string) (bool, error) {
	return sourcemtime.SourceDirNewerThanPath(srcRoot, refPath)
}

// PickLatestModTimePath returns the newest existing path, or an empty string.
func PickLatestModTimePath(paths []string) string {
	return sourcemtime.PickLatestModTimePath(paths)
}
