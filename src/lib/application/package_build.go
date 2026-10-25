package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func cmdPackageBuild(args []string) error {
	if len(args) == 0 {
		return cmdPackageBuildSource(nil)
	}
	if strings.HasPrefix(args[0], "-") {
		return cmdPackageBuildSource(args)
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageBuildUsageText)
		return nil
	}
	switch args[0] {
	case "source":
		return cmdPackageBuildSource(args[1:])
	case "store":
		return cmdPackageBuildStore(args[1:])
	default:
		return fmt.Errorf("unknown package build target %q (try: dockpipe package build --help)", args[0])
	}
}

func cmdPackageBuildStore(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageBuildStoreUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir         string
		outDir          string
		only            string
		fallbackVersion string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--out" && i+1 < len(args):
			outDir = args[i+1]
			i++
		case args[i] == "--only" && i+1 < len(args):
			only = args[i+1]
			i++
		case args[i] == "--version" && i+1 < len(args):
			fallbackVersion = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package build store --help)", args[i])
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
	return RunPackageBuildStoreFromFlags(workdir, outDir, only, fallbackVersion)
}

func cmdPackageBuildSource(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageBuildSourceUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		only    string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--only" && i+1 < len(args):
			only = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package build source --help)", args[i])
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
	_, err = RunPackageBuildSourceFromFlags(workdir, only)
	return err
}

// RunPackageBuildStoreFromEnv runs the same logic as "dockpipe package build store" using merged workflow env.
// Recognized keys: PACKAGE_STORE_OUT (--out), PACKAGE_STORE_ONLY (--only), PACKAGE_STORE_VERSION (--version).
func RunPackageBuildStoreFromEnv(projectRoot string, env map[string]string) error {
	outDir := strings.TrimSpace(env["PACKAGE_STORE_OUT"])
	only := strings.TrimSpace(env["PACKAGE_STORE_ONLY"])
	ver := strings.TrimSpace(env["PACKAGE_STORE_VERSION"])
	return RunPackageBuildStoreFromFlags(projectRoot, outDir, only, ver)
}

// RunPackageBuildStoreFromFlags implements package build store for a project root (same as CLI flags).
func RunPackageBuildStoreFromFlags(workdir, outDir, only, fallbackVersion string) error {
	repoRoot, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return err
	}
	if err := validatePackageBuildStoreOnly(only); err != nil {
		return err
	}
	packagesRoot, err := infrastructure.PackagesRoot(repoRoot)
	if err != nil {
		return err
	}
	if outDir == "" {
		outDir = filepath.Join(repoRoot, infrastructure.DefaultRepoArtifactsDir)
	} else {
		outDir = filepath.Clean(outDir)
		if !filepath.IsAbs(outDir) {
			outDir = filepath.Join(repoRoot, outDir)
		}
	}
	if fallbackVersion == "" {
		if v, err := readRepoVersion(repoRoot); err == nil {
			fallbackVersion = v
		}
	}
	opIDs := mergeOperationResultIDs(buildOperationIDs(repoRoot, ""), map[string]string{
		"slice": strings.TrimSpace(strings.ToLower(only)),
	})
	if opIDs["slice"] == "" {
		opIDs["slice"] = "all"
	}
	if strings.TrimSpace(fallbackVersion) != "" {
		opIDs["version_fallback"] = strings.TrimSpace(fallbackVersion)
	}
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.build.store", "Building compiled package store artifacts…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		if err := validateCompileOutputsForMode(repoRoot, true); err != nil {
			return fmt.Errorf("package build store: %w", err)
		}
		m, err := packagebuild.BuildCompiledStore(packagesRoot, outDir, fallbackVersion, only)
		if err != nil {
			return err
		}
		n := 0
		n += len(m.Packages.Workflows) + len(m.Packages.Resolvers)
		opIDs["count"] = strconv.Itoa(n)
		opIDs["result"] = "built"
		opIDs["output_dir"] = filepath.ToSlash(outDir)
		opIDs["manifest"] = filepath.ToSlash(filepath.Join(outDir, "packages-store-manifest.json"))
		return nil
	})
}

func validatePackageBuildStoreOnly(only string) error {
	switch strings.TrimSpace(strings.ToLower(only)) {
	case "", "all", "workflows", "resolvers":
		return nil
	default:
		return fmt.Errorf(`invalid --only %q (use all, workflows, resolvers)`, only)
	}
}

func readRepoVersion(repoRoot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("read VERSION: %w (use --version)", err)
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("VERSION is empty (use --version)")
	}
	return v, nil
}

const packageBuildUsageText = `dockpipe package build

Run package-owned build flows and author release artifacts for self-hosted package sources.

Usage:
  dockpipe package build [options]
  dockpipe package build source [options]
  dockpipe package build store [options]

Without a subtarget, dockpipe package build defaults to source.

  source Run package-owned authoring-tree build scripts declared in package.yml build.source.script.
         Source-checkout only; installed tarballs should already include the artifacts they ship.

  store  From the compiled package store (bin/.dockpipe/internal/packages after dockpipe build) —
         dockpipe-{workflow|resolver}-<name>-<ver>.tar.gz + packages-store-manifest.json.

`

const packageBuildSourceUsageText = `dockpipe package build source

Run package-owned source-checkout build scripts declared in package.yml:

  build:
    source:
      script: assets/scripts/build-source.sh

This is for authoring trees only. Installed tarballs should not require a local build step just to run.

Options:
  --workdir <path>   Project/worktree root (default: current directory, or nearest dockpipe.config.json root)
  --only <name>      Only run the matching package name

`

const packageBuildStoreUsageText = `dockpipe package build store

Packs each compiled package under <workdir>/bin/.dockpipe/internal/packages into a gzip tarball
with a stable in-archive prefix (workflows/<name>/, resolvers/<name>/).
Writes packages-store-manifest.json listing every artifact with sha256. Each tarball has a matching .sha256 file.

The store may be empty; optional workflow and resolver packages are exported when present.
Version in each tarball name comes from that package's package.yml, or --version, or 0.0.0.
Workflow tarballs exported here must set namespace: (workflow config.yml, package.yml, or dockpipe.config.json packages.namespace),
because store-facing package resolution uses namespace as part of the package contract.

Options:
  --workdir <path>   Project root (default: directory with dockpipe.config.json, else cwd)
  --out <dir>        Output directory (default: <workdir>/release/artifacts)
  --only <slice>     all (default) = workflows + resolvers; or workflows | resolvers
  --version <ver>    Fallback version when package.yml omits version (default: VERSION file or 0.0.0)

`
