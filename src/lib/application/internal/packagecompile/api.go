// Package packagecompile owns package-store compilation and validation.
package packagecompile

import (
	"dockpipe/src/lib/application/internal/imageartifact"
	"dockpipe/src/lib/application/internal/packageversion"
	"dockpipe/src/lib/domain"
)

// CoreSourceBuild runs an optional package-owned source build against the staged core tree.
type CoreSourceBuild func(workdir, staging string) error

// Command dispatches a package compile command.
func Command(args []string, sourceBuild CoreSourceBuild) error {
	return cmdPackageCompile(args, sourceBuild)
}

// InjectWorkdirFromProjectConfig applies compile command project-root discovery.
func InjectWorkdirFromProjectConfig(args []string) ([]string, error) {
	return injectCompileWorkdirFromProjectConfig(args)
}

// CompileAll compiles the full configured package store.
func CompileAll(args []string, sourceBuild CoreSourceBuild) error {
	return cmdPackageCompileAll(args, sourceBuild)
}

// CompileWorkflows compiles configured workflow authoring roots.
func CompileWorkflows(args []string) error {
	return cmdPackageCompileWorkflowsBatch(args)
}

// CompileResolverDir compiles one resolver directory for parent integration compatibility.
func CompileResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion string, force bool) error {
	return compileSingleResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion, force)
}

// CompileWorkflow compiles one workflow authoring directory.
func CompileWorkflow(workdir, source, name string, force bool) error {
	return compileWorkflowOne(workdir, source, name, force)
}

// CompileClosureForWorkflow compiles the transitive package closure for a workflow.
func CompileClosureForWorkflow(projectRoot, workflowName string, force bool, sourceBuild CoreSourceBuild) error {
	return compileClosureForWorkflow(projectRoot, workflowName, force, sourceBuild)
}

// ValidateOutputsForMode validates compiled outputs using the requested namespace mode.
func ValidateOutputsForMode(workdir string, requireWorkflowNamespace bool) error {
	return validateCompileOutputsForMode(workdir, requireWorkflowNamespace)
}

// WriteJSONFile writes a compiled JSON artifact with stable formatting.
func WriteJSONFile(path string, value any) error {
	return writeJSONFile(path, value)
}

// ClosureWorkflowOrderAndResolvers exposes dependency ordering for parent integration tests.
func ClosureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir string, cfg *domain.DockpipeProjectConfig) ([]string, map[string]bool, error) {
	return closureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir, cfg)
}

func authoredPackageVersion(workdir string) string {
	return packageversion.Authored(workdir)
}

func buildImageArtifactManifest(repoRoot, workflowName, packageName, imageKey, imageRef, buildDir, contextDir, policyFingerprint string, provenance domain.ImageArtifactProvenance) (*domain.ImageArtifactManifest, error) {
	return imageartifact.BuildManifest(imageartifact.ManifestInput{
		RepoRoot:          repoRoot,
		WorkflowName:      workflowName,
		PackageName:       packageName,
		ImageKey:          imageKey,
		ImageRef:          imageRef,
		BuildDir:          buildDir,
		ContextDir:        contextDir,
		PolicyFingerprint: policyFingerprint,
		Provenance:        provenance,
	})
}
