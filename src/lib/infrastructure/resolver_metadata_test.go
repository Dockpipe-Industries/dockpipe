package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverMetadataSetupContracts(t *testing.T) {
	for _, test := range []struct {
		name, capability, hooks, mode string
		invalid                       bool
	}{
		{"edge", "remote.edge", "DOCKPIPE_REMOTE_EDGE_SETUP=assets/setup.sh", "local", false},
		{"hosted", "remote.broker", "DOCKPIPE_REMOTE_BROKER_SETUP=assets/login.sh", "hosted", false},
		{"unrelated", "editor", "DOCKPIPE_REMOTE_BROKER_SETUP=assets/login.sh", "", false},
		{"ambiguous", "remote.edge", "DOCKPIPE_REMOTE_EDGE_SETUP=assets/setup.sh\nDOCKPIPE_REMOTE_BROKER_SETUP=assets/login.sh", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := fmt.Sprintf("schema: 1\nkind: resolver\nname: example.%s\nversion: 1.2.3\ntitle: Example provider\ncapability: %s\n", test.name, test.capability)
			if err := os.WriteFile(filepath.Join(root, "package.yml"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(root, "profile")
			if err := os.WriteFile(profile, []byte(test.hooks+"\nPRIVATE=do-not-publish"), 0600); err != nil {
				t.Fatal(err)
			}
			metadata, err := DescribeResolver(profile, "resolved.lookup.name")
			if test.invalid {
				if err == nil {
					t.Fatal("ambiguous setup accepted")
				}
				return
			}
			if err != nil || metadata.RemoteSetup != test.mode || metadata.Name != "resolved.lookup.name" || metadata.Version != "1.2.3" {
				t.Fatalf("metadata=%+v, error=%v", metadata, err)
			}
			data, err := json.Marshal(metadata)
			if err != nil || strings.Contains(string(data), "do-not-publish") || strings.Contains(string(data), "assets/") {
				t.Fatal("profile values exposed")
			}
		})
	}
}
