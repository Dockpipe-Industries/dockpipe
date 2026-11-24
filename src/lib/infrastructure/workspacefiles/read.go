package workspacefiles

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

// Every traversed directory is checked and pinned before using its children.
// os.Root enforces the actual root boundary, including path replacement races.
func directory(ctx context.Context, root *os.Root, name string) (*os.Root, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return current, nil
	}
	for _, component := range strings.Split(name, "/") {
		if err := ctx.Err(); err != nil {
			current.Close()
			return nil, err
		}
		before, err := current.Lstat(component)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			if err != nil {
				return nil, err
			}
			return nil, ErrOutsideRoot
		}
		next, err := current.OpenRoot(component)
		current.Close()
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(before, opened) {
			next.Close()
			return nil, ErrChanged
		}
		current = next
	}
	return current, nil
}

func parent(ctx context.Context, root *os.Root, name string) (*os.Root, string, error) {
	selected, err := directory(ctx, root, path.Dir(name))
	return selected, path.Base(name), err
}

func read(ctx context.Context, root *os.Root, name string) ([]byte, os.FileInfo, error) {
	selected, base, err := parent(ctx, root, name)
	if err != nil {
		return nil, nil, err
	}
	defer selected.Close()
	return readAt(ctx, selected, base)
}

func readAt(ctx context.Context, root *os.Root, name string) ([]byte, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, ErrNotRegular
	}
	if before.Size() > MaxFileBytes {
		return nil, nil, ErrLimit
	}
	file, err := openRead(root, name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, nil, ErrChanged
	}
	content, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(content) > MaxFileBytes {
		return nil, nil, ErrLimit
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, nil, ErrChanged
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return content, after, nil
}

type namesHeap []string

func (names namesHeap) Len() int                  { return len(names) }
func (names namesHeap) Less(left, right int) bool { return names[left] > names[right] }
func (names namesHeap) Swap(left, right int)      { names[left], names[right] = names[right], names[left] }
func (names *namesHeap) Push(value any)           { *names = append(*names, value.(string)) }
func (names *namesHeap) Pop() any {
	old := *names
	value := old[len(old)-1]
	*names = old[:len(old)-1]
	return value
}

func list(ctx context.Context, root *os.Root, request Request) (Result, error) {
	// Match the existing graph reader's 20-second directory contract. The CLI
	// process also owns an outer deadline for blocking filesystem calls.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	selected, err := directory(ctx, root, request.Path)
	if err != nil {
		return Result{}, err
	}
	defer selected.Close()
	before, err := selected.Stat(".")
	if err != nil {
		return Result{}, err
	}
	stream, err := selected.Open(".")
	if err != nil {
		return Result{}, err
	}
	defer stream.Close()
	names := &namesHeap{}
	fingerprint := [sha256.Size]byte{}
	scanned := uint64(0)
	retainedBytes := 0
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		batch, readError := stream.ReadDir(256)
		for _, entry := range batch {
			name := entry.Name()
			if !utf8.ValidString(name) || len(name) > MaxPathBytes {
				return Result{}, errors.New("directory contains a name the file protocol cannot represent")
			}
			sum := sha256.Sum256([]byte(name))
			for index := range fingerprint {
				fingerprint[index] ^= sum[index]
			}
			scanned++
			if strings.EqualFold(name, ".git") || !request.Hidden && strings.HasPrefix(name, ".") || name <= request.After {
				continue
			}
			if names.Len() == request.Limit+1 && name >= (*names)[0] {
				continue
			}
			if names.Len() == request.Limit+1 {
				removed := heap.Pop(names).(string)
				retainedBytes -= len(removed)
			}
			// Reserve room for JSON's worst-case sixfold string escaping in
			// the existing 2 MiB directory-response budget.
			if retainedBytes+len(name) > (2<<20)/6 {
				return Result{}, errors.New("directory page names exceed the response budget")
			}
			heap.Push(names, name)
			retainedBytes += len(name)
		}
		if errors.Is(readError, io.EOF) {
			break
		}
		if readError != nil {
			return Result{}, readError
		}
	}
	after, err := selected.Stat(".")
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return Result{}, ErrChanged
	}
	stamp := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%x", after.ModTime().UnixNano(), scanned, fingerprint)))
	version := hex.EncodeToString(stamp[:])
	if request.Revision != "" && request.Revision != version {
		return Result{}, ErrChanged
	}
	more := names.Len() > request.Limit
	if more {
		heap.Pop(names)
	}
	ordered := make([]string, names.Len())
	for index := len(ordered) - 1; index >= 0; index-- {
		ordered[index] = heap.Pop(names).(string)
	}
	result := Result{Schema: Schema, Path: request.Path, Revision: version, More: more, After: request.After,
		Entries: make([]Entry, 0, len(ordered))}
	for _, name := range ordered {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		info, err := selected.Lstat(name)
		entry := Entry{Name: name, Kind: "unavailable"}
		if err != nil {
			// An enumerable name can deny metadata access or be locked.
			// Do not open or classify its target.
			if !entryMetadataUnavailable(err) {
				return Result{}, ErrChanged
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			entry.Kind = "link"
		} else if info.IsDir() {
			entry.Kind = "directory"
		} else if info.Mode().IsRegular() {
			entry.Kind = "file"
			entry.SizeBytes = info.Size()
		}
		result.Entries = append(result.Entries, entry)
		result.After = name
	}
	return result, nil
}
