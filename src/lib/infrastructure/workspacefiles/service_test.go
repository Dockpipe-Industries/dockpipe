package workspacefiles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

func localService(t *testing.T, root string) *Service {
	t.Helper()
	service, err := New(root, invocation.Target{Kind: invocation.Local, ID: "fixture-host"},
		invocation.Identity{Issuer: "native-fixture", Subject: "local-owner"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func request(action, name string) Request {
	return Request{Schema: Schema, Action: action, Path: name}
}

func fixtureFile(t *testing.T, root, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), content, 0640); err != nil {
		t.Fatal(err)
	}
}

func TestReadAndMutationsPreserveBytesAndReviewedRevision(t *testing.T) {
	root := t.TempDir()
	service := localService(t, root)
	original := []byte("\xef\xbb\xbfhello\r\n")
	fixtureFile(t, root, "document.md", original)
	ctx := context.Background()
	opened, err := service.Run(ctx, request("read", "document.md"))
	if err != nil || !bytes.Equal(opened.Content, original) || opened.Revision != revision(original) {
		t.Fatalf("exact-byte read: %+v %v", opened, err)
	}
	write := request("write", "document.md")
	write.Revision = opened.Revision
	write.Content = []byte("updated\r\n")
	saved, err := service.Run(ctx, write)
	if err != nil || saved.Revision != revision(write.Content) {
		t.Fatalf("reviewed write: %+v %v", saved, err)
	}
	content, err := os.ReadFile(filepath.Join(root, "document.md"))
	if err != nil || !bytes.Equal(content, write.Content) {
		t.Fatalf("published bytes: %q %v", content, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(root, "document.md"))
		if err != nil || info.Mode().Perm() != 0640 {
			t.Fatalf("file permissions changed: %v %v", info, err)
		}
	}
	// A retry with a stale reviewed revision cannot overwrite the completed save.
	write.Content = []byte("stale edit")
	if _, err := service.Run(ctx, write); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale write accepted: %v", err)
	}
	rename := request("rename", "document.md")
	rename.Destination, rename.Revision = "renamed.md", saved.Revision
	renamed, err := service.Run(ctx, rename)
	if err != nil || renamed.Path != "renamed.md" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "document.md")); !os.IsNotExist(err) {
		t.Fatalf("old name remains after rename: %v", err)
	}
	deletion := request("delete", "renamed.md")
	deletion.Revision = opened.Revision
	if _, err := service.Run(ctx, deletion); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale delete accepted: %v", err)
	}
	deletion.Revision = saved.Revision
	if _, err := service.Run(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "renamed.md")); !os.IsNotExist(err) {
		t.Fatalf("delete did not remove the reviewed file: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("save left private temporary files: %v %v", entries, err)
	}
}

func TestCreateAndRenameNeverReplaceAnExistingDestination(t *testing.T) {
	root := t.TempDir()
	service := localService(t, root)
	fixtureFile(t, root, "existing", []byte("keep"))
	create := request("create", "existing")
	create.Content = []byte("replace")
	if _, err := service.Run(context.Background(), create); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("create replaced an existing file: %v", err)
	}
	create.Path = "new"
	created, err := service.Run(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	rename := request("rename", "new")
	rename.Destination, rename.Revision = "existing", created.Revision
	if _, err := service.Run(context.Background(), rename); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("rename replaced an existing file: %v", err)
	}
	for name, expected := range map[string]string{"existing": "keep", "new": "replace"} {
		actual, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(actual) != expected {
			t.Fatalf("destination collision changed %s: %q %v", name, actual, err)
		}
	}
}

func TestInputAndConfinementRejectBeforeEffects(t *testing.T) {
	root := t.TempDir()
	service := localService(t, root)
	for _, selected := range []Request{
		{Schema: "unknown", Action: "read", Path: "file"},
		{Schema: Schema, Action: "execute", Path: "file"},
		request("read", "../escape"), request("read", "/absolute"),
		request("read", "a/../escape"), request("read", "a\\escape"),
		request("read", ".git/config"), request("read", "folder/.GIT/config"),
		request("delete", "."), request("read", "a\x00b"),
		{Schema: Schema, Action: "write", Path: "file", Revision: "not-a-content-revision"},
		{Schema: Schema, Action: "list", Path: ".", Limit: MaxPageEntries + 1},
	} {
		if _, err := service.Run(context.Background(), selected); err == nil {
			t.Fatalf("accepted invalid request: %+v", selected)
		}
	}
	outside := t.TempDir()
	fixtureFile(t, outside, "secret", []byte("outside"))
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	for _, action := range []string{"read", "create", "mkdir"} {
		selected := request(action, "link/secret")
		if _, err := service.Run(context.Background(), selected); !errors.Is(err, ErrOutsideRoot) {
			t.Fatalf("%s traversed a directory link: %v", action, err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "file-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Run(context.Background(), request("read", "file-link")); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("read followed a file link: %v", err)
	}
	actual, err := os.ReadFile(filepath.Join(outside, "secret"))
	if err != nil || string(actual) != "outside" {
		t.Fatalf("outside content changed: %q %v", actual, err)
	}
}

func TestFileLimitsAndDirectoryPaging(t *testing.T) {
	root := t.TempDir()
	service := localService(t, root)
	fixtureFile(t, root, "limit", bytes.Repeat([]byte{'a'}, MaxFileBytes))
	if _, err := service.Run(context.Background(), request("read", "limit")); err != nil {
		t.Fatalf("maximum supported file rejected: %v", err)
	}
	fixtureFile(t, root, "limit", bytes.Repeat([]byte{'a'}, MaxFileBytes+1))
	if _, err := service.Run(context.Background(), request("read", "limit")); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized read accepted: %v", err)
	}
	create := request("create", "oversized")
	create.Content = bytes.Repeat([]byte{'b'}, MaxFileBytes+1)
	if _, err := service.Run(context.Background(), create); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized create accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "oversized")); !os.IsNotExist(err) {
		t.Fatalf("oversized create had an effect: %v", err)
	}
	directoryRoot := t.TempDir()
	directoryService := localService(t, directoryRoot)
	for index := 0; index < MaxPageEntries+3; index++ {
		fixtureFile(t, directoryRoot, fmt.Sprintf("entry-%03d", index), nil)
	}
	if err := os.Mkdir(filepath.Join(directoryRoot, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	page := request("list", ".")
	page.Limit, page.Hidden = MaxPageEntries, true
	first, err := directoryService.Run(context.Background(), page)
	if err != nil || len(first.Entries) != MaxPageEntries || !first.More {
		t.Fatalf("first directory page: %+v %v", first, err)
	}
	page.After, page.Revision = first.After, first.Revision
	second, err := directoryService.Run(context.Background(), page)
	if err != nil || len(second.Entries) != 3 || second.More {
		t.Fatalf("second directory page: %+v %v", second, err)
	}
	names := make([]string, 0, MaxPageEntries+3)
	for _, entry := range append(first.Entries, second.Entries...) {
		names = append(names, entry.Name)
	}
	if !sort.StringsAreSorted(names) || names[0] != "entry-000" {
		t.Fatalf("directory pages are unstable or include metadata: %v", names)
	}
	fixtureFile(t, directoryRoot, "added", nil)
	if _, err := directoryService.Run(context.Background(), page); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed directory reused a stale cursor: %v", err)
	}
}

