package packagecompile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/compileconfig"
	"dockpipe/src/lib/infrastructure"
)

func cmdPackageCompileAll(args []string, sourceBuild CoreSourceBuild) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileAllUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case args[i] == "--with-bundles", args[i] == "--skip-bundles":
			// Ignored compatibility no-ops.
		case args[i] == "--help" || args[i] == "-h":
			fmt.Print(packageCompileAllUsageText)
			return nil
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile all --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	cfg, err := compileconfig.Load(repoRoot)
	if err != nil {
		return err
	}
	opIDs := packageCompileIDs(workdir, nil)
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.all", "Compiling package store…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		if err := cmdPackageCompileCore(workdirAndForceArgs(workdir, force), sourceBuild); err != nil {
			return err
		}
		resRoots := compileconfig.ResolverRoots(cfg, repoRoot)
		if len(resRoots) == 0 {
			opIDs["resolver_result"] = "noop"
		} else {
			resArgs := []string{"--workdir", workdir}
			if force {
				resArgs = append(resArgs, "--force")
			}
			for _, p := range resRoots {
				resArgs = append(resArgs, "--from", p)
			}
			if err := cmdPackageCompileResolvers(resArgs); err != nil {
				return err
			}
		}
		wfArgs := workdirAndForceArgs(workdir, force)
		if force {
			wfArgs = append(wfArgs, "--prune-stale")
		}
		for _, p := range compileconfig.WorkflowRoots(cfg, repoRoot) {
			wfArgs = append(wfArgs, "--from", p)
		}
		if err := cmdPackageCompileWorkflowsBatch(wfArgs); err != nil {
			return err
		}
		if err := validateCompileOutputs(workdir); err != nil {
			return err
		}
		opIDs["result"] = "compiled"
		return nil
	})
}

func workdirAndForceArgs(workdir string, force bool) []string {
	out := []string{"--workdir", workdir}
	if force {
		out = append(out, "--force")
	}
	return out
}
