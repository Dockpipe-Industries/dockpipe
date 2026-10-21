package mcpbundle

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Installed is an immutable, verified artifact receipt. Installation neither
// edits project configuration nor launches a process or grants filesystem access.
type Installed struct {
	Inspection Inspection `json:"inspection"`
	Directory  string     `json:"directory"`
	Executable string     `json:"executable"`
	Started    bool       `json:"started"`
}

// Install materializes a verified native bundle under the caller's existing
// artifact cache. Names are digest-derived, publication is atomic and existing
// versions are never overwritten. Updates select a new digest, not mutable files.
func Install(ctx context.Context, filename, expected, platform, arch, cache string) (Installed, error) {
	return InstallConfigured(ctx, filename, expected, platform, arch, cache, Bindings{})
}

func InstallConfigured(ctx context.Context, filename, expected, platform, arch, cache string, bindings Bindings) (Installed, error) {
	var result Installed
	// Copy into an exclusive task-owned snapshot before inspection/extraction so
	// concurrent changes to the original cannot change the verified launch bytes.
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		return result, err
	}
	defer root.Close()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return result, err
	}
	stage := ".stage-" + hex.EncodeToString(nonce[:])
	if err := root.Mkdir(stage, 0o700); err != nil {
		return result, err
	}
	defer root.RemoveAll(stage)
	snapshot, err := root.OpenFile(filepath.Join(stage, "archive.mcpb"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, err
	}
	source, err := os.Open(filename)
	if err != nil {
		return result, errors.Join(err, snapshot.Close())
	}
	n, copyErr := copyContext(ctx, snapshot, io.LimitReader(source, MaxArchiveBytes+1))
	closeErr := errors.Join(source.Close(), snapshot.Close())
	if copyErr != nil {
		return result, errors.Join(copyErr, closeErr)
	}
	if closeErr != nil {
		return result, closeErr
	}
	if n > MaxArchiveBytes {
		return result, fmt.Errorf("MCPB exceeds archive budget")
	}
	snapshotPath := filepath.Join(cache, stage, "archive.mcpb")
	inspected, err := InspectConfigured(ctx, snapshotPath, expected, platform, arch, bindings)
	if err != nil {
		return result, err
	}
	destination := inspected.SHA256 + "-" + platform + "-" + arch
	if inspected.ConfigurationDigest != "" {
		destination += "-" + inspected.ConfigurationDigest
	}
	if _, err := root.Lstat(destination); err == nil {
		return result, fmt.Errorf("MCPB artifact already exists; existing versions are never overwritten")
	} else if !os.IsNotExist(err) {
		return result, err
	}
	payload := filepath.Join(stage, "payload")
	if err := root.Mkdir(payload, 0o700); err != nil {
		return result, err
	}
	archive, err := zip.OpenReader(snapshotPath)
	if err != nil {
		return result, err
	}
	defer archive.Close()
	for _, member := range archive.File {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := filepath.Join(payload, filepath.FromSlash(member.Name))
		if member.Mode().IsDir() {
			if err := root.MkdirAll(name, 0o700); err != nil {
				return result, err
			}
			continue
		}
		if err := root.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			return result, err
		}
		mode := os.FileMode(0o600)
		if member.Mode().Perm()&0o111 != 0 || member.Name == inspected.EntryPoint {
			mode = 0o700
		}
		out, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return result, err
		}
		in, err := member.Open()
		if err != nil {
			return result, errors.Join(err, out.Close())
		}
		if member.UncompressedSize64 > maxExpandedBytes {
			return result, errors.Join(fmt.Errorf("MCPB member exceeds expansion budget"), in.Close(), out.Close())
		}
		expectedSize := int64(member.UncompressedSize64)
		n, copyErr := copyContext(ctx, out, io.LimitReader(in, expectedSize+1))
		closeErr := errors.Join(in.Close(), out.Close())
		if copyErr != nil {
			return result, errors.Join(copyErr, closeErr)
		}
		if closeErr != nil {
			return result, closeErr
		}
		if n != expectedSize {
			return result, fmt.Errorf("MCPB expansion changed")
		}
	}
	if err := archive.Close(); err != nil {
		return result, err
	}
	// Keep original digest-verified archive beside the payload for independent
	// provenance verification. No installer script or repository code is executed.
	receipt, err := json.MarshalIndent(inspected, "", "  ")
	if err != nil {
		return result, err
	}
	out, err := root.OpenFile(filepath.Join(stage, "receipt.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, err
	}
	_, writeErr := out.Write(append(receipt, '\n'))
	closeErr = out.Close()
	if writeErr != nil {
		return result, writeErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := root.Rename(stage, destination); err != nil {
		return result, err
	}
	directory := filepath.Join(cache, destination, "payload")
	return Installed{inspected, directory, filepath.Join(directory, filepath.FromSlash(inspected.EntryPoint)), false}, nil
}