func TestRemoteAuthorityIsRequiredAndOwnsPreparedInput(t *testing.T) {
	root := t.TempDir()
	target := invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "worker-lifetime"}
	identity := invocation.Identity{Issuer: "verified-fixture", Subject: "operator"}
	noAuthority, err := New(root, target, identity, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer noAuthority.Close()
	if _, err := noAuthority.Run(context.Background(), request("read", "file")); !errors.Is(err, invocation.ErrAuthorizationUnavailable) {
		t.Fatalf("remote read used a local authority fallback: %v", err)
	}
	denial := errors.New("fixture access denied")
	denied, err := New(filepath.Join(root, "nonexistent"), target, identity,
		func(context.Context, invocation.Binding) error { return denial })
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Close()
	if _, err := denied.Run(context.Background(), request("read", "file")); !errors.Is(err, denial) {
		t.Fatalf("filesystem access preceded authorization: %v", err)
	}
	create := request("create", "owned")
	create.Content = []byte("approved")
	var accepted invocation.Binding
	allowed, err := New(root, target, identity, func(_ context.Context, binding invocation.Binding) error {
		accepted = binding
		copy(create.Content, []byte("modified"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer allowed.Close()
	if _, err := allowed.Run(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "owned"))
	if err != nil || string(content) != "approved" || len(accepted.Digest) != 64 || accepted.Target != target || accepted.Contract.Action != "create" {
		t.Fatalf("authorized input was not owned/bound: %q %+v %v", content, accepted, err)
	}
}

func TestCancellationAndCloseDrainOwnedOperations(t *testing.T) {
	root := t.TempDir()
	service := localService(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Run(ctx, request("read", "file")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled call reached the filesystem: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Run(context.Background(), request("read", "file")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed service accepted work: %v", err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	target := invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("b", 64), Generation: "worker-2"}
	blocked, err := New(root, target, invocation.Identity{Issuer: "fixture", Subject: "operator"},
		func(context.Context, invocation.Binding) error {
			close(entered)
			<-release
			return errors.New("denied after cancellation fixture")
		})
	if err != nil {
		t.Fatal(err)
	}
	done, closed := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _ = blocked.Run(context.Background(), request("read", "file"))
	}()
	<-entered
	go func() {
		_ = blocked.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned while an owned operation was still active")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	for _, completed := range []<-chan struct{}{done, closed} {
		select {
		case <-completed:
		case <-time.After(2 * time.Second):
			t.Fatal("owned operation did not drain")
		}
	}
}

func TestFileOnlyRemovalAndPrivateTemporaryOwnership(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Mkdir("replacement-directory", 0700); err != nil {
		t.Fatal(err)
	}
	if err := removeFile(root, "replacement-directory"); err == nil {
		t.Fatal("file-only deletion removed an empty replacement directory")
	}
	if info, err := root.Lstat("replacement-directory"); err != nil || !info.IsDir() {
		t.Fatalf("replacement directory was changed: %v %v", info, err)
	}
	fixtureFile(t, directory, "private-save", []byte("original"))
	owned, err := root.Lstat("private-save")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("private-save", "published"); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, directory, "private-save", []byte("another writer"))
	if err := cleanupTemporary(root, "private-save", owned, true); err != nil {
		t.Fatalf("cleanup inspected a consumed save name: %v", err)
	}
	if err := cleanupTemporary(root, "private-save", owned, false); !errors.Is(err, ErrReconcile) {
		t.Fatalf("cleanup accepted a replacement save file: %v", err)
	}
	content, err := root.ReadFile("private-save")
	if err != nil || string(content) != "another writer" {
		t.Fatalf("cleanup removed another writer's file: %q %v", content, err)
	}
}
