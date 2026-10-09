//go:build linux || darwin

package cloudflare

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestDebianDependencyInstaller(t *testing.T) {
	metadata, err := domain.ParsePackageManifest("../../resolvers/dockpipe.cloudflare.remote-edge/package.yml")
	if err != nil {
		t.Fatal(err)
	}
	var installer string
	for _, dependency := range metadata.Dependencies.Host {
		if dependency.Command == "cloudflared" {
			installer = dependency.Install.Deb
		}
	}
	if installer == "" {
		t.Fatal("cloudflared must declare a Debian installer")
	}

	for _, failure := range []string{"", "download", "key", "update"} {
		t.Run("failure="+failure, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			temporary := filepath.Join(root, "temporary")
			for _, directory := range []string{bin, temporary} {
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"mktemp", "rm"} {
				path, err := exec.LookPath(command)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(bin, command)); err != nil {
					t.Fatal(err)
				}
			}
			fixtures := map[string]string{
				"dpkg": "#!/bin/sh\n[ \"$*\" = --print-architecture ] || exit 1\nprintf 'amd64\\n'\n",
				"curl": `#!/bin/sh
set -eu
printf 'download\n' >> "$TEST_ROOT/calls"
[ "$TEST_FAILURE" != download ] || exit 22
found_url=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    https://pkg.cloudflare.com/cloudflare-main.gpg) found_url=true ;;
    --output) shift; output=$1 ;;
  esac
  shift
done
[ "$found_url" = true ]
printf 'test-signing-key' > "$output"
`,
				"sudo": `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$TEST_ROOT/calls"
case "$*" in
  'install -d -m 0755 /usr/share/keyrings') ;;
  'install -m 0644 '*'/usr/share/keyrings/cloudflare-main.gpg')
    [ "$TEST_FAILURE" != key ] || exit 1
    /bin/cp "$4" "$TEST_ROOT/key"
    ;;
  'install -m 0644 '*'/etc/apt/sources.list.d/cloudflared.list')
    /bin/cp "$4" "$TEST_ROOT/source"
    ;;
  'apt-get update -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/cloudflared.list -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0')
    [ -f "$TEST_ROOT/key" ] && [ -f "$TEST_ROOT/source" ]
    [ "$TEST_FAILURE" != update ] || exit 100
    touch_file="$TEST_ROOT/refreshed"
    printf done > "$touch_file"
    ;;
  'apt-get install -y cloudflared')
    [ -f "$TEST_ROOT/refreshed" ]
    printf done > "$TEST_ROOT/installed"
    ;;
  'apt-get update'|'apt-get install -y ca-certificates curl') ;;
  *) exit 99 ;;
esac
`,
			}
			for name, contents := range fixtures {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("/bin/sh", "-c", installer)
			// No real sudo, curl or apt executable is reachable by the installer.
			command.Env = []string{"PATH=" + bin, "TMPDIR=" + temporary, "TEST_ROOT=" + root, "TEST_FAILURE=" + failure}
			output, err := command.CombinedOutput()
			if (err != nil) != (failure != "") {
				t.Fatalf("installer result: %v\n%s", err, output)
			}
			_, installedErr := os.Stat(filepath.Join(root, "installed"))
			if (installedErr == nil) != (failure == "") {
				t.Fatal("package install must follow successful key and repository setup")
			}
			if failure == "download" || failure == "key" {
				if _, err := os.Stat(filepath.Join(root, "source")); !os.IsNotExist(err) {
					t.Fatal("repository published despite signing key failure")
				}
			}
			if failure == "" {
				source, err := os.ReadFile(filepath.Join(root, "source"))
				if err != nil {
					t.Fatal(err)
				}
				expected := "deb [arch=amd64 signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main"
				if strings.TrimSpace(string(source)) != expected {
					t.Fatalf("unexpected trust or architecture configuration: %s", source)
				}
			}
			remaining, err := os.ReadDir(temporary)
			if err != nil || len(remaining) != 0 {
				t.Fatalf("installer left temporary downloads: %v, %v", remaining, err)
			}
		})
	}
}
