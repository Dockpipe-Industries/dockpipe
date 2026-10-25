package application

import (
	"os/exec"

	"dockpipe/src/lib/application/internal/hostdependencies"
	"dockpipe/src/lib/domain"

	"golang.org/x/term"
)

var (
	dependencyLookPathFn         = exec.LookPath
	dependencyPowerShellLookupFn = hostdependencies.LookupViaPowerShell
	dependencyRunShellFn         = hostdependencies.RunInstallShellCommand
)

func hostDependencyService() hostdependencies.Service {
	return hostdependencies.New(hostdependencies.Adapters{
		LookupCommand:      dependencyLookPathFn,
		LookupPowerShell:   dependencyPowerShellLookupFn,
		RunInstaller:       dependencyRunShellFn,
		PackageToolsDir:    packageToolsDir,
		BundledCommandPath: bundledDependencyCommandPath,
		TerminalAvailable: func() bool {
			fd, ok := stdinFDInt()
			return ok && term.IsTerminal(fd)
		},
	})
}

func checkWorkflowHostDependencies(wf *domain.Workflow, wfRoot, wfConfig string, opts *CliOpts) error {
	request := hostdependencies.Request{
		Workflow:             wf,
		WorkflowRoot:         wfRoot,
		Config:               wfConfig,
		Workdir:              effectiveWorkdirForWorkflowOpts(opts),
		ApproveSystemChanges: opts != nil && opts.ApproveSystemChanges,
	}
	return hostDependencyService().Check(request)
}

func resolveDependencyCommandPath(command string) (string, error) {
	return hostDependencyService().ResolveCommandPath(command)
}

func currentDependencyPlatform() string {
	return hostdependencies.CurrentPlatform()
}

func dependencyPlatformsSupportCurrent(platforms []string) bool {
	return hostdependencies.PlatformsSupportCurrent(platforms)
}

func installCommandForCurrentPlatform(dep domain.HostDependency) string {
	return hostdependencies.InstallCommandForCurrentPlatform(dep)
}
