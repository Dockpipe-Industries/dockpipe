package packagecompile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

var aptPackageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+._:-]*$`)

func normalizeAptPackages(pkgs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

func validateAptPackages(pkgs []string) error {
	for _, pkg := range pkgs {
		if !aptPackageNamePattern.MatchString(pkg) {
			return fmt.Errorf("image.packages.apt contains invalid package name %q", pkg)
		}
	}
	return nil
}

func writeDerivedAptImageBuild(sourceRoot, key, baseRef, baseDockerfileDir string, pkgs []string) (string, error) {
	if err := validateAptPackages(pkgs); err != nil {
		return "", err
	}
	baseDockerfile := filepath.Join(baseDockerfileDir, "Dockerfile")
	b, err := os.ReadFile(baseDockerfile)
	if err != nil {
		return "", fmt.Errorf("read base Dockerfile for image.packages: %w", err)
	}
	body := insertAptInstallAfterBaseImage(string(b), pkgs)
	return writeDerivedImageDockerfile(sourceRoot, key, body)
}

func writeDerivedRegistryAptImageBuild(sourceRoot, key, baseRef string, pkgs []string) (string, error) {
	if err := validateAptPackages(pkgs); err != nil {
		return "", err
	}
	body := strings.Join([]string{
		"# syntax=docker/dockerfile:1.7",
		"FROM " + strings.TrimSpace(baseRef),
		"",
		aptInstallDockerfileRun(pkgs),
		"",
	}, "\n")
	return writeDerivedImageDockerfile(sourceRoot, key, body)
}

func writeDerivedImageDockerfile(sourceRoot, key, body string) (string, error) {
	key = packagebuild.SafeTarballToken(firstNonEmptyString(strings.TrimSpace(key), "workflow"))
	dir := filepath.Join(sourceRoot, domain.RuntimeManifestDirName, "images", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(body), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

func insertAptInstallAfterBaseImage(dockerfile string, pkgs []string) string {
	lines := strings.Split(strings.ReplaceAll(dockerfile, "\r\n", "\n"), "\n")
	lines = ensureDockerfileSyntax(lines)
	insertAt := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			insertAt = i + 1
			break
		}
	}
	block := []string{
		"",
		"# DockPipe workflow-authored image packages.",
		"USER root",
		aptInstallDockerfileRun(pkgs),
		"",
	}
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:insertAt]...)
	out = append(out, block...)
	out = append(out, lines[insertAt:]...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func ensureDockerfileSyntax(lines []string) []string {
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# syntax=") {
		return lines
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, "# syntax=docker/dockerfile:1.7")
	out = append(out, lines...)
	return out
}

func aptInstallDockerfileRun(pkgs []string) string {
	return "RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked apt-get update && apt-get install -y --no-install-recommends " + strings.Join(pkgs, " ") + " && rm -rf /var/lib/apt/lists/*"
}

func derivedImageRef(baseRef string, pkgs []string) string {
	fp, err := domain.FingerprintJSON(struct {
		Base string   `json:"base"`
		Apt  []string `json:"apt"`
	}{Base: strings.TrimSpace(baseRef), Apt: pkgs})
	token := "tools"
	if err == nil && strings.HasPrefix(fp, "sha256:") && len(fp) >= len("sha256:")+12 {
		token = fp[len("sha256:") : len("sha256:")+12]
	}
	return fmt.Sprintf("dockpipe-%s-tools:%s", imageRefSlug(baseRef), token)
}

func imageRefSlug(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	var b strings.Builder
	lastDash := false
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "image"
	}
	if len(out) > 64 {
		return out[:64]
	}
	return out
}
