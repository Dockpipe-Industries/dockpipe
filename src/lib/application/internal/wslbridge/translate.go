package wslbridge

import (
	"path/filepath"
	"strings"
)

// Translator rewrites Windows host arguments for a dockpipe invocation forwarded into WSL.
type Translator struct {
	translatePath func(string) string
	getwd         func() (string, error)
}

// New returns a Translator backed by the application-owned path and working-directory hooks.
func New(translatePath func(string) string, getwd func() (string, error)) Translator {
	return Translator{translatePath: translatePath, getwd: getwd}
}

// ForwardScript translates argv and builds the bash command executed inside WSL.
func (t Translator) ForwardScript(wslWd string, argv []string) string {
	return buildBashForwardScript(wslWd, translateBridgeArgv(t, argv))
}

// translateBridgeArgv rewrites path-like flag values and subcommand args from Windows
// to WSL paths before exec'ing dockpipe inside Linux.
func translateBridgeArgv(t Translator, argv []string) []string {
	before, after := splitArgvAtDoubleDash(argv)
	before = translateDockpipeArgs(t, before)
	if after == nil {
		return before
	}
	return append(before, after...)
}

func splitArgvAtDoubleDash(argv []string) (before []string, after []string) {
	for i, a := range argv {
		if a == "--" {
			return argv[:i], argv[i:]
		}
	}
	return argv, nil
}

func translateDockpipeArgs(t Translator, argv []string) []string {
	out := append([]string(nil), argv...)
	// Pass 1: long flags with path (or env) values
	for i := 0; i < len(out); {
		a := out[i]
		switch a {
		case "--data-dir", "--run", "--pre-script", "--act", "--action", "--workdir", "--work-path", "--bundle-out", "--env-file",
			"--isolate", "--template", "--image", "--base-url", "--url", "--repo-root", "--out", "--workflows-dir", "--to":
			if i+1 < len(out) {
				out[i+1] = maybeTranslateWinPath(t, out[i+1])
			}
			i += 2
		case "--mount":
			if i+1 < len(out) {
				out[i+1] = translateMountSpec(t, out[i+1])
			}
			i += 2
		case "--build":
			if i+1 < len(out) {
				out[i+1] = maybeTranslateWinPath(t, out[i+1])
			}
			i += 2
		case "--env", "--var":
			if i+1 < len(out) {
				out[i+1] = translateEnvOrVarLine(t, out[i+1])
			}
			i += 2
		default:
			i++
		}
	}
	// Pass 2: subcommand positionals (init / action init / pre init / template init)
	if len(out) == 0 {
		return out
	}
	switch out[0] {
	case "init":
		translateInitSubcommandArgs(t, out)
	case "release":
		if len(out) >= 2 && out[1] == "upload" {
			translateReleaseUploadLocalPath(t, out)
		}
	case "action", "pre":
		if len(out) >= 2 && (out[1] == "init" || out[1] == "create") {
			translateInitLikePositionals(t, out, 2)
		}
	case "template":
		if len(out) >= 2 && (out[1] == "init" || out[1] == "create") {
			translateInitLikePositionals(t, out, 2)
		}
	}
	return out
}

// translateReleaseUploadLocalPath maps the local file argument for release upload (first non-flag after "upload").
func translateReleaseUploadLocalPath(t Translator, out []string) {
	i := 2
	for i < len(out) {
		a := out[i]
		switch a {
		case "--bucket", "--key", "--endpoint-url", "--region", "--content-type":
			if i+1 < len(out) {
				i += 2
			} else {
				i++
			}
			continue
		case "--dry-run":
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			i++
			continue
		}
		out[i] = maybeTranslateWinPath(t, out[i])
		return
	}
}

// translateInitSubcommandArgs maps Windows paths in dockpipe init --from <path> only.
// The optional workflow name positional must not be rewritten (it is not a filesystem path).
func translateInitSubcommandArgs(t Translator, out []string) {
	for i := 1; i < len(out); i++ {
		if out[i] == "--from" && i+1 < len(out) {
			i++
			out[i] = maybeTranslateWinPath(t, out[i])
		}
	}
}

func translateInitLikePositionals(t Translator, out []string, start int) {
	for i := start; i < len(out); i++ {
		if out[i] == "--from" {
			if i+1 < len(out) {
				i++
				out[i] = maybeTranslateWinPath(t, out[i])
			}
			continue
		}
		if strings.HasPrefix(out[i], "-") {
			continue
		}
		if out[i] == "." {
			continue
		}
		out[i] = maybeTranslateWinPath(t, out[i])
	}
}

func isURL(s string) bool {
	return strings.Contains(strings.TrimSpace(s), "://")
}

