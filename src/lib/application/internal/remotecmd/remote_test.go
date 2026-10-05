package remotecmd

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

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
