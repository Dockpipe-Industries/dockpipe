package infrastructure

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// materializeLibraryResources publishes a caller-selected, finite library resource
// set as one verified content-addressed cache directory. Callers select fixed
// embedded filenames; this helper does not enumerate workflow or package trees.
func materializeLibraryResources(workdir, namespace string, files map[string][]byte) (string, error) {
	if !fs.ValidPath(namespace) || namespace == "." || strings.Contains(namespace, "/") {
		return "", fmt.Errorf("invalid library resource namespace %q", namespace)
	}
	names := make([]string, 0, len(files))
	var total int64
	for name, content := range files {
		if !fs.ValidPath(name) || name == "." {
			return "", fmt.Errorf("invalid library resource path %q", name)
		}
		if total > math.MaxInt64-int64(len(content)) {
			return "", fmt.Errorf("library resource aggregate size overflows")
		}
		total += int64(len(content))
		names = append(names, name)
	}
	if _, err := resourceDirectoryChildren(files); err != nil {
		return "", err
	}
	sort.Strings(names)
	hash := sha256.New()
	var length [8]byte
	for _, name := range names {
		binary.LittleEndian.PutUint64(length[:], uint64(len(name)))
		hash.Write(length[:])
		hash.Write([]byte(name))
		binary.LittleEndian.PutUint64(length[:], uint64(len(files[name])))
		hash.Write(length[:])
		hash.Write(files[name])
	}
	name := hex.EncodeToString(hash.Sum(nil))
	cache, err := StateInternalCacheDir(workdir)
	if err != nil {
		return "", err
	}
	cache = filepath.Join(cache, "library-resources", namespace)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if existing, err := root.OpenRoot(name); err == nil {
		defer existing.Close()
		if err := verifyLibraryResources(existing, files); err != nil {
			return "", err
		}
		return filepath.Join(cache, name), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	temporaryName := ".resources-" + hex.EncodeToString(token[:])
	if err := root.Mkdir(temporaryName, 0o700); err != nil {
		return "", err
	}
	defer root.RemoveAll(temporaryName)
	temporary, err := root.OpenRoot(temporaryName)
	if err != nil {
		return "", err
	}
	for _, path := range names {
		if err := temporary.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", errors.Join(err, temporary.Close())
		}
		file, err := temporary.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return "", errors.Join(err, temporary.Close())
		}
		_, writeErr := file.Write(files[path])
		closeErr := file.Close()
		if writeErr != nil {
			return "", errors.Join(writeErr, closeErr, temporary.Close())
		}
		if closeErr != nil {
			return "", errors.Join(closeErr, temporary.Close())
		}
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := root.Rename(temporaryName, name); err != nil {
		// Another caller may have published the same immutable resource set.
		existing, openErr := root.OpenRoot(name)
		if openErr != nil {
			return "", err
		}
		defer existing.Close()
		if err := verifyLibraryResources(existing, files); err != nil {
			return "", err
		}
	}
	return filepath.Join(cache, name), nil
}

// resourceDirectoryChildren derives the exact finite directory shape from the
// caller-selected filenames. Directory enumeration is bounded by this shape.
func resourceDirectoryChildren(files map[string][]byte) (map[string]map[string]bool, error) {
	directories := map[string]map[string]bool{".": {}}
	for name := range files {
		parts := strings.Split(name, "/")
		parent := "."
		for index, part := range parts {
			isDirectory := index < len(parts)-1
			children := directories[parent]
			if previous, exists := children[part]; exists && previous != isDirectory {
				return nil, fmt.Errorf("conflicting library resource path %q", name)
			}
			children[part] = isDirectory
			if isDirectory {
				parent = namePrefix(parent, part)
				if directories[parent] == nil {
					directories[parent] = make(map[string]bool)
				}
			}
		}
	}
	return directories, nil
}

func namePrefix(parent, name string) string {
	if parent == "." {
		return name
	}
	return parent + "/" + name
}

func verifyLibraryResources(root *os.Root, files map[string][]byte) error {
	directories, err := resourceDirectoryChildren(files)
	if err != nil {
		return err
	}
	return verifyLibraryResourceDirectory(root, ".", directories, files)
}

func verifyLibraryResourceDirectory(root *os.Root, directory string, directories map[string]map[string]bool, files map[string][]byte) error {
	children := directories[directory]
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	// One extra entry proves that the cache differs without enumerating an
	// arbitrarily wide directory. Unknown subtrees are rejected before descent.
	entries, readErr := listing.ReadDir(len(children) + 1)
	closeErr := listing.Close()
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) != len(children) {
		return fmt.Errorf("library cache directory %q has unexpected entries", directory)
	}
	for _, entry := range entries {
		path := namePrefix(directory, entry.Name())
		isDirectory, expected := children[entry.Name()]
		if !expected || isDirectory != entry.IsDir() {
			return fmt.Errorf("unexpected library cache entry %q", path)
		}
		if isDirectory {
			child, err := root.OpenRoot(entry.Name())
			if err != nil {
				return err
			}
			verifyErr := verifyLibraryResourceDirectory(child, path, directories, files)
			closeErr := child.Close()
			if verifyErr != nil {
				return verifyErr
			}
			if closeErr != nil {
				return closeErr
			}
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected library cache entry %q", path)
		}
		if err := verifyLibraryResourceFile(root, entry.Name(), files[path]); err != nil {
			return fmt.Errorf("library cache resource %q: %w", path, err)
		}
	}
	return nil
}

func verifyLibraryResourceFile(root *os.Root, name string, expected []byte) error {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() || info.Size() != int64(len(expected)) {
		return errors.Join(fmt.Errorf("invalid file type or size"), file.Close())
	}
	content, readErr := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
	closeErr := file.Close()
	if readErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if !bytes.Equal(content, expected) {
		return fmt.Errorf("content does not match this binary")
	}
	return nil
}
