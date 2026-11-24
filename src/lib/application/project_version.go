package application

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/engineversion"
)

// checkCLIProjectVersion runs before command dispatch can execute project code.
// Informational and installation commands remain available to repair a mismatch.
func checkCLIProjectVersion(argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	switch argv[0] {
	case "terminal":
		// A private native terminal owns an explicit launch snapshot and does
		// not load project workflows. Its stream must emit its own admission and
		// cleanup receipts even when cwd contains a mismatched project pin.
		return nil
	case "init", "install", "get", "core", "doctor", "__state", "--version", "version":
		return nil
	}
	if strings.HasPrefix(argv[0], "-") {
		_, options, err := ParseFlags("", argv)
		if err == nil && options.Help {
			return nil
		}
	}
	workdir := ""
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		if arg == "--" {
			break
		}
		if (arg == "--help" || arg == "-h") && (index == 0 || !strings.HasPrefix(argv[index-1], "-")) {
			return nil
		}
		if arg == "--workdir" && index+1 < len(argv) {
			index++
			workdir = argv[index]
		} else if strings.HasPrefix(arg, "--workdir=") {
			workdir = strings.TrimPrefix(arg, "--workdir=")
		}
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			workdir = workflowInspectionDirectory(argv)
			if workdir == "" {
				return err
			}
		}
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return err
	}
	config, err := domain.LoadDockpipeProjectConfig(root)
	if err != nil {
		return err
	}
	return domain.CheckProjectDockpipeVersion(config, engineversion.Current())
}

// A document portal can invalidate the cwd's directory entry while the
// caller's workspace path still identifies the same open directory. Use that
// spelling for version admission without letting it select a different project.
func workflowInspectionDirectory(argv []string) string {
	if len(argv) < 2 || argv[0] != "workflow" || (argv[1] != "document" && argv[1] != "validate") {
		return ""
	}

	workdir := os.Getenv("DOCKPIPE_WORKDIR")
	if !filepath.IsAbs(workdir) {
		return ""
	}
	workdir, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		return ""
	}

	current, err := os.Stat(".")
	if err != nil {
		return ""
	}
	workspace, err := os.Stat(workdir)
	if err != nil || !os.SameFile(current, workspace) {
		return ""
	}

	return workdir
}
