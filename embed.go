package dockpipe

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"sort"
)

// BundledFS contains the exact authored inputs in embed_assets.go. Release
// preparation adds only its three declared tooling binaries. Files elsewhere in
// a checkout (including ignored build output) cannot enter the bundle.
var BundledFS = bundledFS{authoredFS}

var releaseFS embed.FS

type bundledFS struct{ embed.FS }

func (b bundledFS) ReadFile(name string) ([]byte, error) {
	data, err := b.FS.ReadFile(name)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return releaseFS.ReadFile(name)
	}
	return data, err
}

func (b bundledFS) ReadDir(name string) ([]fs.DirEntry, error) {
	base, err := b.FS.ReadDir(name)
	extra, extraErr := releaseFS.ReadDir(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if extraErr != nil && !errors.Is(extraErr, fs.ErrNotExist) {
		return nil, extraErr
	}
	if err != nil && extraErr != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(base))
	for _, entry := range base {
		seen[entry.Name()] = true
	}
	for _, entry := range extra {
		if !seen[entry.Name()] {
			base = append(base, entry)
		}
	}
	sort.Slice(base, func(i, j int) bool { return base[i].Name() < base[j].Name() })
	return base, nil
}

func (b bundledFS) Open(name string) (fs.File, error) {
	file, err := b.FS.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		file, err = releaseFS.Open(name)
	}
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		return file, err
	}
	entries, err := b.ReadDir(name)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &bundleDirectory{File: file, entries: entries}, nil
}

type bundleDirectory struct {
	fs.File
	entries []fs.DirEntry
}

func (d *bundleDirectory) ReadDir(n int) ([]fs.DirEntry, error) {
	if n > 0 && len(d.entries) == 0 {
		return nil, io.EOF
	}
	if n <= 0 || n > len(d.entries) {
		n = len(d.entries)
	}
	entries := d.entries[:n]
	d.entries = d.entries[n:]
	return entries, nil
}
