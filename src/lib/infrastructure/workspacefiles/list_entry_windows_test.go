package workspacefiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/windows"
)

func TestListKeepsLockedEntryUnavailable(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, "locked.txt", []byte("locked"))
	fixtureFile(t, root, "visible.txt", []byte("visible"))

	service := localService(t, root)
	listRequest := request("list", ".")
	listRequest.Limit = 2
	ctx := context.Background()

	func() {
		path, err := windows.UTF16PtrFromString(filepath.Join(root, "locked.txt"))
		if err != nil {
			t.Fatal(err)
		}

		handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer windows.CloseHandle(handle)

		pinned, err := os.OpenRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		defer pinned.Close()

		if _, err := pinned.Lstat("locked.txt"); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatalf("fixture did not prevent metadata access: %v", err)
		}

		result, err := service.Run(ctx, listRequest)
		if err != nil {
			t.Fatal(err)
		}
		want := []Entry{
			{Name: "locked.txt", Kind: "unavailable"},
			{Name: "visible.txt", Kind: "file", SizeBytes: 7},
		}
		if !slices.Equal(result.Entries, want) {
			t.Fatalf("unexpected listing while file is locked: %+v", result.Entries)
		}
	}()

	result, err := service.Run(ctx, listRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || result.Entries[0] != (Entry{Name: "locked.txt", Kind: "file", SizeBytes: 6}) {
		t.Fatalf("unexpected listing after file was unlocked: %+v", result.Entries)
	}
}
