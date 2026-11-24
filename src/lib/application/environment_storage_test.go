package application

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/model/managedenvironment"
)

func TestEnvironmentStoragePassesOnlyVerifiedImageToWorkflow(t *testing.T) {
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("fixture requires %s", tool)
		}
	}
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("AWS_ACCESS_KEY_ID", "ambient-must-not-reach-hook")
	cache := filepath.Join(global, "cache", "environment-images")
	blob := []byte("fixture image contents")
	digestBytes := sha256.Sum256(blob)
	digest := hex.EncodeToString(digestBytes[:])
	base := filepath.Join(cache, digest+".qcow2")

	if err := infrastructure.WritePrivateJSON(filepath.Join(global, "environment-storage", "dev.json"), map[string]any{
		"schema":   environmentStorageProfileSchema,
		"resolver": "fixture-reader",
		"secrets": map[string]any{
			"resolver": "fixture-vault",
			"bindings": map[string]string{"R2_TEST_VALUE": "DEMO_PRIVATE_BINDING"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	writeFixtureResolver(t, global, "fixture-vault", "DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE", `
printf '%s' '{"schema":"dockpipe.secret-environment/v1","values":{"R2_TEST_VALUE":"value-never-in-workflow"}}'
`)
	writeFixtureResolver(t, global, "fixture-reader", "DOCKPIPE_ENVIRONMENT_STORAGE_RESTORE", fmt.Sprintf(`
set -eu
exec python3 -c '
import json, os, pathlib, sys
request = json.load(sys.stdin)
assert request["schema"] == "dockpipe.environment.storage/v1"
assert request["values"] == {"R2_TEST_VALUE": "value-never-in-workflow"}
assert "AWS_ACCESS_KEY_ID" not in os.environ
base = pathlib.Path(request["cache"]) / "%s.qcow2"
if not base.exists():
    base.write_bytes(b"fixture image contents")
    base.chmod(0o400)
print(json.dumps({"schema": request["schema"], "base": str(base), "sha256": "%s", "bytes": %d}))
'`, digest, digest, len(blob)))

	definition := &managedenvironment.Storage{Profile: "dev"}
	inherited := []string{"KEPT=present", "dockpipe_environment_image_base=spoofed",
		"DOCKPIPE_ENVIRONMENT_INSTANCE_ID=inherited", "dockpipe_environment_state_workdir=inherited"}
	got, err := prepareEnvironmentStorage(definition, inherited)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"KEPT=present", environmentImageBaseKey + "=" + base, environmentImageDigestKey + "=" + digest}
	if !slices.Equal(got, want) {
		t.Fatal("storage exposed an unexpected variable or omitted the verified image")
	}
	if err := os.Chmod(base, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base, []byte(strings.Repeat("X", len(blob))), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(base, 0o400); err != nil {
		t.Fatal(err)
	}
	if got, err := prepareEnvironmentStorage(definition, inherited); err == nil || got != nil {
		t.Fatal("a corrupt same-size cached image was admitted")
	}
}

func writeFixtureResolver(t *testing.T, global, name, key, script string) {
	t.Helper()
	dir := filepath.Join(global, "packages", "resolvers", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile"), []byte(key+"=fixture.sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}
