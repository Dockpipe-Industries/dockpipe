//go:build windows

package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsWorkerLockExcludesOtherHandlesAndReleases(t *testing.T) {
	root := privateTestDirectory(t)
	release, err := LockWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	other, err := LockWorker(root)
	if other != nil {
		other()
	}
	if err == nil || !strings.Contains(err.Error(), "another remote process") {
		t.Fatalf("expected immediate lock contention, got %v", err)
	}

	if err := os.Remove(filepath.Join(root, "worker.lock")); err == nil {
		t.Fatal("the held lock was removable")
	}

	release()
	again, err := LockWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer again()
}

func TestWindowsFeatureWorkerLockHasIndependentLifetime(t *testing.T) {
	root := privateTestDirectory(t)
	releaseWorker, err := LockWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWorker()

	releaseFeatures, err := lockFeatureWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFeatures()

	other, err := lockFeatureWorker(root)
	if other != nil {
		other()
	}
	if err == nil || !strings.Contains(err.Error(), "another remote process") {
		t.Fatalf("expected immediate feature lock contention, got %v", err)
	}

	releaseFeatures()
	again, err := lockFeatureWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer again()
}

func TestWindowsDesktopWorkerLockExcludesConcurrentDesktopOwners(t *testing.T) {
	root := privateTestDirectory(t)
	releaseWorker, err := LockWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWorker()

	releaseDesktop, err := LockDesktopWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseDesktop()

	other, err := LockDesktopWorker(root)
	if other != nil {
		other()
	}
	if err == nil || !strings.Contains(err.Error(), "another remote process") {
		t.Fatalf("expected desktop lock contention, got %v", err)
	}

	releaseDesktop()
	again, err := LockDesktopWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	defer again()
}
