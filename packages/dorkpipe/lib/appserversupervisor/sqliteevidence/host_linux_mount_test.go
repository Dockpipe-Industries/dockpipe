//go:build linux

package sqliteevidence

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRequireQualifiedExt4MountAcceptsDirectWholeDeviceMounts(t *testing.T) {
	for _, test := range []struct {
		name       string
		root       string
		mountPoint string
	}{
		{name: "host root filesystem", root: "/var/tmp/dockpipe-evidence", mountPoint: "/"},
		{name: "VM qualification device", root: "/var/lib/dockpipe-qualification", mountPoint: "/var/lib/dockpipe-qualification"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mount, mounts := qualifiedExt4MountFixture(test.root, test.mountPoint)
			if err := requireQualifiedExt4MountWithProbes(test.root, linuxPathFact{FSMagic: linuxExt4Magic}, mount, mounts, qualifiedBlockDevice, fixedRemovable(false, nil)); err != nil {
				t.Fatalf("direct whole-device ext4 mount rejected: %v", err)
			}
		})
	}
}

func TestRequireQualifiedExt4MountRejectsUnqualifiedMounts(t *testing.T) {
	const root = "/var/lib/dockpipe-qualification"
	errProbe := errors.New("probe failed")

	for _, test := range []struct {
		name           string
		mutate         func(*linuxPathFact, *linuxMountInfo, *[]linuxMountInfo)
		statSource     func(string) (os.FileMode, error)
		blockRemovable func(string) (bool, error)
		want           string
	}{
		{
			name: "bind mount",
			mutate: func(_ *linuxPathFact, mount *linuxMountInfo, _ *[]linuxMountInfo) {
				mount.Root = "/qualification"
			},
			want: "want whole-device root /",
		},
		{
			name: "mount point does not equal fixture root",
			mutate: func(_ *linuxPathFact, mount *linuxMountInfo, _ *[]linuxMountInfo) {
				mount.MountPoint = "/var/lib"
			},
			want: "mounted at / or fixture root",
		},
		{
			name: "wrong filesystem",
			mutate: func(_ *linuxPathFact, mount *linuxMountInfo, _ *[]linuxMountInfo) {
				mount.FileSystem = "xfs"
			},
			want: "want 0xef53/ext4",
		},
		{
			name: "wrong filesystem magic",
			mutate: func(rootFact *linuxPathFact, _ *linuxMountInfo, _ *[]linuxMountInfo) {
				rootFact.FSMagic = 0
			},
			want: "want 0xef53/ext4",
		},
		{
			name: "read-only mount",
			mutate: func(_ *linuxPathFact, mount *linuxMountInfo, _ *[]linuxMountInfo) {
				mount.MountOptions = "ro,relatime"
			},
			want: "not unambiguously read-write",
		},
		{
			name: "non-device source",
			statSource: func(string) (os.FileMode, error) {
				return 0, nil
			},
			want: "not an accessible block device",
		},
		{
			name: "inaccessible device source",
			statSource: func(string) (os.FileMode, error) {
				return 0, errProbe
			},
			want: "not an accessible block device",
		},
		{
			name:           "removable device",
			blockRemovable: fixedRemovable(true, nil),
			want:           "source removable=true",
		},
		{
			name:           "unknown removability",
			blockRemovable: fixedRemovable(false, errProbe),
			want:           "probe failed",
		},
		{
			name: "non-local source",
			mutate: func(_ *linuxPathFact, mount *linuxMountInfo, _ *[]linuxMountInfo) {
				mount.Source = "host:/qualification"
			},
			want: "not a local block device",
		},
		{
			name: "nested mount",
			mutate: func(_ *linuxPathFact, _ *linuxMountInfo, mounts *[]linuxMountInfo) {
				*mounts = append(*mounts, linuxMountInfo{ID: 3, MountPoint: root + "/nested", Raw: "nested mount"})
			},
			want: "crosses or contains nested mount",
		},
		{
			name: "crossing ancestor mount",
			mutate: func(_ *linuxPathFact, _ *linuxMountInfo, mounts *[]linuxMountInfo) {
				*mounts = append(*mounts, linuxMountInfo{ID: 3, MountPoint: "/var/lib", Raw: "crossing mount"})
			},
			want: "crosses or contains nested mount",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootFact := linuxPathFact{FSMagic: linuxExt4Magic}
			mount, mounts := qualifiedExt4MountFixture(root, root)
			if test.mutate != nil {
				test.mutate(&rootFact, &mount, &mounts)
			}
			statSource := test.statSource
			if statSource == nil {
				statSource = qualifiedBlockDevice
			}
			blockRemovable := test.blockRemovable
			if blockRemovable == nil {
				blockRemovable = fixedRemovable(false, nil)
			}
			err := requireQualifiedExt4MountWithProbes(root, rootFact, mount, mounts, statSource, blockRemovable)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func qualifiedExt4MountFixture(root, mountPoint string) (linuxMountInfo, []linuxMountInfo) {
	mount := linuxMountInfo{
		ID:           2,
		ParentID:     1,
		Major:        252,
		Minor:        16,
		Root:         "/",
		MountPoint:   mountPoint,
		MountOptions: "rw,relatime",
		FileSystem:   "ext4",
		Source:       "/dev/vdb",
		SuperOptions: "rw",
		Raw:          "direct ext4 qualification mount",
	}
	mounts := []linuxMountInfo{mount}
	if mountPoint != "/" {
		mounts = append(mounts, linuxMountInfo{ID: 1, MountPoint: "/", Raw: "root filesystem"})
	}
	return mount, mounts
}

func qualifiedBlockDevice(string) (os.FileMode, error) {
	return os.ModeDevice, nil
}

func fixedRemovable(removable bool, err error) func(string) (bool, error) {
	return func(string) (bool, error) {
		return removable, err
	}
}
