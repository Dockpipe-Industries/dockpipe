package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDockerGitSnapshotNativeAttachedAndDetached(t *testing.T) {
	image := os.Getenv("DOCKPIPE_TEST_DOCKER_GIT_IMAGE")
	if image == "" {
		t.Skip("set DOCKPIPE_TEST_DOCKER_GIT_IMAGE to an existing Linux image with Git for native Docker qualification")
	}
	if runtime.GOOS != "linux" {
		t.Fatal("detached file-bind lifetime is currently qualified only on Linux")
	}
	_, linked := dockerLinkedFixture(t)
	if err := os.WriteFile(filepath.Join(linked, "staged.txt"), []byte("staged-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dockerGitFixture(t, linked, "", "add", "staged.txt")
	dockerGitFixture(t, linked, "", "update-index", "--split-index")
	gitfile, _ := os.ReadFile(filepath.Join(linked, ".git"))
	indexPath := dockerGitFixture(t, linked, "", "rev-parse", "--git-path", "index")
	index, _ := os.ReadFile(indexPath)
	for _, fixture := range []struct {
		name     string
		detached bool
		volume   bool
	}{
		{name: "attached-bind"}, {name: "attached-volume", volume: true}, {name: "detached-bind", detached: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			detached := fixture.detached
			volume := ""
			if fixture.volume {
				volume = fmt.Sprintf("dockpipe-git-proof-%d-%d", os.Getpid(), time.Now().UnixNano())
				t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", volume).Run() })
			}
			input, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			output, err := os.CreateTemp(t.TempDir(), "stdout")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			errors, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			defer errors.Close()
			script := `test "$(git show :staged.txt)" = staged-only
                test "$(git branch --show-current)" = fixture-branch
                git ls-files --error-unmatch tracked.txt staged.txt >/dev/null
                git bundle create /tmp/fixture.bundle HEAD
                git bundle verify /tmp/fixture.bundle
                test "$(git config --get remote.origin.url || true)" = ""
                test "$GIT_OPTIONAL_LOCKS" = 0
                printf 'snapshot-ready\n' > /work/docker-git-proof.txt`
			if detached {
				script = "sleep 2\n" + script
			}
			code, runErr := RunContainer(RunOpts{Image: image, WorkdirHost: linked, WorkdirVolume: volume, Detach: detached, NetworkMode: "none",
				ExtraEnv: []string{"GIT_DIR=/poison", "GIT_WORK_TREE=/poison", "GIT_CONFIG_COUNT=7"},
				Stdin:    input, Stdout: output, Stderr: errors}, []string{"sh", "-ec", script})
			if runErr != nil || code != 0 {
				details, _ := os.ReadFile(errors.Name())
				t.Fatalf("native RunContainer: code=%d err=%v\n%s", code, runErr, details)
			}
			if detached {
				data, _ := os.ReadFile(output.Name())
				container := strings.TrimSpace(string(data))
				if len(container) != 64 {
					t.Fatalf("invalid detached container ID: %q", container)
				}
				t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", container).Run() })
				rawMounts, inspectErr := exec.Command("docker", "inspect", "--format", "{{json .Mounts}}", container).Output()
				if inspectErr != nil {
					t.Fatalf("cannot inspect detached fixture mounts: %v", inspectErr)
				}
				var mounts []struct {
					Source      string
					Destination string
					RW          bool
				}
				if err := json.Unmarshal(rawMounts, &mounts); err != nil {
					t.Fatal(err)
				}
				retainedFiles := 0
				for _, mount := range mounts {
					if mount.Destination != containerWorkMount+"/.git" && !strings.HasPrefix(mount.Destination, dockerGitDirectory+"/") {
						continue
					}
					if mount.RW {
						t.Fatal("Git snapshot mount is writable")
					}
					if _, err := os.Lstat(mount.Source); !os.IsNotExist(err) {
						t.Fatalf("host snapshot source was not removed before detached inspection: %v", err)
					}
					retainedFiles++
				}
				if retainedFiles < 7 {
					t.Fatalf("missing isolated snapshot mounts: %d", retainedFiles)
				}
				deadline := time.Now().Add(20 * time.Second)
				for {
					state, inspectErr := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", container).CombinedOutput()
					if inspectErr != nil || strings.TrimSpace(string(state)) == "false" {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("detached fixture exceeded its deadline")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			proof, err := os.ReadFile(filepath.Join(linked, "docker-git-proof.txt"))
			if err != nil || string(proof) != "snapshot-ready\n" {
				t.Fatalf("container Git inspection did not complete: %q %v", proof, err)
			}
			if err := os.Remove(filepath.Join(linked, "docker-git-proof.txt")); err != nil {
				t.Fatal(err)
			}
		})
	}
	afterGitfile, _ := os.ReadFile(filepath.Join(linked, ".git"))
	afterIndex, _ := os.ReadFile(indexPath)
	if string(gitfile) != string(afterGitfile) || string(index) != string(afterIndex) {
		t.Fatal("Docker execution changed original linked-worktree metadata")
	}
}
