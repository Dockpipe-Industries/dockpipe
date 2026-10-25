package application

import (
	"dockpipe/src/lib/application/internal/packagecompile"
	"dockpipe/src/lib/domain"
)

func cmdPackageCompile(args []string) error {
	return packagecompile.Command(args)
}

func injectCompileWorkdirFromProjectConfig(args []string) ([]string, error) {
	return packagecompile.InjectWorkdirFromProjectConfig(args)
}

func cmdPackageCompileAll(args []string) error {
	return packagecompile.CompileAll(args)
}

func cmdPackageCompileWorkflowsBatch(args []string) error {
	return packagecompile.CompileWorkflows(args)
}

func compileSingleResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion string, force bool) error {
	return packagecompile.CompileResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion, force)
}

func compileClosureForWorkflow(projectRoot, workflowName string, force bool) error {
	return packagecompile.CompileClosureForWorkflow(projectRoot, workflowName, force)
}

func validateCompileOutputsForMode(workdir string, requireWorkflowNamespace bool) error {
	return packagecompile.ValidateOutputsForMode(workdir, requireWorkflowNamespace)
}

func writeJSONFile(path string, value any) error {
	return packagecompile.WriteJSONFile(path, value)
}

func closureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir string, cfg *domain.DockpipeProjectConfig) ([]string, map[string]bool, error) {
	return packagecompile.ClosureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir, cfg)
}
