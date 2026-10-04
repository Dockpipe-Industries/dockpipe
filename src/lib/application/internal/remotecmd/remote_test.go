package remotecmd

import (
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestInitializationDoesNotReplaceCredentials(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	first, err := initialize(root, "127.0.0.1:47831")
	if err != nil {
		t.Fatal(err)
	}
	second, err := initialize(root, "127.0.0.1:47831")
	if err != nil || second.Token != first.Token {
		t.Fatalf("retry changed identity: %v", err)
	}
	if _, err := initialize(root, "0.0.0.0:47831"); err == nil {
		t.Fatal("accepted public plain HTTP listener")
	}
	if err := infrastructure.ValidatePrivatePath(filepath.Join(root, "operator.json"), false); err != nil {
		t.Fatal("operator file is not private")
	}
}

func TestServiceManifestQuotesPathsAndContainsNoCredentials(t *testing.T) {
	arguments := []string{"/Applications/Dock Pipe/bin/dockpipe", "remote", "worker", "--state", "/Users/test/A&B/remote"}
	plist := launchAgent("com.dockpipe.test", arguments, "/Users/test/A&B/remote/worker.log")
	decoder := xml.NewDecoder(strings.NewReader(plist))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
			break
		}
	}
	if !strings.Contains(plist, "A&amp;B") || strings.Contains(plist, "token") {
		t.Fatal(plist)
	}
	unit := systemdUnit([]string{"/tmp/a%name/$value/with space", "remote", "worker"})
	if !strings.Contains(unit, `"/tmp/a%%name/$$value/with space"`) || !strings.Contains(unit, "KillMode=control-group") {
		t.Fatal(unit)
	}
}
