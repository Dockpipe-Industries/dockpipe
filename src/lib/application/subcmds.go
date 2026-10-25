package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/treecopy"
	"dockpipe/src/lib/infrastructure"
)

const actionBoilerplate = `#!/usr/bin/env bash
# dockpipe action — runs inside the container after your command.
set -euo pipefail
cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
if [[ "${DOCKPIPE_EXIT_CODE:-1}" -eq 0 ]]; then
  echo "Command succeeded, acting on results..."
else
  echo "Command failed with code ${DOCKPIPE_EXIT_CODE}" >&2
fi
exit "${DOCKPIPE_EXIT_CODE:-1}"
`

const preBoilerplate = `#!/usr/bin/env bash
# dockpipe pre-script — runs on the host before the container.
set -euo pipefail
`

func cmdInit(args []string) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printInitUsage()
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	var name, from string
	var resolver, runtime, strategy string
	var gitignore bool
	var workflowsDir string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--gitignore":
			gitignore = true
		case args[i] == "--workflows-dir" && i+1 < len(args):
			workflowsDir = args[i+1]
			i++
		case args[i] == "--from" && i+1 < len(args):
			from = args[i+1]
			i++
		case args[i] == "--resolver" && i+1 < len(args):
			resolver = args[i+1]
			i++
		case args[i] == "--runtime" && i+1 < len(args):
			runtime = args[i+1]
			i++
		case args[i] == "--strategy" && i+1 < len(args):
			strategy = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s", args[i])
		default:
			if name != "" {
				return fmt.Errorf("unexpected argument %q", args[i])
			}
			name = args[i]
		}
	}

	if (resolver != "" || runtime != "" || strategy != "") && name == "" {
		return fmt.Errorf("--resolver, --runtime, and --strategy require a workflow name: dockpipe init <name>")
	}
	if strings.TrimSpace(from) != "" && name == "" {
		return fmt.Errorf("--from requires a workflow name: dockpipe init <name> --from <source>")
	}
	if strings.TrimSpace(workflowsDir) != "" && name == "" {
		return fmt.Errorf("--workflows-dir requires a workflow name: dockpipe init <name> --workflows-dir <path>")
	}
	if workflowsDir != "" {
		infrastructure.SetWorkflowsDirForProcess(workflowsDir)
		defer infrastructure.SetWorkflowsDirForProcess("")
	}

	if err := infrastructure.RunOperationWithOptions(
		os.Stderr,
		"init.project",
		"Scaffolding project files…",
		map[string]string{"project": projectDir},
		infrastructure.OperationOptions{Spinner: false},
		func() error {
			return ensureProjectScaffold(projectDir)
		},
	); err != nil {
		return err
	}
	if gitignore {
		if err := appendDockpipeGitignore(projectDir); err != nil {
			return err
		}
	}
	if name == "" {
		if err := ensureDefaultStarterWorkflow(projectDir); err != nil {
			return err
		}
		return nil
	}

	fromSource := strings.TrimSpace(from)
	if fromSource == "" {
		fromSource = "blank"
	}
	return createNamedWorkflow(projectDir, name, fromSource, resolver, runtime, strategy)
}

func cmdAction(args []string) error {
	return cmdInitLikeScript(args, "my-action.sh", actionBoilerplate)
}

func cmdPre(args []string) error {
	return cmdInitLikeScript(args, "my-pre.sh", preBoilerplate)
}

func cmdTemplate(args []string) error {
	if len(args) == 0 || (args[0] != "init" && args[0] != "create") {
		return fmt.Errorf("usage: dockpipe template init [--from <local-template>] <dirname>")
	}
	args = args[1:]
	var name, from string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires argument")
			}
			from = args[i+1]
			i++
			continue
		}
		if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		name = "my-workflow"
	}
	if from == "" {
		from = "init"
	}
	dest := name
	if !filepath.IsAbs(dest) {
		wd, _ := os.Getwd()
		dest = filepath.Join(wd, dest)
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	workflow := initScaffoldWorkflow(filepath.Base(dest), from)
	var sourceDirectory string
	if workflow == nil {
		var err error
		sourceDirectory, err = resolveInitFromSource(from)
		if err != nil {
			return fmt.Errorf("unknown template source %q: %w", from, err)
		}
	}
	return infrastructure.RunOperationWithOptions(
		os.Stderr,
		"init.template",
		"Creating template workspace…",
		map[string]string{
			"template": from,
			"path":     dest,
		},
		infrastructure.OperationOptions{Spinner: false},
		func() error {
			if workflow != nil {
				if err := os.MkdirAll(dest, 0o755); err != nil {
					return err
				}
				return writeWorkflowYAML(filepath.Join(dest, "config.yml"), workflow)
			}
			if err := copyDir(sourceDirectory, dest); err != nil {
				return err
			}
			_ = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
				if err == nil && strings.HasSuffix(p, ".sh") {
					_ = os.Chmod(p, 0o755)
				}
				return nil
			})
			return nil
		},
	)
}

func cmdInitLikeScript(args []string, defaultName, boiler string) error {
	if len(args) == 0 || (args[0] != "init" && args[0] != "create") {
		return fmt.Errorf("usage: dockpipe <cmd> init [--from <local-script>] <filename>")
	}
	args = args[1:]
	var name, from string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires argument")
			}
			from = args[i+1]
			i++
			continue
		}
		if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		name = defaultName
	}
	if !strings.HasSuffix(name, ".sh") {
		name += ".sh"
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	dest := name
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(wd, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	source := "boilerplate"
	if from != "" {
		source = strings.TrimSuffix(from, ".sh")
	}
	return infrastructure.RunOperationWithOptions(
		os.Stderr,
		"init.script",
		"Creating helper script…",
		map[string]string{
			"path":   dest,
			"source": source,
		},
		infrastructure.OperationOptions{Spinner: false},
		func() error {
			if from != "" {
				src, err := filepath.Abs(from)
				if err != nil {
					return err
				}
				switch template := strings.TrimSuffix(from, ".sh"); template {
				case "clone-worktree", "commit-worktree", "export-patch", "print-summary":
					src, err = infrastructure.LegacyGitHookPath(wd, template)
					if err != nil {
						return err
					}
				}

				info, err := os.Stat(src)
				if err != nil {
					return fmt.Errorf("read script source %q: %w", from, err)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("script source %q must be a regular file", from)
				}

				if err := copyFile(src, dest); err != nil {
					return err
				}
				return os.Chmod(dest, 0o755)
			}
			return os.WriteFile(dest, []byte(boiler), 0o755)
		},
	)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	return os.WriteFile(dst, b, 0o644)
}

func copyDir(src, dst string) error {
	return treecopy.Copy(src, dst)
}
