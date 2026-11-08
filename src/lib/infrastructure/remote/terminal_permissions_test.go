package remote

import (
	"os"
	"path/filepath"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestTerminalPermissionsRequireOneExplicitOwnerProfile(t *testing.T) {
	root := FeatureRootPermission{FeatureRoot: contract.FeatureRoot{ID: "owner", Name: "Owner", Terminal: true}, Path: t.TempDir()}
	profile := &TerminalProfile{Executable: filepath.Join(t.TempDir(), "owner-shell")}
	invalid := []FeaturePermissions{
		{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{root}},
		{Schema: contract.FeatureVersion, TerminalProfile: profile},
		{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{root, {FeatureRoot: contract.FeatureRoot{ID: "second", Name: "Second", Terminal: true}, Path: root.Path}}, TerminalProfile: profile},
	}
	for _, permissions := range invalid {
		destination := filepath.Join(t.TempDir(), "not-created")
		if err := SaveFeaturePermissions(destination, permissions); err == nil {
			t.Fatal("invalid terminal authority persisted")
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("invalid terminal configuration had filesystem effects")
		}
	}
	valid := FeaturePermissions{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{root}, TerminalProfile: profile}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	if !terminalSupported() {
		destination := filepath.Join(t.TempDir(), "not-created")
		if err := SaveFeaturePermissions(destination, valid); err == nil {
			t.Fatal("unsupported terminal grant persisted")
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("unsupported terminal config had effects")
		}
		catalog := valid.catalog(contract.FeatureTarget{})
		if catalog.Roots[0].Terminal {
			t.Fatal("unsupported worker advertised terminal")
		}
	}
}
