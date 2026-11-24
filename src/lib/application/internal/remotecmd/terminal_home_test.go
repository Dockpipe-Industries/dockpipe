package remotecmd

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestTerminalHomePreservesWorkspacePermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	homeVariable := "HOME"
	if runtime.GOOS == "windows" {
		homeVariable = "USERPROFILE"
	}
	t.Setenv(homeVariable, home)
	t.Setenv("DOCKPIPE_REMOTE_WORKER", "")

	project := remoteio.FeatureRootPermission{
		FeatureRoot: contract.FeatureRoot{ID: "project", Name: "Project", FilesRead: true, Terminal: true},
		Path:        t.TempDir(),
	}
	permissions := remoteio.FeaturePermissions{
		Schema:          contract.FeatureVersion,
		Roots:           []remoteio.FeatureRootPermission{project},
		DockerRead:      true,
		TerminalProfile: &remoteio.TerminalProfile{Executable: filepath.Join(t.TempDir(), "owner-shell")},
	}
	expected := permissions
	expected.Roots = []remoteio.FeatureRootPermission{project, {
		FeatureRoot: contract.FeatureRoot{ID: "home", Name: "Home", Terminal: true},
		Path:        home,
	}}
	for _, existing := range []remoteio.FeaturePermissions{permissions, expected} {
		if err := remoteio.WritePrivate(filepath.Join(root, "features.json"), existing); err != nil {
			t.Fatal(err)
		}
		got, err := terminalHomePermissions(root)
		if err != nil || !reflect.DeepEqual(got, expected) {
			t.Fatalf("home grant changed existing authority: %v", err)
		}
	}

	t.Setenv("DOCKPIPE_REMOTE_WORKER", "1")
	if _, err := terminalHomePermissions(root); err == nil {
		t.Fatal("delivered workflow enabled the home default")
	}
}