func isProbablyWindowsFilesystemPath(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[1] == ':' {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	return false
}

func maybeTranslateWinPath(t Translator, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return p
	}
	if isURL(p) {
		return p
	}
	slash := filepath.ToSlash(p)
	if strings.HasPrefix(slash, "/mnt/") {
		return slash
	}
	// Linux-absolute path (already for WSL); do not rewrite
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, `\\`) {
		return p
	}
	pp := p
	if !isProbablyWindowsFilesystemPath(p) && strings.Contains(p, `\`) {
		wd, err := t.getwd()
		if err != nil {
			return p
		}
		abs, err := filepath.Abs(filepath.Join(wd, p))
		if err != nil {
			return p
		}
		pp = abs
	}
	if !isProbablyWindowsFilesystemPath(pp) && !strings.HasPrefix(pp, `\\`) {
		return p
	}
	return t.translatePath(pp)
}

func translateEnvOrVarLine(t Translator, line string) string {
	key, val, ok := strings.Cut(line, "=")
	if !ok {
		return line
	}
	val = strings.TrimSpace(val)
	unq := strings.Trim(val, `"'`)
	translated := maybeTranslateWinPath(t, unq)
	if translated == unq {
		return line
	}
	return key + "=" + translated
}

// splitDockerMountHostContainer splits "HOST:CONTAINER" where CONTAINER is a Unix-style path.
// Scans from the right so Windows hosts like C:/Users/x:/work are not split at the first ":/".
func splitDockerMountHostContainer(val string) (host, container string) {
	for i := len(val) - 1; i >= 0; i-- {
		if val[i] != ':' {
			continue
		}
		r := val[i+1:]
		if len(r) == 0 {
			continue
		}
		if r[0] == '/' || strings.HasPrefix(r, "./") || strings.HasPrefix(r, "../") {
			return strings.TrimSpace(val[:i]), r
		}
	}
	return val, ""
}

// translateMountSpec maps host paths in -v style "HOST:CONTAINER" (and optional :ro / :rw suffix).
func translateMountSpec(t Translator, val string) string {
	val = strings.TrimSpace(val)
	orig := val
	modeSuffix := ""
	lower := strings.ToLower(val)
	for _, suf := range []string{":ro", ":rw", ":z", ":Z"} {
		if strings.HasSuffix(lower, suf) {
			modeSuffix = val[len(val)-len(suf):]
			val = val[:len(val)-len(suf)]
			break
		}
	}
	host, cont := splitDockerMountHostContainer(val)
	if cont == "" {
		if shouldTranslateMountHost(val) {
			return maybeTranslateWinPath(t, val) + modeSuffix
		}
		return orig
	}
	if shouldTranslateMountHost(host) {
		host = maybeTranslateWinPath(t, host)
	}
	return host + ":" + cont + modeSuffix
}

func shouldTranslateMountHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	return isProbablyWindowsFilesystemPath(host) || strings.Contains(host, `\`)
}

// PathToWSLFallback maps C:\foo\bar to /mnt/c/foo/bar when wslpath fails.
// It uses drive-letter parsing so it behaves the same on Windows and Linux.
func PathToWSLFallback(winPath string) string {
	// Normalize Windows separators; on Unix GOOS, filepath.Clean leaves slashes alone.
	s := strings.TrimSpace(winPath)
	s = strings.ReplaceAll(s, `\`, `/`)
	// filepath.Clean turns "//server/share" into "/server/share" on Unix — breaks UNC.
	if !isUNCPathNormalized(s) {
		s = filepath.Clean(s)
		// On Windows, Clean reintroduces '\' — WSL paths must use '/'.
		s = strings.ReplaceAll(s, `\`, `/`)
	}
	if len(s) >= 2 && s[1] == ':' && s[0] != '/' {
		drive := strings.ToLower(string(s[0]))
		rest := strings.TrimPrefix(s[2:], "/")
		if rest == "" {
			return "/mnt/" + drive
		}
		return "/mnt/" + drive + "/" + rest
	}
	return s
}

func isUNCPathNormalized(s string) bool {
	// After \ -> /, UNC is //server/share (not a triple slash).
	return len(s) >= 3 && strings.HasPrefix(s, "//") && s[2] != '/'
}

func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func buildBashForwardScript(wslWd string, argv []string) string {
	var sb strings.Builder
	sb.WriteString("cd ")
	sb.WriteString(bashSingleQuote(wslWd))
	sb.WriteString(" && exec dockpipe")
	for _, a := range argv {
		sb.WriteString(" ")
		sb.WriteString(bashSingleQuote(a))
	}
	return sb.String()
}
