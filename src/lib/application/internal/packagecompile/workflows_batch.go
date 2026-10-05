package packagecompile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dockpipe/src/lib/application/internal/compileconfig"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func cmdPackageCompileWorkflowsBatch(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowsUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir    string
		from       []string
		force      bool
		pruneStale bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			from = append(from, args[i+1])
			i++
		case args[i] == "--force":
			force = true
		case args[i] == "--prune-stale":
			pruneStale = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile workflows --help)", args[i])
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
	if len(from) == 0 {
		cfg, err := compileconfig.Load(repoRoot)
		if err != nil {
			return err
		}
		from = compileconfig.WorkflowRoots(cfg, repoRoot)
	}
	opIDs := packageCompileIDs(workdir, map[string]string{
		"root_count": strconv.Itoa(len(from)),
	})
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.workflows", "Compiling workflow packages…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		seen := make(map[string]struct{})
		total := 0
		skippedRoots := 0
		skippedDuplicates := 0
		for _, root := range from {
			rootAbs, err := filepath.Abs(filepath.Clean(root))
			if err != nil {
				return err
			}
			if _, err := os.Stat(rootAbs); err != nil {
				skippedRoots++
				continue
			}
			if err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || (d.Name() != "config.yml" && d.Name() != "config.pipe") {
					return nil
				}
				wfDir := filepath.Dir(path)
				wfName := filepath.Base(wfDir)
				if strings.HasPrefix(wfName, ".") {
					return nil
				}
				if d.Name() == "config.pipe" {
					if _, err := os.Stat(filepath.Join(wfDir, "config.yml")); err == nil {
						return nil
					}
				}
				if _, ok := seen[wfName]; ok {
					skippedDuplicates++
					return nil
				}
				if d.Name() == "config.pipe" {
					if err := compileWorkflowOneFromPipe(workdir, wfDir, force); err != nil {
						return fmt.Errorf("workflow %q (config.pipe): %w", wfName, err)
					}
				} else if err := compileWorkflowOne(workdir, wfDir, "", force); err != nil {
					return fmt.Errorf("workflow %q: %w", wfName, err)
				}
				seen[wfName] = struct{}{}
				total++
				return nil
			}); err != nil {
				return err
			}
		}
		pruned := 0
		if pruneStale {
			n, err := pruneStaleWorkflowTarballs(workdir, seen)
			if err != nil {
				return err
			}
			pruned = n
		}
		opIDs["count"] = strconv.Itoa(total)
		if skippedRoots > 0 {
			opIDs["skipped_roots"] = strconv.Itoa(skippedRoots)
		}
		if skippedDuplicates > 0 {
			opIDs["skipped_duplicates"] = strconv.Itoa(skippedDuplicates)
		}
		if pruned > 0 {
			opIDs["pruned"] = strconv.Itoa(pruned)
		}
		if total == 0 {
			opIDs["result"] = "noop"
		} else {
			opIDs["result"] = "compiled"
		}
		return nil
	})
}

func pruneStaleWorkflowTarballs(workdir string, seen map[string]struct{}) (int, error) {
	pw, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return 0, err
	}
	all, err := filepath.Glob(filepath.Join(pw, "dockpipe-workflow-*.tar.gz"))
	if err != nil {
		return 0, err
	}
	keep := map[string]struct{}{}
	for name := range seen {
		pattern := filepath.Join(pw, fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", packagebuild.SafeTarballToken(name)))
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return 0, err
		}
		for _, path := range matches {
			keep[filepath.Clean(path)] = struct{}{}
		}
	}
	removed := 0
	for _, path := range all {
		clean := filepath.Clean(path)
		if _, ok := keep[clean]; ok {
			continue
		}
		if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		_ = os.Remove(clean + ".sha256")
		removed++
	}
	return removed, nil
}
