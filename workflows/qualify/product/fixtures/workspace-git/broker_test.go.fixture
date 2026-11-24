package workspacegit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type productGitFixtureConfig struct {
	Schema           string `json:"schema"`
	Nonce            string `json:"nonce"`
	ControlDirectory string `json:"control_directory"`
	Git              string `json:"git"`
	GitSHA256        string `json:"git_sha256"`
	LifetimeSeconds  int    `json:"lifetime_seconds"`
}

func validateProductGitFixtureConfig(config productGitFixtureConfig) error {
	if config.Schema != "dockpipe.product-git-fixture/v1" || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(config.Nonce) {
		return fmt.Errorf("invalid generated fixture identity")
	}
	if config.LifetimeSeconds < 10 || config.LifetimeSeconds > 120 {
		return fmt.Errorf("fixture lifetime must be between 10 and 120 seconds")
	}
	if !filepath.IsAbs(config.ControlDirectory) || !filepath.IsAbs(config.Git) {
		return fmt.Errorf("fixture paths must be absolute")
	}
	info, err := os.Lstat(config.ControlDirectory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("fixture control directory must be a private regular directory")
	}
	return nil
}

func productGitFixtureJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	temporary := path + ".pending"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("fixture receipt already exists")
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
}

func TestProductWorkspaceGitFixtureConfig(t *testing.T) {
	control := t.TempDir()
	if err := os.Chmod(control, 0o700); err != nil {
		t.Fatal(err)
	}
	valid := productGitFixtureConfig{Schema: "dockpipe.product-git-fixture/v1", Nonce: strings.Repeat("a", 32),
		ControlDirectory: control, Git: "/generated/git", LifetimeSeconds: 60}
	if err := validateProductGitFixtureConfig(valid); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*productGitFixtureConfig){
		func(config *productGitFixtureConfig) { config.Nonce = "foreign" },
		func(config *productGitFixtureConfig) { config.LifetimeSeconds = 121 },
		func(config *productGitFixtureConfig) { config.ControlDirectory = "relative" },
	} {
		config := valid
		mutate(&config)
		if validateProductGitFixtureConfig(config) == nil {
			t.Fatal("invalid fixture configuration admitted")
		}
	}
}

func TestProductWorkspaceGitFixtureBroker(t *testing.T) {
	path := os.Getenv("DOCKPIPE_PRODUCT_GIT_FIXTURE")
	if path == "" {
		t.Skip("manual generated fixture broker requires an explicit configuration")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 16384 {
		t.Fatal("invalid bounded fixture configuration")
	}
	var config productGitFixtureConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if err := validateProductGitFixtureConfig(config); err != nil {
		t.Fatal(err)
	}
	selectedGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	selectedGit, err = filepath.EvalSymlinks(selectedGit)
	if err != nil || selectedGit != config.Git {
		t.Fatal("fixture Git executable differs from its selection")
	}
	gitBytes, err := os.ReadFile(selectedGit)
	if err != nil {
		t.Fatal(err)
	}
	gitDigest := sha256.Sum256(gitBytes)
	if hex.EncodeToString(gitDigest[:]) != config.GitSHA256 {
		t.Fatal("fixture Git executable differs from its pin")
	}
	var root string
	t.Cleanup(func() {
		_, err := os.Lstat(root)
		productGitFixtureJSON(t, filepath.Join(config.ControlDirectory, "closed.json"), map[string]any{
			"schema": config.Schema, "nonce": config.Nonce, "root": root, "repo_absent": os.IsNotExist(err),
		})
	})
	root = initGitSessionTestRepo(t)
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"selected.txt", "unrelated.txt", "draft.txt"} {
		write(name, name+" base\n")
	}
	mustGitOutput(t, root, "add", "--", "selected.txt", "unrelated.txt", "draft.txt")
	mustGitOutput(t, root, "commit", "-m", "Seed generated product fixture")
	head := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD"))
	write("selected.txt", "selected generated change\n")
	write("unrelated.txt", "unrelated staged fixture\n")
	mustGitOutput(t, root, "add", "--", "unrelated.txt")
	write("unrelated.txt", "unrelated working draft\n")
	write("draft.txt", "tracked generated draft\n")
	write("untracked-draft.txt", "untracked generated draft\n")
	productGitFixtureJSON(t, filepath.Join(config.ControlDirectory, "ready.json"), map[string]any{
		"schema": config.Schema, "nonce": config.Nonce, "root": root, "initial_head": head,
		"git_sha256": config.GitSHA256, "pid": os.Getpid(),
	})
	deadline := time.NewTimer(time.Duration(config.LifetimeSeconds) * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("generated fixture lifetime expired")
		case <-tick.C:
			stop, err := os.ReadFile(filepath.Join(config.ControlDirectory, "stop.json"))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || len(stop) > 4096 {
				t.Fatal("invalid bounded fixture stop request")
			}
			var request struct {
				Nonce   string `json:"nonce"`
				Command string `json:"command"`
			}
			if json.Unmarshal(stop, &request) != nil || request.Nonce != config.Nonce || request.Command != "stop" {
				t.Fatal("fixture stop identity differs")
			}
			productGitFixtureJSON(t, filepath.Join(config.ControlDirectory, "oracle.json"), map[string]any{
				"schema": config.Schema, "nonce": config.Nonce,
				"head":            strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD")),
				"committed_paths": strings.Fields(mustGitOutput(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")),
				"selected_head":   mustGitOutput(t, root, "show", "HEAD:selected.txt"),
				"unrelated_head":  mustGitOutput(t, root, "show", "HEAD:unrelated.txt"),
				"unrelated_index": mustGitOutput(t, root, "show", ":unrelated.txt"),
				"staged_paths":    strings.Fields(mustGitOutput(t, root, "diff", "--cached", "--name-only")),
			})
			return
		}
	}
}
