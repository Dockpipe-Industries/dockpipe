package workspacefiles

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// ErrReconcile means an effect or cleanup may have completed. Refresh and inspect;
// a transport must never replay a mutation merely because it received this error.
var ErrReconcile = errors.New("the file operation needs inspection before retrying")

func mutate(ctx context.Context, root *os.Root, request Request) (Result, error) {
	selected, name, err := parent(ctx, root, request.Path)
	if err != nil {
		return Result{}, err
	}
	defer selected.Close()
	if request.Action == "mkdir" {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if err := selected.Mkdir(name, 0755); err != nil {
			return Result{}, err
		}
		return Result{Schema: Schema, Path: request.Path}, nil
	}
	if request.Action == "create" {
		return publish(ctx, selected, name, request, nil)
	}
	content, info, err := readAt(ctx, selected, name)
	if err != nil {
		return Result{}, err
	}
	if request.Revision != revision(content) {
		return Result{}, ErrChanged
	}
	switch request.Action {
	case "write":
		return publish(ctx, selected, name, request, info)
	case "delete":
		if err := unchanged(ctx, selected, name, info); err != nil {
			return Result{}, err
		}
		if err := removeFile(selected, name); err != nil {
			return Result{}, err
		}
		return Result{Schema: Schema, Path: request.Path}, nil
	case "rename":
		destination, destinationName, err := parent(ctx, root, request.Destination)
		if err != nil {
			return Result{}, err
		}
		defer destination.Close()
		// Root.Link publishes a new regular-file name without replacing any
		// destination. Directory renames require a native no-replace primitive;
		// they are deliberately not emulated with a racy exists/Rename sequence.
		// Link accepts two paths under one pinned root; it never follows a new
		// destination outside the granted root.
		if err := unchanged(ctx, selected, name, info); err != nil {
			return Result{}, err
		}
		if err := linkBetween(root, selected, name, destination, destinationName, request.Path, request.Destination); err != nil {
			return Result{}, err
		}
		linked, linkError := destination.Lstat(destinationName)
		if linkError != nil || !linked.Mode().IsRegular() || !os.SameFile(info, linked) {
			return Result{}, ErrReconcile
		}
		if err := unchanged(ctx, selected, name, info); err != nil {
			return Result{}, cleanupLink(destination, destinationName, info, err)
		}
		if err := removeFile(selected, name); err != nil {
			return Result{}, cleanupLink(destination, destinationName, info, err)
		}
		return Result{Schema: Schema, Path: request.Destination, Revision: request.Revision}, nil
	default:
		return Result{}, ErrInvalidRequest
	}
}

func unchanged(ctx context.Context, root *os.Root, name string, expected os.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := root.Lstat(name)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(expected, current) ||
		expected.Size() != current.Size() || !expected.ModTime().Equal(current.ModTime()) {
		return ErrChanged
	}
	return nil
}

func cleanupLink(root *os.Root, name string, expected os.FileInfo, cause error) error {
	current, err := root.Lstat(name)
	if err != nil || !os.SameFile(expected, current) {
		return errors.Join(cause, ErrReconcile)
	}
	if err := removeFile(root, name); err != nil {
		return errors.Join(cause, ErrReconcile, err)
	}
	return cause
}

func publish(ctx context.Context, root *os.Root, name string, request Request, previous os.FileInfo) (result Result, resultError error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Result{}, err
	}
	temporary := ".dockpipe-save-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	created, err := file.Stat()
	if err != nil {
		return Result{}, errors.Join(err, ErrReconcile)
	}
	consumed := false
	defer func() {
		if err := cleanupTemporary(root, temporary, created, consumed); err != nil {
			resultError = errors.Join(resultError, err)
		}
	}()
	if _, err := file.Write(request.Content); err != nil {
		return Result{}, err
	}
	// New files remain private. Do not widen permissions by chmod'ing a fixed
	// 0644 mode that would bypass a stricter process umask.
	mode := os.FileMode(0600)
	if previous != nil {
		mode = previous.Mode().Perm()
	}
	if err := file.Chmod(mode); err != nil {
		return Result{}, err
	}
	if err := file.Sync(); err != nil {
		return Result{}, err
	}
	// Close before publication for Windows handle-sharing compatibility.
	if err := file.Close(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	temporaryInfo, err := root.Lstat(temporary)
	if err != nil || !temporaryInfo.Mode().IsRegular() || !os.SameFile(created, temporaryInfo) {
		return Result{}, ErrChanged
	}
	if previous == nil {
		if err := root.Link(temporary, name); err != nil {
			return Result{}, err
		}
	} else {
		if err := unchanged(ctx, root, name, previous); err != nil {
			return Result{}, err
		}
		if err := root.Rename(temporary, name); err != nil {
			return Result{}, err
		}
		consumed = true
	}
	return Result{Schema: Schema, Path: request.Path, Revision: revision(request.Content)}, nil
}

func cleanupTemporary(root *os.Root, name string, owned os.FileInfo, consumed bool) error {
	if consumed {
		return nil
	}
	current, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(owned, current) {
		return ErrReconcile
	}
	if err := removeFile(root, name); err != nil {
		return errors.Join(ErrReconcile, fmt.Errorf("remove private save file: %w", err))
	}
	return nil
}
