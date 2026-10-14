package infrastructure

import (
	"context"
	"dockpipe/src/lib/infrastructure/telemetry"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDockerInventoryProjectionAndCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	directory := t.TempDir()
	script := `#!/bin/sh
case "$1:$2" in
container:ls) printf '%s\n' '{"ID":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Names":"web","State":"running","Command":"SECRET"}' ;;
network:ls) printf '%s\n' '{"ID":"net","Name":"network","Labels":"SECRET"}' ;;
volume:ls) printf '%s\n' '{"Name":"data","Mountpoint":"SECRET"}' ;;
container:inspect) printf '%s\n' '{"Id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Name":"/web","Labels":{"private":"SECRET","com.docker.compose.service":"web"},"Config":{"Env":["SECRET"]},"Networks":{"network":{"IPAddress":"SECRET"}},"Mounts":[{"Type":"volume","Name":"data","Source":"SECRET"}]}' ;;
*) exit 1 ;;
esac
`
	path := filepath.Join(directory, "docker")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("DOCKPIPE_DOCKER_BIN", path)
	snapshot := CollectDockerInventory(context.Background())
	encoded, err := json.Marshal(snapshot)
	if err != nil || !snapshot.Available || len(snapshot.Containers) != 1 || len(snapshot.Topology) != 1 || strings.Contains(string(encoded), "SECRET") {
		t.Fatalf("unsafe or missing inventory: %s %v", encoded, err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if result := CollectDockerInventory(ctx); result.Available || time.Since(started) > 2*time.Second || ctx.Err() == nil {
		t.Fatal("cancelled Docker query was not bounded")
	}
}

func TestInventoryBounds(t *testing.T) {
	data := []byte(strings.Repeat("{}\n", telemetry.MaxDockerObjects+1))
	rows, truncated, err := inventoryRows[telemetry.DockerVolume](data)
	if err != nil || !truncated || len(rows) != telemetry.MaxDockerObjects {
		t.Fatal("inventory object bound missing")
	}
	output := &inventoryOutput{}
	if _, err := output.Write(make([]byte, 512*1024+1)); err == nil || output.Len() != 0 {
		t.Fatal("oversized Docker output retained")
	}
}
